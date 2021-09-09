package koupletbuild

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	apiv1alpha1 "github.com/jgwest/kouplet-operator/api/v1alpha1"
	"github.com/jgwest/kouplet-operator/controllers/shared"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	kubeErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// DefaultMaxOverallBuildTimeInSeconds ...
	DefaultMaxOverallBuildTimeInSeconds = 7200

	// DefaultMaxNumberofBuildAttempts ...
	DefaultMaxNumberofBuildAttempts = 5

	// DefaultMaxJobBuildTimeInSeconds ...
	DefaultMaxJobBuildTimeInSeconds = 3600

	// DefaultMaxJobWaitStartTimeInSeconds ...
	DefaultMaxJobWaitStartTimeInSeconds = 3600
)

const (
	// DelayOnKubernetesResourceCreation ...
	DelayOnKubernetesResourceCreation = 10 * time.Second
)

const (
	DefaultBuilderImageName = "jgwest/k-builder-util"
	DefaultBuilderImageTag  = "latest"
)

type buildStatusState string

const (
	buildStatusStateWaiting  = buildStatusState("Waiting")
	buildStatusStateRunning  = buildStatusState("Running")
	buildStatusStateComplete = buildStatusState("Completed")
	buildStatusStateExpired  = buildStatusState("Expired")
)

// KBContext ...
type KBContext struct {
	Client    *client.Client
	Namespace string
	Scheme    *runtime.Scheme
}

func (db *KoupletBuildDB) Process(kubeBuild apiv1alpha1.KoupletBuild, ctx KBContext) {
	newBuildStatusCreated := false

	build := db.builds[string(kubeBuild.UID)]
	if build == nil {
		build = newBuildStatus(kubeBuild)
		db.builds[string(kubeBuild.UID)] = build
		newBuildStatusCreated = true
	}

	build.process(db, ctx, newBuildStatusCreated)

	db.scheduledTasks.RunTasksIfNeeded(db, ctx)

}

func cleanupOldDatabaseEntries(dbParam interface{}, ctxParam interface{}) error {

	db := dbParam.(*KoupletBuildDB)
	ctx := ctxParam.(KBContext)

	UIDsToRemove := []string{}
	for k, v := range db.builds {

		// Fetch the KoupletBuild instance
		instance, err := getInstance(v.kubeObj.Name, v.kubeObj.Namespace, ctx)

		deleteFromDb := false

		if err != nil {
			if kubeErrors.IsNotFound(err) {
				deleteFromDb = true
				shared.LogInfo("Cleaning up database as Kubernetes object was not found " + v.name + "(" + v.uid + ")")
			} else {
				shared.LogErrorMsg(err, "A generic error occured during database cleanup")
				continue
			}
		} else {

			if string(instance.UID) != string(v.kubeObj.UID) {
				deleteFromDb = true
				shared.LogInfo("Cleaning up database as Kubernetes object was not found " + v.name + "(" + v.uid + ")")
			}

		}

		if deleteFromDb {
			UIDsToRemove = append(UIDsToRemove, k)
		}
	}

	for _, uidToRemove := range UIDsToRemove {

		buildStatus := db.builds[uidToRemove]
		if buildStatus != nil {
			err := db.persistenceContext.deleteBuildStatus(buildStatus, ctx)
			if err == nil {
				delete(db.builds, uidToRemove)
			} else {
				// If we're not able to delete at this point, we will get it on the next job run
				shared.LogErrorMsg(err, "Unable to delete build status from database")
			}
		}
	}

	return nil
}

func getInstance(name string, namespace string, ctx KBContext) (*apiv1alpha1.KoupletBuild, error) {
	// Fetch the KoupletTest instance
	instance := apiv1alpha1.KoupletBuild{}
	err := (*(ctx.Client)).Get(context.TODO(), types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}, &instance)

	return &instance, err
}

