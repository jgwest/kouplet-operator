package koupletbuild

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/jgwest/kouplet/pkg/controller/shared"
	kubeErrors "k8s.io/apimachinery/pkg/api/errors"
)

// PersistenceContext ...
type PersistenceContext struct {
	persistenceStore *shared.PersistenceStore
}

func newPersistenceContext(path string) *PersistenceContext {
	perStore, err := shared.NewFilePersistence(path)

	if err != nil {
		shared.LogErrorMsg(err, "Unable to create file persistence context")
		return nil
	}

	var persistenceStore shared.PersistenceStore = perStore

	return &PersistenceContext{
		persistenceStore: &persistenceStore,
	}
}

func (pc *PersistenceContext) load(ctx KBContext) (*koupletBuildDB, error) {

	// Load all values from the database
	keys, err := (*pc.persistenceStore).ListKeysByPrefix("kouplet-build-")
	if err != nil {
		return nil, err
	}

	values, err := (*pc.persistenceStore).ReadValues(keys)
	if err != nil {
		return nil, err
	}

	result := &koupletBuildDB{
		builds:             map[string]*buildStatus{},
		persistenceContext: pc,
	}

	databaseKeysToDelete := []string{}

	for key, buildStatusJSONStr := range values {

		if buildStatusJSONStr == nil {
			shared.LogError(fmt.Errorf("Database did not contain a value for key '%s'", key))
			continue
		}

		buildStatus := buildStatusJSON{}

		err = unmarshalFromJSON(*buildStatusJSONStr, &buildStatus)
		if err != nil {
			shared.LogSevereMsg(err, "Unable to unmarshal JSON from DB '"+key+"'")
			continue
		}

		buildStatusFromJSON, deleteFromDB := convertJSONToBuildStatus(&buildStatus, ctx)
		if !deleteFromDB {
			result.builds[buildStatus.UID] = buildStatusFromJSON
		} else {
			// The database contains a reference to a Kubernetes resource that no longer exists, so skip it and remove the database entry
			databaseKeysToDelete = append(databaseKeysToDelete, getKeyFromJSON(&buildStatus))
		}

	}

	if len(databaseKeysToDelete) > 0 {
		err = (*pc.persistenceStore).DeleteByKey(databaseKeysToDelete)
		if err != nil {
			shared.LogErrorMsg(err, "Unable to delete old database keys (those w/o a corresponding k8s reference)")
		}
	}

	return result, nil
}

func (pc *PersistenceContext) updateBuildStatus(status *buildStatus, ctx KBContext) {

	json := convertBuildStatusToJSON(status)
	if json == nil {
		shared.LogSevere(fmt.Errorf("Unable convert buildStatus into JSON struct, for status '%s'", status.shortString()))
		return
	}

	jsonString, err := marshalToJSON(json)
	if err != nil {
		shared.LogSevere(fmt.Errorf("Unable convert buildStatus JSON struct to JSON string, for status '%s'", status.shortString()))
		return
	}

	key := getKeyFromJSON(json)

	valuesToWrite := map[string]string{
		key: jsonString,
	}

	err = (*pc.persistenceStore).WriteValues(valuesToWrite)
	if err != nil {
		shared.LogErrorMsg(err, "Unable to write build status for '"+status.shortString()+"'")
	}

}

func (pc *PersistenceContext) deleteBuildStatus(status *buildStatus, ctx KBContext) error {
	key := getKeyFromBuildStatus(status)

	err := (*pc.persistenceStore).DeleteByKey([]string{key})

	return err

}

func convertJSONToBuildStatus(json *buildStatusJSON, ctx KBContext) (*buildStatus, bool) {

	activeAttempts := []*buildAttempt{}
	completedAttempts := []*buildAttempt{}

	for _, activeAttempt := range json.ActiveAttempts {
		activeAttempts = append(activeAttempts, convertJSONToBuildAttempt(activeAttempt))
	}

	for _, completedAttempt := range json.CompletedAttempts {
		completedAttempts = append(completedAttempts, convertJSONToBuildAttempt(completedAttempt))
	}

	buildStatusStateVal, err := convertStringToBuildStatusState(json.State)
	if err != nil {
		shared.LogErrorMsg(err, "Couldn't get build status state for "+getKeyFromJSON(json))
		return nil, false
	}

	instance, err := getInstance(json.Name, ctx.namespace, ctx)
	if err != nil || instance == nil {
		if kubeErrors.IsNotFound(err) {
			shared.LogInfo("Could not find kubernetes resource for '" + getKeyFromJSON(json) + "', so will delete")
			return nil, true
		}

		shared.LogSevereMsg(err, "Could not retrieve kubernetes resource for '"+getKeyFromJSON(json)+"'")
		return nil, false
	}

	result := &buildStatus{
		name:              json.Name,
		uid:               json.UID,
		state:             buildStatusStateVal,
		kubeObj:           *instance,
		activeAttempts:    activeAttempts,
		completedAttempts: completedAttempts,
		kubeCreationTime:  *convertInt64ToTime(&json.KubeCreationTime),
		startTime:         convertInt64ToTime(json.StartTime),
		completionTime:    convertInt64ToTime(json.CompletionTime),
		successResult:     json.SuccessResult,
	}

	return result, false
}