// KoupletBuildDB ...
type KoupletBuildDB struct {
	builds             map[string]*buildStatus
	scheduledTasks     shared.ScheduledTasks
	persistenceContext *PersistenceContext
}

var newDB = false

func NewKoupletBuildDB(persistenceContext *PersistenceContext, ctx KBContext) *KoupletBuildDB {

	shared.LogInfo(fmt.Sprintf("Creating KoupletBuildDB %v", newDB))

	var result *KoupletBuildDB

	if newDB {
		result = &KoupletBuildDB{
			builds:             map[string]*buildStatus{},
			persistenceContext: persistenceContext,
		}

	} else {

		var err error
		result, err = persistenceContext.load(ctx)
		if err != nil {
			time.Sleep(30 * 1000 * time.Millisecond)
			shared.LogSevereMsg(err, "Error when loading database from persistence context")
			return nil
		}

	}

	timeBetweenCleanups := 5 * time.Minute
	result.scheduledTasks.AddTask("cleanupOldDatabaseEntries", time.Now().Add(timeBetweenCleanups), timeBetweenCleanups, cleanupOldDatabaseEntries)

	return result
}

func newBuildStatus(kubeBuild apiv1alpha1.KoupletBuild) *buildStatus {

	creationTime := time.Now()

	buildStatus := &buildStatus{
		name:              kubeBuild.Name,
		uid:               string(kubeBuild.UID),
		kubeObj:           kubeBuild,
		state:             buildStatusStateWaiting,
		activeAttempts:    []*buildAttempt{},
		completedAttempts: []*buildAttempt{},
		kubeCreationTime:  creationTime,
	}

	return buildStatus
}

type buildStatus struct {
	name              string
	uid               string
	kubeObj           apiv1alpha1.KoupletBuild
	state             buildStatusState
	successResult     *bool
	activeAttempts    []*buildAttempt
	completedAttempts []*buildAttempt

	// kubeCreationTime is time at which the kube object was first seen
	kubeCreationTime time.Time

	// startTime is the time at which the first buildAttempt job started
	startTime *time.Time

	// The time at which the buildStatus completed (either successfully, or it expired, or failed too many times)
	completionTime *time.Time
}

func (buildStatus *buildStatus) process(db *KoupletBuildDB, ctx KBContext, initialCreation bool) {

	stateChangedAtLeastOnce := false

	// If the buildStatus was JUST created by our parent function, then we want to persist it and update the state
	if initialCreation {
		stateChangedAtLeastOnce = true
	}

	for {
		stateChanged := buildStatus.subProcess(ctx)

		if stateChanged {
			stateChangedAtLeastOnce = true
		} else {
			break
		}
	}

	if stateChangedAtLeastOnce {
		updateKubeStatus(buildStatus, ctx)

		shared.LogInfo("State changed, updating database")
		db.persistenceContext.updateBuildStatus(buildStatus, ctx)
	}

}

func (buildStatus *buildStatus) subProcess(ctx KBContext) bool {

	stateChanged := false

	instance, err := getInstance(buildStatus.kubeObj.Name, buildStatus.kubeObj.Namespace, ctx)

	buildConstraints := instance.Spec.BuildContraints

	if err != nil {
		shared.LogErrorMsg(err, "Unable to locate kube object by name")
		return stateChanged
	}

	if buildStatus.state == buildStatusStateWaiting {
		buildStatus.updateState(buildStatusStateRunning)
	}

	if buildStatus.state == buildStatusStateComplete || buildStatus.state == buildStatusStateExpired {
		shared.LogDebug("Ignoring " + buildStatus.name + " since it is complete/expired")
		return stateChanged

	} else if buildStatus.state == buildStatusStateRunning {
		// Ignore
	} else {
		shared.LogError(errors.New("Unexpected buildStatus state:" + convertBuildStatusStateToString(buildStatus.state)))
		return stateChanged
	}

	activeAttemptsToRemove := []*buildAttempt{}

	// Scan through existing jobs
	for _, activeAttempt := range buildStatus.activeAttempts {

		var job batchv1.Job

		err := (*ctx.Client).Get(context.TODO(), types.NamespacedName{Name: activeAttempt.jobName, Namespace: ctx.Namespace}, &job)
		if err != nil {
			if kubeErrors.IsNotFound(err) {
				shared.LogErrorMsg(err, "Unable to locate attempt job: "+activeAttempt.jobName)
				activeAttemptsToRemove = append(activeAttemptsToRemove, activeAttempt)
				continue

			} else {
				// Generic kube error: log but don't remove
				shared.LogErrorMsg(err, "Generic error on GET for job")
			}
		}

		jobStatus := job.Status

		// If Job is still running...
		if jobStatus.Failed+jobStatus.Succeeded == 0 {

			// Update build start time (if not already done)
			if buildStatus.startTime == nil && jobStatus.StartTime != nil {
				startTime := jobStatus.StartTime
				buildStatus.startTime = &startTime.Time
				stateChanged = true
			}

			// Update attempt start time (if not already done)
			if activeAttempt.jobStartTime == nil && jobStatus.StartTime != nil {
				startTime := jobStatus.StartTime
				activeAttempt.jobStartTime = &startTime.Time
				stateChanged = true
			}

			// Expire old jobs
			maxJobWaitStartTimeInSeconds := DefaultMaxJobWaitStartTimeInSeconds
			maxJobBuildTimeInSeconds := DefaultMaxJobBuildTimeInSeconds

			if buildConstraints != nil {
				if buildConstraints.MaxJobWaitStartTimeInSeconds != nil {
					maxJobWaitStartTimeInSeconds = *buildConstraints.MaxJobWaitStartTimeInSeconds
				}

				if buildConstraints.MaxJobBuildTimeInSeconds != nil {
					maxJobBuildTimeInSeconds = *buildConstraints.MaxJobBuildTimeInSeconds
				}
			}

			expireTime := activeAttempt.jobQueueTime.Add(time.Duration(maxJobWaitStartTimeInSeconds) * time.Second)
			if time.Now().After(expireTime) {
				// Kubernetes Job did not start in time
				shared.LogError(fmt.Errorf("Kubernetes job did not start in time: %s (%s)", job.Name, job.UID))
				activeAttemptsToRemove = append(activeAttemptsToRemove, activeAttempt)
			}

			if jobStatus.StartTime != nil {

				expireTime := jobStatus.StartTime.Add(time.Duration(maxJobBuildTimeInSeconds) * time.Second)

				if time.Now().After(expireTime) {
					// Kubernetes job did not complete in time
					shared.LogError(fmt.Errorf("Kubernetes job did not complete in time: %s (%s)", job.Name, job.UID))
					activeAttemptsToRemove = append(activeAttemptsToRemove, activeAttempt)
				}
			}

		} else {
			// Else if job is complete

			if activeAttempt.jobCompletionTime == nil {

				// Substitute the current time if the job did not complete normally
				completionTime := time.Now()
				if jobStatus.CompletionTime != nil {
					completionTime = jobStatus.CompletionTime.Time
				}

				activeAttempt.jobCompletionTime = &completionTime
				stateChanged = true
			}

			jobSucceeded := false

			// Locate the exit code of the container in the pod in the job
			containerExitCode := getContainerExitCode(string(job.UID), ctx)

			if containerExitCode != nil {
				shared.LogInfo(fmt.Sprintf("Job completed and container exit code is %d", *containerExitCode))
				jobSucceeded = *containerExitCode == 0
			} else {
				shared.LogSevere(fmt.Errorf("Unable to locate container exit code from job %s (%s)", job.Name, job.UID))
				jobSucceeded = false
			}

			shared.LogInfo("Moving active attempt '" + activeAttempt.jobName + "' to activeAttemptsToRemove")

			// Move the active attempt to completed
			activeAttemptsToRemove = append(activeAttemptsToRemove, activeAttempt)
			activeAttempt.jobSucceeded = &jobSucceeded

			if jobSucceeded {
				boolTrue := true
				buildStatus.successResult = &boolTrue
			}

			stateChanged = true

		} // end job is complete else

	} // end scan through existing jobs

	// Move any completed attempts from 'activeAttempts' to 'completedAttempts'
	newActiveAttempts := []*buildAttempt{}
	anyMatches := false
	for _, activeAttempt := range buildStatus.activeAttempts {
		matchInRemove := false
		for _, toRemove := range activeAttemptsToRemove {
			if activeAttempt.jobUID == toRemove.jobUID {
				matchInRemove = true
				break
			}
		}

		if !matchInRemove {
			// shared.LogInfo("No match in remove '" + activeAttempt.shortString())
			newActiveAttempts = append(newActiveAttempts, activeAttempt)
		} else {
			shared.LogInfo("Moving active attempt '" + activeAttempt.shortString() + "' to completedAttempts")
			buildStatus.completedAttempts = append(buildStatus.completedAttempts, activeAttempt)
			stateChanged = true
			anyMatches = true
		}
	}
	if anyMatches {
		buildStatus.activeAttempts = newActiveAttempts
		stateChanged = true
	}

	// Check max number of build attempts
	if buildStatus.expireBuildIfExceedsMaxNumberOfBuildAttempts(instance) {
		stateChanged = true
	}

	// Check max overall build time
	if buildStatus.expireBuildIfExceedsMaxOverallBuildTime(instance) {
		stateChanged = true
	}

	if buildStatus.state == buildStatusStateRunning {

		shared.LogInfo(fmt.Sprintf("Completed attempts is currently %d", len(buildStatus.completedAttempts)))

		// Look for a successful attempt, and move the buildStatus to completed if so
		for _, completedAttempt := range buildStatus.completedAttempts {

			if completedAttempt.jobSucceeded != nil && *completedAttempt.jobSucceeded == true {
				now := time.Now()
				buildStatus.updateState(buildStatusStateComplete)
				buildStatus.completionTime = &now
				boolTrue := true
				buildStatus.successResult = &boolTrue
				stateChanged = true

				shared.LogInfo(fmt.Sprintf("buildStatus %s succeeded due to successful job %s", buildStatus.shortString(), completedAttempt.jobName))
				break
			}
		}
	}

	if buildStatus.state == buildStatusStateRunning {
		// Don't merge this w/ the above if block

		// Start a new attempt if there is not already one active
		if len(buildStatus.activeAttempts) == 0 {

			newBuildAttempt, err := buildStatus.newBuildAttempt(ctx, *instance)
			if err == nil {
				shared.LogInfo("New buildAttempt started " + newBuildAttempt.jobName + "(" + newBuildAttempt.jobUID + ")")
				buildStatus.activeAttempts = append(buildStatus.activeAttempts, newBuildAttempt)
				stateChanged = true

			} else {
				shared.LogErrorMsg(err, "Unable to create new buildAttempt for "+buildStatus.shortString())
			}
		}

	}

	return stateChanged
}