func convertBuildStatusToJSON(buildStatus *buildStatus) *buildStatusJSON {
	// buildStatusJSON

	activeAttempts := []*buildAttemptJSON{}
	completedAttempts := []*buildAttemptJSON{}

	for _, activeAttempt := range buildStatus.activeAttempts {
		activeAttempts = append(activeAttempts, convertBuildAttemptToJSON(activeAttempt))
	}

	for _, completedAttempt := range buildStatus.completedAttempts {
		completedAttempts = append(completedAttempts, convertBuildAttemptToJSON(completedAttempt))
	}

	return &buildStatusJSON{
		Name:              buildStatus.name,
		UID:               buildStatus.uid,
		State:             convertBuildStatusStateToString(buildStatus.state),
		ActiveAttempts:    activeAttempts,
		CompletedAttempts: completedAttempts,
		KubeCreationTime:  *convertTime(&buildStatus.kubeCreationTime),
		StartTime:         convertTime(buildStatus.startTime),
		CompletionTime:    convertTime(buildStatus.completionTime),
		SuccessResult:     buildStatus.successResult,
	}
}

func convertJSONToBuildAttempt(json *buildAttemptJSON) *buildAttempt {

	buildAttempt := &buildAttempt{
		jobName:           json.JobName,
		jobUID:            json.JobUID,
		jobQueueTime:      *convertInt64ToTime(&json.JobQueueTime),
		jobStartTime:      convertInt64ToTime(json.JobStartTime),
		jobCompletionTime: convertInt64ToTime(json.JobCompletionTime),
		jobSucceeded:      json.JobSucceeded,
	}

	return buildAttempt
}

func convertBuildAttemptToJSON(attempt *buildAttempt) *buildAttemptJSON {

	buildAttemptJSON := &buildAttemptJSON{
		JobName:           attempt.jobName,
		JobUID:            attempt.jobUID,
		JobCompletionTime: convertTime(attempt.jobCompletionTime),
		JobStartTime:      convertTime(attempt.jobStartTime),
		JobQueueTime:      *convertTime(&attempt.jobQueueTime),
		JobSucceeded:      attempt.jobSucceeded,
	}

	return buildAttemptJSON

}

type buildStatusJSON struct {
	Name              string              `json:"name"`
	UID               string              `json:"uid"`
	State             string              `json:"state"`
	SuccessResult     *bool               `json:"successResult,omitempty"`
	ActiveAttempts    []*buildAttemptJSON `json:"activeAttempts"`
	CompletedAttempts []*buildAttemptJSON `json:"completedAttempts"`

	// kubeCreationTime is time at which the kube object was first seen
	KubeCreationTime int64 `json:"kubeCreationTime"`

	// startTime is the time at which the first buildAttempt job started
	StartTime *int64 `json:"startTime,omitempty"`

	// The time at which the buildStatus completed (either successfully, or it expired, or failed too many times)
	CompletionTime *int64 `json:"completionTime,omitempty"`
}

type buildAttemptJSON struct {
	JobName string `json:"jobName"`
	JobUID  string `json:"jobUID"`

	// jobQueueTime is the time at which the Job object was created on the cluster
	JobQueueTime      int64  `json:"jobQueueTime"`
	JobStartTime      *int64 `json:"jobStartTime,omitempty"`
	JobCompletionTime *int64 `json:"jobCompletionTime,omitempty"`

	JobSucceeded *bool `json:"jobSucceeded,omitempty"`
}

func getKeyFromBuildStatus(status *buildStatus) string {
	return "kouplet-build-" + status.name + "-" + status.uid

}

func getKeyFromJSON(json *buildStatusJSON) string {
	return "kouplet-build-" + json.Name + "-" + json.UID
}

func unmarshalFromJSON(str string, obj interface{}) error {
	return json.Unmarshal([]byte(str), &obj)
}

func marshalToJSON(obj interface{}) (string, error) {
	result, err := json.Marshal(obj)

	if err != nil {
		return "", nil
	}

	return string(result), nil

}

func convertInt64ToTime(epochTime *int64) *time.Time {
	if epochTime == nil {
		return nil
	}

	inNanos := *epochTime * int64(1000000)

	inSeconds := int64((inNanos / 1000000) / 1000)

	remainingNanos := inNanos - (inSeconds * 1000000 * 1000)

	result := time.Unix(inSeconds, remainingNanos)

	return &result
}

func convertTime(time *time.Time) *int64 {

	if time == nil {
		return nil
	}

	result := time.UnixNano() / 1000000

	return &result

}