// Check max number of build attempts
func (buildStatus *buildStatus) expireBuildIfExceedsMaxNumberOfBuildAttempts(instance *apiv1alpha1.KoupletBuild) bool {

	stateChanged := false
	buildConstraints := instance.Spec.BuildContraints

	maxNumberOfBuildAttempts := DefaultMaxNumberofBuildAttempts

	if buildConstraints != nil && buildConstraints.MaxNumberofBuildAttempts != nil {
		maxNumberOfBuildAttempts = *buildConstraints.MaxNumberofBuildAttempts
	}

	// Fail if exceeded max number of build attempts
	if len(buildStatus.completedAttempts) >= maxNumberOfBuildAttempts {
		buildStatus.updateState(buildStatusStateExpired)
		boolFalse := false
		buildStatus.successResult = &boolFalse
		now := time.Now()
		buildStatus.completionTime = &now
		shared.LogError(fmt.Errorf("buildStatus %s exceeded maximum number of build attempts", buildStatus.shortString()))
		stateChanged = true
	}

	return stateChanged
}

// Check max overall build time
func (buildStatus *buildStatus) expireBuildIfExceedsMaxOverallBuildTime(instance *apiv1alpha1.KoupletBuild) bool {

	stateChanged := false
	buildConstraints := instance.Spec.BuildContraints

	maxOverallBuildTimeInSeconds := DefaultMaxOverallBuildTimeInSeconds

	if buildConstraints != nil && buildConstraints.MaxOverallBuildTimeInSeconds != nil {
		maxOverallBuildTimeInSeconds = *buildConstraints.MaxOverallBuildTimeInSeconds
	}

	// Fail if overall build didn't complete in time
	expireTime := buildStatus.kubeCreationTime.Add(time.Duration(maxOverallBuildTimeInSeconds) * time.Second)

	if time.Now().After(expireTime) {

		// Set state to expired
		buildStatus.updateState(buildStatusStateExpired)

		// Set completion time
		completionTime := time.Now()
		buildStatus.completionTime = &completionTime

		boolFalse := false
		buildStatus.successResult = &boolFalse
		stateChanged = true

		// No need to clean up active jobs; this is the responsibility of caller
		shared.LogError(fmt.Errorf("Build expired %s", buildStatus.shortString()))

	}

	return stateChanged
}

func updateKubeStatus(buildStatus *buildStatus, ctx KBContext) {

	// Update kube status
	kubeActiveAttempts := []apiv1alpha1.KoupletBuildAttempt{}
	kubeCompletedAttempts := []apiv1alpha1.KoupletBuildAttempt{}

	for _, buildAttempt := range buildStatus.completedAttempts {
		koupletBuildAttempt := buildAttempt.convertToKubeObject()
		kubeCompletedAttempts = append(kubeCompletedAttempts, *koupletBuildAttempt)
	}

	for _, buildAttempt := range buildStatus.activeAttempts {
		koupletBuildAttempt := buildAttempt.convertToKubeObject()
		kubeActiveAttempts = append(kubeActiveAttempts, *koupletBuildAttempt)
	}

	instance, err := getInstance(buildStatus.kubeObj.Name, buildStatus.kubeObj.Namespace, ctx)
	if err != nil {
		if kubeErrors.IsNotFound(err) {
			shared.LogError(fmt.Errorf("Unable to locate kube resource for '%s'", buildStatus.shortString()))
		} else {
			shared.LogErrorMsg(err, "Error while retrieving kube resource for '"+buildStatus.shortString()+"'")
		}
		return
	}

	instance.Status = apiv1alpha1.KoupletBuildStatus{
		Status:                 convertBuildStatusStateToString(buildStatus.state),
		CompletedBuildAttempts: kubeCompletedAttempts,
		ActiveBuildAttempts:    kubeActiveAttempts,
	}

	err = (*(ctx.Client)).Status().Update(context.TODO(), instance)
	if err != nil {
		shared.LogErrorMsg(err, "Unable to update kube obj for "+instance.Name+" ("+string(instance.UID)+")")
	} else {
		shared.LogDebug("Successfully updated " + instance.Name + " (" + string(instance.UID) + ")")
	}

}

func getContainerExitCode(jobUID string, ctx KBContext) *int32 {
	// Locate the exit code of the container in the pod in the job
	opts := []client.ListOption{
		client.InNamespace(ctx.Namespace),
	}
	var podList corev1.PodList
	err := (*ctx.Client).List(context.TODO(), &podList, opts...)
	if err != nil {
		shared.LogErrorMsg(err, "Unable to list pods")
	}

	var containerExitCode *int32 = nil

	for _, pod := range podList.Items {

		podName := ""
		for _, ownerRef := range pod.OwnerReferences {

			if string(ownerRef.UID) == jobUID {
				podName = pod.Name
			}

		}

		if podName == "" {
			continue
		}

		var matchingPod corev1.Pod
		err := (*ctx.Client).Get(context.TODO(), types.NamespacedName{
			Namespace: ctx.Namespace,
			Name:      podName,
		}, &matchingPod)

		if err != nil {
			shared.LogErrorMsg(err, "Unable to locate pod with name "+podName)
			break
		}

		// This pod is owned by the job
		for _, containerStatus := range matchingPod.Status.ContainerStatuses {

			terminated := containerStatus.State.Terminated
			if terminated != nil {
				containerExitCode = &terminated.ExitCode
			}
		}

		break
	}

	return containerExitCode
}

func newJobFor(attemptNumber int, cr *apiv1alpha1.KoupletBuild) batchv1.Job {

	defaultVolumeMode := int32(0600)

	jobName := fmt.Sprintf("builder-job-%s-attempt-%d", cr.Name, attemptNumber)

	// oneCompletion := int32(1)
	oneParallelism := int32(1)

	zeroBackoffLimit := int32(0)

	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: cr.Namespace,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &zeroBackoffLimit,
			Completions:  nil,
			Parallelism:  &oneParallelism,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      jobName + "-pod",
					Namespace: cr.Namespace,
				},
				Spec: corev1.PodSpec{
					// ServiceAccountName: "kouplet",
					Volumes: []corev1.Volume{
						{
							Name: "git-ssh-key-secret-volume",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  cr.Spec.GitRepo.CredentialsSecretName,
									DefaultMode: &defaultVolumeMode,
								},
							},
						},
						{
							Name: "buildah-volume",
						},
					},
					Containers:    []corev1.Container{newContainersFor(cr)},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}

	return job
}

func newContainersFor(cr *apiv1alpha1.KoupletBuild) corev1.Container {

	privilegedVal := true

	envVars := []corev1.EnvVar{}

	for index, url := range cr.Spec.Urls {
		envVars = append(envVars, corev1.EnvVar{
			Name:  fmt.Sprintf("kouplet-url-%d", index+1),
			Value: url,
		})
	}

	// Add username from secret
	envVars = append(envVars, corev1.EnvVar{
		Name: "kouplet-registry-username",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: cr.Spec.ContainerRegistry.CredentialsSecretName,
				},
				Key: "username",
			},
		},
	})

	// Add password from secret
	envVars = append(envVars, corev1.EnvVar{
		Name: "kouplet-registry-password",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: cr.Spec.ContainerRegistry.CredentialsSecretName,
				},
				Key: "password",
			},
		},
	})

	envVars = append(envVars, corev1.EnvVar{
		Name: "kouplet-registry-host",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: cr.Spec.ContainerRegistry.CredentialsSecretName,
				},
				Key: "registry",
			},
		},
	})

	// Add remaining variables
	envVars = append(envVars, corev1.EnvVar{
		Name:  "kouplet-image-git-repository-url",
		Value: cr.Spec.GitRepo.URL,
	})

	envVars = append(envVars, corev1.EnvVar{
		Name:  "kouplet-image",
		Value: cr.Spec.Image,
	})

	if cr.Spec.GitRepo.SubPath != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "kouplet-repository-subpath",
			Value: cr.Spec.GitRepo.SubPath,
		})
	}

	builderImage := DefaultBuilderImageName + ":" + DefaultBuilderImageTag
	if cr.Spec.BuilderImage != "" {
		builderImage = cr.Spec.BuilderImage
	}

	container := corev1.Container{
		Name:  "builder-container",
		Image: builderImage,
		Env:   envVars,
		SecurityContext: &corev1.SecurityContext{
			Privileged: &privilegedVal,
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "git-ssh-key-secret-volume",
				ReadOnly:  true,
				MountPath: "/etc/git-ssh-key-secret-volume",
			},
			{
				Name:      "buildah-volume",
				MountPath: "/var/lib/containers",
			},
		},
	}
	return container
}

func (buildStatus *buildStatus) newBuildAttempt(ctx KBContext, kubeObj apiv1alpha1.KoupletBuild) (*buildAttempt, error) {

	currentAttempt := len(buildStatus.completedAttempts) + 1

	job := newJobFor(currentAttempt, &buildStatus.kubeObj)

	// Set KoupletBuild instance as the owner and controller
	if err := controllerutil.SetControllerReference(&kubeObj, &job, ctx.Scheme); err != nil {
		shared.LogErrorMsg(err, "Unable set controller ref for newBuildAttempt")
		return nil, err
	}

	newClient := ctx.Client

	err := (*newClient).Create(context.TODO(), &job)
	if err != nil {
		shared.LogErrorMsg(err, "Unable to create job for newBuildAttempt")
		return nil, err
	}

	result := &buildAttempt{
		jobName:      job.Name,
		jobUID:       string(job.UID),
		jobQueueTime: time.Now(),
	}

	// Sleep after creation: there is some kind of race condition in the k8s client (or some k8s instances) where (client).Get(...) will not find the job
	// immediately after it is created.
	time.Sleep(DelayOnKubernetesResourceCreation)

	return result, nil

}

func (buildStatus *buildStatus) updateState(state buildStatusState) {
	buildStatus.state = state
	shared.LogInfo("State of '" + buildStatus.shortString() + "' is now " + convertBuildStatusStateToString(state))
}

func (buildStatus *buildStatus) shortString() string {
	return fmt.Sprintf("%s (%s)", buildStatus.name, buildStatus.kubeObj.UID)
}

type buildAttempt struct {
	jobName string
	jobUID  string

	// jobQueueTime is the time at which the Job object was created on the cluster
	jobQueueTime      time.Time
	jobStartTime      *time.Time
	jobCompletionTime *time.Time

	jobSucceeded *bool
}

func convertBuildStatusStateToString(state buildStatusState) string {
	return string(state)
}

func convertStringToBuildStatusState(stateString string) (buildStatusState, error) {

	if stateString == string(buildStatusStateWaiting) {
		return buildStatusStateWaiting, nil

	} else if stateString == string(buildStatusStateRunning) {
		return buildStatusStateRunning, nil

	} else if stateString == string(buildStatusStateComplete) {
		return buildStatusStateComplete, nil

	} else if stateString == string(buildStatusStateExpired) {
		return buildStatusStateExpired, nil
	}

	err := fmt.Errorf("Invalid state '%s'", stateString)
	shared.LogSevere(err)
	return "", err

}

func (buildAttempt *buildAttempt) shortString() string {
	return buildAttempt.jobName
}

func (buildAttempt *buildAttempt) convertToKubeObject() *apiv1alpha1.KoupletBuildAttempt {

	var jobStartTime *int64 = nil
	if buildAttempt.jobStartTime != nil {
		jobStartTimeVal := buildAttempt.jobStartTime.UnixNano() / 1000000
		jobStartTime = &jobStartTimeVal
	}

	var jobCompletionTime *int64 = nil
	if buildAttempt.jobCompletionTime != nil {
		jobCompletionTimeVal := buildAttempt.jobCompletionTime.UnixNano() / 1000000
		jobCompletionTime = &jobCompletionTimeVal
	}

	koupletBuildAttempt := &apiv1alpha1.KoupletBuildAttempt{
		JobName:           buildAttempt.jobName,
		JobUID:            buildAttempt.jobUID,
		JobQueueTime:      buildAttempt.jobQueueTime.UnixNano() / 1000000,
		JobStartTime:      jobStartTime,
		JobCompletionTime: jobCompletionTime,
		JobSucceeded:      buildAttempt.jobSucceeded,
	}

	return koupletBuildAttempt
}

func prettyPrint(obj interface{}) string {
	val, _ := json.MarshalIndent(obj, "", "    ")
	return string(val)
}
