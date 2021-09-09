package kouplettest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	"time"
	"unicode"

	apiv1alpha1 "github.com/jgwest/kouplet-operator/api/v1alpha1"

	"github.com/google/uuid"
	"github.com/jgwest/kouplet-operator/controllers/shared"
	gojunit "github.com/joshdk/go-junit"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// KoupletDB is a single instance of the internal database, shared by all KoupletTest objects that are created.
type KoupletDB struct {
	waitingCollections   []*TestCollection
	runningCollections   []*TestCollection
	completedCollections []*TestCollection
	collections          map[string] /* kube obj uid -> */ *TestCollection

	persistenceContext PersistenceContext

	testStartTimes []time.Time

	scheduledTasks ScheduledTasks
}

func (db *KoupletDB) Process(ctx KTContext) error {

	// log.Info("==================================================")

	// List of test collections that changed during this process(...) call
	stateChanged := map[string] /* TestCollection UUID -> */ *TestCollection{}

	for {
		// log.Info("------------------------------------------------")
		updatedTestCollections := db.subProcess(ctx)

		// For each test collection that was mutated, add it to the stateChangedMap
		if len(updatedTestCollections) > 0 {
			for _, updatedTC := range updatedTestCollections {
				stateChanged[updatedTC.uuid] = updatedTC
			}

			time.Sleep(1000 * time.Millisecond)
		} else {
			break
		}
	}

	// For each running collection, if it is complete then move it to completedCollections
	{
		newRunningCollections := []*TestCollection{}

		collectionMoved := false

		for _, runningCollection := range db.runningCollections {

			if !runningCollection.isComplete() {
				newRunningCollections = append(newRunningCollections, runningCollection)
			} else {
				// Remove from runningCollections and add to completed.
				shared.LogInfo("Moving running collection to completed: " + runningCollection.toShortString())
				db.completedCollections = append(db.completedCollections, runningCollection)

				runningCollection.markAsCompleteAndPersist(db)
				collectionMoved = true
				stateChanged[runningCollection.uuid] = runningCollection
			}
		}

		if collectionMoved {
			db.cleanupOldTestStartTimes()
		}

		db.runningCollections = newRunningCollections
	}

	// GC collections which do not have a corresponding kube object
	db.cleanupOldTestCollections(ctx)

	db.persistenceContext.resolve(db)

	db.scheduledTasks.runTasksIfNeeded(db, ctx)

	// For each test collection that changed, update the corresponding Kubernetes object
	for _, testCollection := range stateChanged {

		shared.LogDebug("Updating testCollection " + testCollection.uuid)

		// Reacquire the instance object of the updated tC
		instance := &apiv1alpha1.KoupletTest{}
		err := (*ctx.Client).Get(context.TODO(), types.NamespacedName{
			Name:      testCollection.kubeObj.ObjectMeta.Name,
			Namespace: testCollection.kubeObj.ObjectMeta.Namespace,
		}, instance)
		if err != nil {
			shared.LogError(err)
			// Error reading the object - requeue the request.
			return err
		}

		// Ensure we are not updating an old object with the same name
		if instance.UID != testCollection.kubeObj.UID {
			shared.LogDebug("Kubernetes UID did not match test collection UID, for " + testCollection.toShortString())
			continue
		}

		instance.Status = testCollection.generateStatus()
		err = (*ctx.Client).Status().Update(context.TODO(), instance)
		if err != nil {
			shared.LogErrorMsg(err, "Failed to update status")
			return err
		}
	}

	return nil
}

func (db *KoupletDB) subProcess(ctx KTContext) []*TestCollection {

	// Process each of the running collections, and also keep track of how many have queued tests.
	testsInQueue := 0
	for index, runningTestCollection := range db.runningCollections {

		processStateChanged, err := runningTestCollection.process(db, ctx)

		if err != nil {
			shared.LogErrorMsg(err, "Error from test collection process for "+runningTestCollection.toShortString())
		}

		// If this is the first time we are seeing this collection in the first position, then set start time
		if index == 0 && runningTestCollection.startTime == nil {
			runningTestCollection.markAsPrimaryTestCollectionAndPersist(db)
		}

		if processStateChanged {
			shared.LogInfo("Test collection state change: " + runningTestCollection.toString())
			return []*TestCollection{runningTestCollection}
		}

		testsInQueue += len(runningTestCollection.queuedTests) + len(runningTestCollection.testsForRetry)

	}

	// If none of the other running collections have any waiting items, then move another
	// test collection into running.
	if testsInQueue == 0 && len(db.waitingCollections) > 0 {

		newRunning := db.waitingCollections[0]
		db.waitingCollections = db.waitingCollections[1:]

		db.runningCollections = append(db.runningCollections, newRunning)

		newRunning.markAsRunningAndPersist(db)

		shared.LogInfo("Moved " + newRunning.toShortString() + " to running collections.")

		return []*TestCollection{newRunning}
	}

	return []*TestCollection{}
}

var newDB = false

func NewKoupletDB(pc *PersistenceContext, context KTContext) *KoupletDB {

	shared.LogInfo(fmt.Sprintf("Creating KoupletDB %v", newDB))

	db := (*KoupletDB)(nil)

	// TODO: LOWER - Do something better with this
	if newDB {
		db = &KoupletDB{
			persistenceContext: *pc,
			collections:        make(map[string]*TestCollection),
			testStartTimes:     []time.Time{},
		}
	} else {
		var err error
		db, err = pc.load(context)
		if err != nil {
			shared.LogSevere(err)
			log.Fatal("Exiting...")
			return nil
		}
	}

	{
		timeBetweenRunsVal := time.Minute * 30
		db.scheduledTasks.addTasks(ScheduledTaskEntry{
			name:              "Clean old jobs ands pods",
			timeBetweenRuns:   timeBetweenRunsVal,
			nextScheduledTime: time.Now().Add(timeBetweenRunsVal),
			fxn:               cleanupOldJobs,
		})
	}

	{
		timeBetweenRunsVal := time.Minute * 30
		gap := time.Minute * 5
		db.scheduledTasks.addTasks(ScheduledTaskEntry{
			name:              "Remove old database keys",
			timeBetweenRuns:   timeBetweenRunsVal,
			nextScheduledTime: time.Now().Add(timeBetweenRunsVal).Add(gap),
			fxn:               taskRemoveOldDatabaseKeys,
		})
	}

	return db

}

func (db *KoupletDB) addToBucketForPersistence(tc *TestCollection) {

	db.collections[tc.uuid] = tc

	switch tc.bucket {
	case testCollectionBucketWaiting:
		db.waitingCollections = append(db.waitingCollections, tc)
	case testCollectionBucketRunning:
		db.runningCollections = append(db.runningCollections, tc)
	case testCollectionBucketCompleted:
		db.completedCollections = append(db.completedCollections, tc)
	default:
		shared.LogSevere(errors.New("Test bucket type not found"))
	}

}

// GetOrCreateCollection ...
func (db *KoupletDB) GetOrCreateCollection(kubeObj apiv1alpha1.KoupletTest) *TestCollection {

	existingCollection := db.collections[string(kubeObj.UID)]
	if existingCollection != nil {
		return existingCollection
	}

	result := newTestCollection(kubeObj)
	db.waitingCollections = append(db.waitingCollections, result)
	db.collections[string(kubeObj.UID)] = result

	db.persistenceContext.updateTestCollectionShallow(result)

	spec := kubeObj.Spec

	for _, test := range spec.Tests {

		curr := &TestEntry{
			id:              result.getAndIncrementNextTestEntryID(),
			retryFromID:     nil,
			status:          testEntryStatusWaiting,
			result:          nil,
			testEntry:       test,
			processed:       false,
			clusterResource: nil,
			bucket:          testEntryBucketQueued,
		}

		result.addToQueuedTests(curr)
		result.addToTestEntries(curr)

		db.persistenceContext.updateTestEntryShallow(curr, result)
		db.persistenceContext.updateTestEntryTestEntry(curr, result)

		shared.LogInfo("Creating new queued test entry: " + curr.toShortString())

	}

	shared.LogInfo("New test collection created: " + result.toShortString())

	db.persistenceContext.resolve(db)

	return result
}

// Look through the internal DB for test collections with kube objects that no longer exist; delete those
// TCs (and children) from internal and persisted DB.
//
// NOTE: Removing old persisted database keys is handled by 'taskRemoveOldDatabaseKeys'; this is a small amount
// overlap between this function and that one.
func (db *KoupletDB) cleanupOldTestCollections(ctx KTContext) {

	collectionsToDelete := []*TestCollection{}

	// For each test collection which SHOULD have a corresponding KoupletTest object
	for _, coll := range db.collections {

		// Fetch the KoupletTest instance
		instance := &apiv1alpha1.KoupletTest{}
		err := (*ctx.Client).Get(context.TODO(), types.NamespacedName{
			Namespace: coll.kubeObj.Namespace,
			Name:      coll.kubeObj.Name,
		}, instance)

		// If it doesn't exist, then we remove it from our database
		if err != nil && kubeerrors.IsNotFound(err) {
			collectionsToDelete = append(collectionsToDelete, coll)
		}
	}

	// If we couldn't find a corresponding kube object, then delete the collection.
	for _, coll := range collectionsToDelete {
		shared.LogInfo("DB Garbage collecting old test collection: " + coll.toShortString())
		delete(db.collections, coll.uuid)

		// Remove from internal DB
		db.waitingCollections = removeByUUID(coll.uuid, db.waitingCollections)
		db.runningCollections = removeByUUID(coll.uuid, db.runningCollections)
		db.completedCollections = removeByUUID(coll.uuid, db.completedCollections)

		db.persistenceContext.deleteTestCollection(coll)
	}

}

// Cleanup jobs/pods for tests that completed naturally, and completed > 2 minutes ago.
// Cleanup jobs/pods that do not have a corresponding test collection in the DB.
// This will only cleanup non-expired jobs; expired jobs are cleaned up via a separate process.
func cleanupOldJobs(db *KoupletDB, ctx KTContext) error {

	jobList := &batchv1.JobList{}
	opts := []client.ListOption{
		client.InNamespace(ctx.Namespace),
	}

	if err := (*(ctx.Client)).List(context.TODO(), jobList, opts...); err != nil {
		return err
	}

	podList := &corev1.PodList{}
	if err := (*(ctx.Client)).List(context.TODO(), podList, opts...); err != nil {
		return err
	}

	for _, job := range getKoupletJobs(jobList, nil) {

		jobCollectionUUID, exists := job.Labels[LabelCollectionUUID]
		if !exists {
			continue
		}

		jobTestEntryID, err := strconv.Atoi(job.Labels[LabelTestEntryID])
		if err != nil {
			shared.LogErrorMsg(err, "Invalid test entry id")
			return err
		}

		delete := true

		te := (*TestEntry)(nil)

		tc, exists := db.collections[jobCollectionUUID]
		if exists {
			te = tc.GetTestEntry(jobTestEntryID)

			if te == nil {
				shared.LogError(fmt.Errorf("Unable to locate test entry %d in test collection", jobTestEntryID))
			} else {
				// The test collection exists in our database, AND so does the test entry id

				// Prerequisite for deletion: the Job must be completed, AND it must have completed
				// longer than 2 minutes ago.
				if job.Status.CompletionTime == nil || !job.Status.CompletionTime.Time.Before(time.Now().Add(-2*time.Minute)) {
					delete = false
				}

				// We should not delete jobs until we have uploaded their logs/test results.
				if !te.processed {
					delete = false
				}
			}
		} else {
			// If the database did not have a record of this test collection, then delete all of its jobs and pods.
			delete = true
		}

		if delete {
			deleteJobAndPods(&job, jobCollectionUUID, jobTestEntryID, podList, ctx.Client)

			if te != nil && tc != nil {
				te.clusterResource = nil
				db.persistenceContext.updateTestEntryShallow(te, tc)
			}
		}

	}

	return nil
}

// Cleanup testStartTimes that are older than an hour
func (db *KoupletDB) cleanupOldTestStartTimes() {

	beforeTime := time.Now().Add(time.Minute * -60)

	filtered := []time.Time{}

	unfiltered := []time.Time{}

	// Locate the tests that have run within the last X seconds (everyXSeconds)
	for _, v := range db.testStartTimes {

		if v.After(beforeTime) {
			unfiltered = append(unfiltered, v)
		} else {
			filtered = append(filtered, v)
		}
	}

	// Remove old values from persistence
	for _, v := range filtered {
		db.persistenceContext.removeTestStartTime(v)
	}

	db.testStartTimes = unfiltered

}

func (db *KoupletDB) canWeRunMoreTests(maxTestsToRunInPeriod int, everyXSeconds int) bool {

	beforeTime := time.Now().Add(time.Second * -1 * time.Duration(everyXSeconds))

	count := 0

	// Locate the tests that have run within the last X seconds (everyXSeconds)
	for _, v := range db.testStartTimes {

		if v.After(beforeTime) {
			count++
		}
	}

	// We should run more tests if the # started < numTestsToRun, within the specified period
	return count < maxTestsToRunInPeriod

}

func (db *KoupletDB) addTestStartTimeAndPersist() {
	testStartTime := time.Now()
	db.testStartTimes = append(db.testStartTimes, testStartTime)
	db.persistenceContext.addTestStartTime(testStartTime)
}

func (db *KoupletDB) addTestStartTimeForPersistence(testStartTimeInUnixEpochMsecs int64) {
	timeVal := time.Unix(0, testStartTimeInUnixEpochMsecs*1000000)
	db.testStartTimes = append(db.testStartTimes, timeVal)
}

// TestCollection ...
type TestCollection struct {
	kubeObj apiv1alpha1.KoupletTest

	queuedTests    []*TestEntry
	runningTests   []*TestEntry
	completedTests []*TestEntry
	testsForRetry  []*TestEntry
	testEntries    map[int]*TestEntry

	nextTestEntryID int
	uuid            string

	status testCollectionStatus

	dateSubmitted *int64

	startTime *int64
	endTime   *int64

	bucket testCollectionBucket
}

func (tc *TestCollection) addToTestEntriesForPersistence(te *TestEntry) {

	tc.testEntries[te.id] = te

	switch te.bucket {
	case testEntryBucketQueued:
		tc.queuedTests = append(tc.queuedTests, te)
	case testEntryBucketRunning:
		tc.runningTests = append(tc.runningTests, te)
	case testEntryBucketRetry:
		tc.testsForRetry = append(tc.testsForRetry, te)
	case testEntryBucketCompleted:
		tc.completedTests = append(tc.completedTests, te)
	default:
		shared.LogSevere(errors.New("Did not recognize test entry bucket entry"))
	}

}

// Caller should ensure te is persisted
func (tc *TestCollection) addToTestEntries(te *TestEntry) {
	tc.testEntries[te.id] = te
}

// Caller should ensure te is persisted
func (tc *TestCollection) addToQueuedTests(te *TestEntry) {
	tc.queuedTests = append(tc.queuedTests, te)
}

func (tc *TestCollection) generateStatus() apiv1alpha1.KoupletTestStatus {

	// Calculate the percent by looking for the retry with the highest percentage, and using
	// that rather than the parent percent where necessary
	percent := float32(0)
	{
		myMap := make(map[int] /* test entry id*/ int /* percent */)

		for k, v := range tc.testEntries {

			if k != v.id { // Sanity check
				shared.LogError(errors.New("Key id did not match value id: " + v.toShortString()))
				continue
			}

			currEntryPercent := 0
			if v.result != nil && v.result.Percent != nil {
				currEntryPercent = *(v.result.Percent)
			}

			parent := getTestEntryRetry(*v, tc)

			parentMapEntry, exists := myMap[parent.id]
			if !exists {
				myMap[parent.id] = currEntryPercent
			} else if currEntryPercent > parentMapEntry {
				// Only update the map if the current entry is larger
				myMap[parent.id] = currEntryPercent
			}

		} // end outer for

		numTests := 0
		success := 0
		for k, v := range myMap {

			testEntry := tc.GetTestEntry(k)
			if testEntry == nil || testEntry.retryFromID != nil {
				continue
			}

			if v == 100 {
				success++
			}
			numTests++

		}

		percent = float32(100) * float32(success) / float32(numTests)

	}

	intPercent := int(percent)

	allResults := []apiv1alpha1.KoupletTestJobResultEntry{}
	for _, v := range tc.testEntries {

		if v.result == nil {
			// If a test is running, and has a cluster resource, then generate a short status for it,
			// so that any consumers can tail the job/pod logs.
			if v.clusterResource != nil {
				crResult := &apiv1alpha1.KoupletTestJobResultEntry{
					ID:              v.id,
					Test:            v.testEntry.Values,
					Labels:          v.testEntry.Labels,
					Status:          testEntryStatusToString(v.status),
					ClusterResource: v.clusterResource,
				}
				allResults = append(allResults, *crResult)
			} else {
				continue
			}
		} else {
			allResults = append(allResults, *v.result)
		}

	}

	result := apiv1alpha1.KoupletTestStatus{
		Status:        testCollectionStatusToString(tc.status),
		DateSubmitted: tc.dateSubmitted,
		Percent:       &intPercent,

		StartTime: tc.startTime,
		EndTime:   tc.endTime,

		Results: allResults,
	}

	return result

}

func getTestEntryRetry(entryParam TestEntry, tc *TestCollection) *TestEntry {

	lastEntrySeen := &entryParam

	for {

		if lastEntrySeen.retryFromID == nil {
			return lastEntrySeen
		}

		result, exists := tc.testEntries[*lastEntrySeen.retryFromID]
		if !exists {
			return lastEntrySeen
		}

		lastEntrySeen = result

	}

}

func (tc *TestCollection) toString() string {
	result := ""

	result += tc.toShortString()

	result += fmt.Sprintf(", queued tests: %d, running tests: %d, retry tests: %d, completed tests: %d", len(tc.queuedTests), len(tc.runningTests), len(tc.testsForRetry), len(tc.completedTests))

	return result
}

func (tc *TestCollection) toShortString() string {
	result := ""

	result += "k8s-name: " + tc.kubeObj.Name + " (tc-uuid: " + tc.uuid + ")"

	return result
}

func (tc *TestCollection) process(kdb *KoupletDB, ctx KTContext) (bool, error) {

	jobList := &batchv1.JobList{}
	opts := []client.ListOption{
		client.InNamespace(tc.kubeObj.Namespace),
	}

	if err := (*(ctx.Client)).List(context.TODO(), jobList, opts...); err != nil {
		return false, err
	}

	podList := &corev1.PodList{}
	if err := (*(ctx.Client)).List(context.TODO(), podList, opts...); err != nil {
		return false, err
	}

	stateChanged := false

	// Handle completed and expired test jobs
	runningStateChanged, err := tc.processRunningJobs(jobList, podList, kdb, ctx)
	if runningStateChanged {
		stateChanged = true
	}
	if err != nil {
		shared.LogErrorMsg(err, "Error from processRunningJobs, continuing.")
	}

	// Start new tests
	if !stateChanged {
		waitingStateChanged, err := tc.processWaitingTests(jobList, podList, kdb, ctx)

		if waitingStateChanged {
			stateChanged = true
		}

		if err != nil {
			shared.LogErrorMsg(err, "Error from processWaitingTests, continuing.")
		}

	}

	// If the test collection state was updated, update the status

	return stateChanged, nil

}

// Move queued/retry tests to running, returns whether the state changed
func (tc *TestCollection) processWaitingTests(jobList *batchv1.JobList, podList *corev1.PodList, kdb *KoupletDB, ctx KTContext) (bool, error) {

	// Do NOT add any returns or breaks to this function without carefully considering the login.
	// - Ensure that testEntries that are removed from tc.queuedTests or tc.waitingTests are restored if
	//   they were removed earlier in the function. See the comment 'On failure, add the removed element to the end of the original slice'

	stateChanged := false

	// Determine number of nodes and # of tests to run
	numberOfNodes := DefaultNumberOfNodes
	testsToRunTotal := 0
	{
		testsToRun := DefaultTestsToRun
		if tc.kubeObj.Spec.MaxActiveTests != nil {
			testsToRun = *tc.kubeObj.Spec.MaxActiveTests
		}

		if tc.kubeObj.Spec.NumberOfNodes != nil && *(tc.kubeObj.Spec.NumberOfNodes) > 0 {
			numberOfNodes = *(tc.kubeObj.Spec.NumberOfNodes)
		}

		testsToRunTotal = testsToRun * numberOfNodes

		existingJobs := 0
		for _, v := range getKoupletJobs(jobList, &tc.uuid) {
			// Only count jobs that have not completed
			if v.Status.CompletionTime == nil {
				existingJobs++
			}
		}

		testsToRunTotal -= existingJobs
	}

	timeBetweenTestRunsInSeconds := DefaultTimeBetweenTestRunsInSeconds
	if tc.kubeObj.Spec.MinimumTimeBetweenTestRuns != nil {
		timeBetweenTestRunsInSeconds = *tc.kubeObj.Spec.MinimumTimeBetweenTestRuns
	}

	for testsToRunTotal > 0 && kdb.canWeRunMoreTests(numberOfNodes, timeBetweenTestRunsInSeconds) {

		// toRun is the next test that we have found to run
		toRun := (*TestEntry)(nil)

		// Whether the toRun came from the 'queuedTests' bucket
		fromQueuedTests := false

		// First go through queued tests and create new jobs
		for index, queuedTest := range tc.queuedTests {

			// Sanity test each of the entries
			if queuedTest.status != testEntryStatusWaiting {
				shared.LogError(errors.New("Invalid test status: " + queuedTest.toString()))
				continue
			}

			shared.LogInfo("Moving test to running from queuedTests: " + queuedTest.toString())

			// preRemoval = append([]*TestEntry(nil), tc.queuedTests...)
			tc.queuedTests = append(tc.queuedTests[:index], tc.queuedTests[index+1:]...)
			toRun = queuedTest
			fromQueuedTests = true
			break
		}

		// If no tests found in queued tests, try 'tests for retry' next
		if toRun == nil {
			for index, retryTest := range tc.testsForRetry {

				// Sanity test each of the entries
				if retryTest.status != testEntryStatusWaiting {
					shared.LogError(errors.New("Invalid test status: " + retryTest.toString()))
					continue
				}

				shared.LogInfo("Moving test to running from testsForRetry: " + retryTest.toString())

				// preRemoval = append([]*TestEntry(nil), tc.testsForRetry...)
				tc.testsForRetry = append(tc.testsForRetry[:index], tc.testsForRetry[index+1:]...)
				toRun = retryTest
				fromQueuedTests = false
				break
			}
		}

		if toRun == nil {
			// No tests to run, exit the outer for loop
			break
		}

		// Create the object, but don't yet pass it to Kube client
		newJob := createNewKubeJob(toRun.id, tc, &tc.kubeObj, &toRun.testEntry)

		err := error(nil)

		// Fetch the KoupletTest instance
		instance := &apiv1alpha1.KoupletTest{}
		err = (*(ctx.Client)).Get(context.TODO(), types.NamespacedName{
			Namespace: tc.kubeObj.Namespace,
			Name:      tc.kubeObj.Name,
		}, instance)

		if err != nil {
			shared.LogErrorMsg(err, "Unable to locate kube object by name, so could not create job")
		}

		if err == nil {
			if string(instance.UID) != tc.uuid {
				err = fmt.Errorf("UID of KoupletTest (%s) did not match the uuid of the test collection (%s)", string(instance.UID), tc.uuid)
				shared.LogError(err)
			}
		}

		if err == nil {
			// Set KoupletTest instance as the owner and controller
			if err = controllerutil.SetControllerReference(instance, newJob, ctx.Scheme); err != nil {
				shared.LogErrorMsg(err, "Unable to set controller ref for "+toRun.toShortString())
			}
		}

		if err == nil {
			shared.LogInfo("Creating a new Job " + newJob.Name)
			err = (*(ctx.Client)).Create(context.TODO(), newJob)
		}

		if err != nil {

			// On failure, add the removed element to the end of the original slice
			if fromQueuedTests {
				tc.queuedTests = append(tc.queuedTests, toRun)
			} else {
				tc.testsForRetry = append(tc.testsForRetry, toRun)
			}

			return true, err
		}

		toRun.clusterResource = &apiv1alpha1.KoupletTestJobResultClusterResource{
			Namespace: newJob.Namespace,
			JobName:   newJob.Name,
		}

		kdb.addTestStartTimeAndPersist()

		tc.runningTests = append(tc.runningTests, toRun)
		toRun.markAsRunningAndPersist(tc, kdb)

		stateChanged = true

		testsToRunTotal--
	}

	return stateChanged, nil
}

func createNewKubeJob(testEntryID int, tc *TestCollection, cr *apiv1alpha1.KoupletTest, kte *apiv1alpha1.KoupletTestEntry) *batchv1.Job {
	labels := map[string]string{
		LabelCollectionUUID: tc.uuid,
		LabelTestEntryID:    strconv.Itoa(testEntryID),
	}

	jobEnv := []corev1.EnvVar{}
	{
		envMap := make(map[string]string)

		// Process the test collection env
		for _, env := range cr.Spec.Env {
			envMap[env.Name] = env.Value
		}

		// Process the test env
		for _, env := range kte.Env {
			envMap[env.Name] = env.Value
		}

		// Process the test values
		for _, env := range kte.Values {
			envMap[env.Name] = env.Value
		}

		// Convert to standard Kube env var
		for k, v := range envMap {

			env := corev1.EnvVar{
				Name:  k,
				Value: v,
			}

			jobEnv = append(jobEnv, env)
		}
	}

	sanitizedJobName := ""
	{
		for _, r := range cr.Spec.Name {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				sanitizedJobName += string(r)
			}

			if r == ' ' {
				sanitizedJobName += "-"
			}
		}

		// TODO: LOWER - Allow the user to customize the Kubernetes name, and use it here, rather than this.

		// Limit the job name to 32 characters
		if len(sanitizedJobName) > 32 {
			sanitizedJobName = sanitizedJobName[0:32]
		}

		uuidObj, _ := uuid.NewRandom()
		uuid := uuidObj.String()
		uuid = uuid[0:strings.Index(uuid, "-")]

		sanitizedJobName += "-" + uuid

		sanitizedJobName = strings.ToLower(sanitizedJobName)

	}

	command := []string(nil)
	if len(cr.Spec.Command) != 0 {
		command = cr.Spec.Command
	}

	result := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%d", sanitizedJobName, testEntryID),
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					// ServiceAccountName: "kouplet",
					Containers: []corev1.Container{
						{
							Name:    fmt.Sprintf("%s-%d-test", sanitizedJobName, testEntryID),
							Image:   cr.Spec.Image,
							Command: command,
							Env:     jobEnv,
							Resources: corev1.ResourceRequirements{
								Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("800m")},
								Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("300m")},
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}

	return result
}

// getKoupletJobs filters out non-Kouplet jobs from the given jobs list.
func getKoupletJobs(jobList *batchv1.JobList, testCollectionUUID *string) []batchv1.Job {

	result := []batchv1.Job{}

	for _, job := range jobList.Items {

		// Job must have collection UUID
		collectionUUID, exists := job.Labels[LabelCollectionUUID]
		if !exists {
			continue
		}

		// Job must match collection UUID param, if provided
		if testCollectionUUID != nil && collectionUUID != *testCollectionUUID {
			continue
		}

		// Must have a test entry ID
		value, exists := job.Labels[LabelTestEntryID]
		if !exists {
			continue
		}

		// Id must be an integer
		_, err := strconv.Atoi(value)
		if err != nil {
			continue
		}

		result = append(result, job)
	}

	return result

}

func (tc *TestCollection) processRunningJobs(jobList *batchv1.JobList, podList *corev1.PodList, kdb *KoupletDB, ctx KTContext) (bool, error) {

	stateChanged := false

	for _, job := range getKoupletJobs(jobList, &tc.uuid) {

		collectionUUID := job.Labels[LabelCollectionUUID]

		if collectionUUID != tc.uuid {
			continue
		}

		testEntryID, _ := strconv.Atoi(job.Labels[LabelTestEntryID])

		fromTestCollection := tc.GetTestEntry(testEntryID)
		if fromTestCollection == nil {
			shared.LogError(fmt.Errorf("Unable to find %d in %s", testEntryID, job.Name))
			continue
		} else if fromTestCollection.status != testEntryStatusRunning {
			// There will be jobs that are completed tests, so ignore them.
			continue
		}

		status := job.Status

		// The job has completed, with either test success or test failure.
		if status.CompletionTime != nil {

			processStateChange, err := tc.processCompletedJob(testEntryID, podList, &job, ctx, kdb)

			if processStateChange {
				stateChanged = true
			}

			if err != nil {
				shared.LogErrorMsg(err, "processCompletedJob error catcher")
				continue
			}

		} else if status.StartTime != nil {
			// The job has started, but not completed.

			elapsedDuration := time.Now().Sub((status.StartTime.Time))

			expireTimeInSeconds := DefaultExpireTimeInSeconds
			if tc.kubeObj.Spec.DefaultTestExpireTime != nil {
				expireTimeInSeconds = *tc.kubeObj.Spec.DefaultTestExpireTime // From TestCollection
			}

			jobTestEntry, exists := tc.testEntries[testEntryID]
			if !exists {

				shared.LogError(fmt.Errorf("Unable to locate %d in map for %s", testEntryID, job.Name))
				continue
			}

			if jobTestEntry.testEntry.Timeout != nil { // From Test-specific entry in kube obj
				expireTimeInSeconds = *jobTestEntry.testEntry.Timeout
			}

			// The test has expired, so nuke it
			if elapsedDuration.Seconds() > float64(expireTimeInSeconds) {

				shared.LogInfo(fmt.Sprintf("Killing test (%s), is older (%d) than expire time (%d).", job.Name, int(elapsedDuration.Seconds()), expireTimeInSeconds))

				podLogs := ""
				_, err := getPodLogsForTestEntry(collectionUUID, testEntryID, podList, &job, ctx.Client)
				if err != nil {
					shared.LogErrorMsg(err, "Unable to get pod log")
				}

				fromTestCollection.markAsExpiredAndPersist(tc, kdb)

				deleteJobAndPods(&job, collectionUUID, testEntryID, podList, ctx.Client)

				processStateChange, err := tc.handleFinishedRunningTestJob(testEntryID, podLogs, "", &job, nil, podList, ctx, kdb)

				if processStateChange {
					stateChanged = true
				}

				if err != nil {
					shared.LogErrorMsg(err, "processCompletedJob error catcher")
					continue
				}

			}
		} // end multi-branch if
	} // end for

	return stateChanged, nil
}

func (tc *TestCollection) processCompletedJob(testEntryID int, podList *corev1.PodList, job *batchv1.Job, ctx KTContext, kdb *KoupletDB) (bool, error) {

	testEntry := tc.GetTestEntry(testEntryID)
	if testEntry == nil {
		return false, fmt.Errorf("Unable to find test entry in processCompletedJob %d %s", testEntryID, tc.toString())
	}

	// Don't process a test entry more than once
	if testEntry.processed {
		return false, nil
	}

	// Extract the JUnit XML and the pod logs
	podLogs := ""
	junitXML := ""
	testSuite := (*gojunit.Suite)(nil)
	{
		podLogsSuccess, err := getPodLogsForTestEntry(tc.uuid, testEntryID, podList, job, ctx.Client)

		if err == nil {
			podLogs = podLogsSuccess

			podLogsArr := strings.Split(podLogs, "\n")

			lineSeen := false
			for _, line := range podLogsArr {

				if strings.Contains(line, "<testsuite ") {
					junitXML = ""
					lineSeen = true
				}

				if lineSeen {
					junitXML += line + "\n"
				}

				if strings.Contains(line, "</testsuite") {
					lineSeen = false
				}

			}

			suites, err := gojunit.IngestReader(strings.NewReader(junitXML))
			if err != nil {
				shared.LogErrorMsg(err, "Unable to parse the JUnit XML (this is probably fine), job "+job.Name+":"+junitXML)
			}
			if len(suites) == 1 {
				testSuite = &(suites[0])
			} else {
				shared.LogInfo(fmt.Sprintf("Unexpected number of suites in output: %d", len(suites)))
			}
		} else {
			// We still continue, even if we couldn't retrieve the pod log. It just means we won't have
			// results for this test, which is not the end of the world.
			shared.LogErrorMsg(err, "Unable to retrieve pod log")
		}

		testEntry.markAsCompleteAndPersist(tc, kdb)
	}

	return tc.handleFinishedRunningTestJob(testEntryID, podLogs, junitXML, job, testSuite, podList, ctx, kdb)

}

func getPodLogsForTestEntry(collectionUUID string, testEntryID int, podList *corev1.PodList, job *batchv1.Job, cl *client.Client) (string, error) {

	// Find the pod that corresponds to the job, by label.
	var matchingPod = (*corev1.Pod)(nil)

	for _, pod := range podList.Items {

		podCollectionUUID, exists := pod.Labels[LabelCollectionUUID]

		if exists {
			podTestEntryID, err := strconv.Atoi(pod.Labels[LabelTestEntryID])
			if err != nil {
				return "", err
			}

			if podCollectionUUID == collectionUUID && podTestEntryID == testEntryID {
				matchingPod = &pod
				break
			}

		}
	}

	if matchingPod == nil {
		err := errors.New("Unable to find matching pod")
		shared.LogErrorMsg(err, "job: "+job.Name)
		return "", err
	}

	// Extract the pod log text
	podLogsSuccess, err := getPodLogs(*matchingPod)

	return podLogsSuccess, err
}

func getPodLogs(pod corev1.Pod) (string, error) {
	podLogOpts := corev1.PodLogOptions{}

	config, err := config.GetConfig()
	// config, err := rest.InClusterConfig()
	if err != nil {
		return "error in getting config", err
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "error in getting access to K8S", err
	}

	req := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &podLogOpts)
	podLogs, err := req.Stream(context.TODO())
	if err != nil {
		return "error in opening stream", err
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "error in copy information from podLogs to buf", err
	}
	str := buf.String()

	return str, nil
}

func deleteJobAndPods(job *batchv1.Job, testCollectionUUID string, jobTestEntryID int, podList *corev1.PodList, cl *client.Client) error {

	// Delete the job
	shared.LogInfo("Deleting job: " + job.Name)
	if err := (*cl).Delete(context.TODO(), job, client.GracePeriodSeconds(5)); err != nil {
		if !kubeerrors.IsNotFound(err) {
			shared.LogErrorMsg(err, "Unable to delete job: "+job.Name)
		}
	}

	// Delete pods
	for _, pod := range podList.Items {

		podCollectionUUID, exists := pod.Labels[LabelCollectionUUID]

		if !exists {
			continue
		}

		podTestEntryID, err := strconv.Atoi(pod.Labels[LabelTestEntryID])
		if err != nil {
			shared.LogSevere(err)
			continue
		}

		if podCollectionUUID == testCollectionUUID && podTestEntryID == jobTestEntryID {
			shared.LogInfo("Deleting pod: " + pod.Name)
			if err := (*cl).Delete(context.TODO(), &pod, client.GracePeriodSeconds(5)); err != nil {

				if !kubeerrors.IsNotFound(err) {
					shared.LogErrorMsg(err, "Unable to delete pod: "+pod.Name)
				}
			}

		}

	}

	return nil
}

func (tc *TestCollection) handleFinishedRunningTestJob(testEntryID int, podLogText string, junitXMLText string, job *batchv1.Job, testSuite *gojunit.Suite, podList *corev1.PodList, ctx KTContext, kdb *KoupletDB) (bool, error) {

	podLogObject := (*string)(nil)
	junitObject := (*string)(nil)

	// Upload result and logs, if applicable
	if tc.kubeObj.Spec.ObjectStorageCredentials != nil {
		if len(podLogText) > 0 {

			podLogObjectStr := fmt.Sprintf("pod-log-%s-%d", job.Name, time.Now().Unix())

			err := uploadLogFromKubeCreds(podLogText, podLogObjectStr, *tc.kubeObj.Spec.ObjectStorageCredentials, ctx)
			if err != nil {
				shared.LogErrorMsg(err, "Unable to upload "+podLogObjectStr+" log to object storage, continuing without upload.")
			}

			podLogObject = &podLogObjectStr

		}

		if len(junitXMLText) > 0 {

			junitObjectStr := fmt.Sprintf("junit-report-%s-%d", job.Name, time.Now().Unix())

			err := uploadLogFromKubeCreds(junitXMLText, junitObjectStr, *tc.kubeObj.Spec.ObjectStorageCredentials, ctx)
			if err != nil {
				shared.LogErrorMsg(err, "Unable to upload "+junitObjectStr+" log to object storage, continuing without upload.")
			}

			junitObject = &junitObjectStr

		}
	}

	// Locate matching test entry and remove it from running tests
	completedTestEntry := (*TestEntry)(nil)
	{
		for index, rt := range tc.runningTests {
			if rt.id == testEntryID {
				tc.runningTests = append(tc.runningTests[:index], tc.runningTests[index+1:]...)
				completedTestEntry = rt
				break
			}
		}

		if completedTestEntry == nil {
			err := fmt.Errorf("Unable to find matching test for %s %d", job.Name, testEntryID)
			shared.LogError(err)
			return false, err
		}
	}

	failedTests := 0
	passedTests := 0
	percentInt := 0
	durationMsecs := int64(0)

	// TODO: LOWER - Should these be nil?
	testsuiteNumTests := 0
	testsuiteNumErrors := 0
	testsuiteNumFailures := 0

	if testSuite != nil {
		failedTests = testSuite.Totals.Failed + testSuite.Totals.Error
		passedTests = testSuite.Totals.Tests - testSuite.Totals.Failed - testSuite.Totals.Error

		percent := float32(100) * (float32(passedTests)) / float32(testSuite.Totals.Tests)

		percentInt = int(percent)

		durationMsecs = testSuite.Totals.Duration.Milliseconds()

		testsuiteNumTests = testSuite.Totals.Tests
		testsuiteNumErrors = testSuite.Totals.Error
		testsuiteNumFailures = testSuite.Totals.Failed
	}

	result := &apiv1alpha1.KoupletTestJobResultEntry{
		ID:              testEntryID,
		Test:            completedTestEntry.testEntry.Values,
		Status:          testEntryStatusToString(completedTestEntry.status),
		Percent:         &percentInt,
		NumTests:        &(testsuiteNumTests),
		NumErrors:       &(testsuiteNumErrors),
		NumFailures:     &(testsuiteNumFailures),
		Labels:          completedTestEntry.testEntry.Labels,
		TestTime:        &(durationMsecs),
		RetryFrom:       completedTestEntry.retryFromID,
		Result:          junitObject,
		Log:             podLogObject,
		ClusterResource: completedTestEntry.clusterResource,
	}

	testSuccess := passedTests > 0 && failedTests == 0

	// Update result
	completedTestEntry.result = result

	tc.completedTests = append(tc.completedTests, completedTestEntry)

	completedTestEntry.bucket = testEntryBucketCompleted

	kdb.persistenceContext.updateTestEntryResult(completedTestEntry, tc)
	kdb.persistenceContext.updateTestEntryShallow(completedTestEntry, tc)

	shared.LogInfo("Test entry moved to completedTests: " + completedTestEntry.toShortString())

	// If the test failed, create a new test entry
	if !testSuccess {

		// How many times have we already retried this test?
		numRetries := 0
		{

			currEntry := completedTestEntry
			for {
				nextID := currEntry.retryFromID
				if nextID == nil {
					break
				}
				numRetries++
				currEntry = tc.GetTestEntry(*nextID)
			}
		}

		failureRetries := tc.kubeObj.Spec.FailureRetries
		if failureRetries == nil {
			defaultRetries := DefaultTestRetries
			failureRetries = &defaultRetries
		}

		// Add tests to retry if there are retries remaining
		if numRetries < *failureRetries {
			newTestEntry := &TestEntry{
				id:              tc.getAndIncrementNextTestEntryID(),
				retryFromID:     &(testEntryID),
				status:          testEntryStatusWaiting,
				result:          nil,
				testEntry:       completedTestEntry.testEntry,
				processed:       false,
				clusterResource: nil,
				bucket:          testEntryBucketRetry,
			}

			tc.testEntries[newTestEntry.id] = newTestEntry

			tc.testsForRetry = append(tc.testsForRetry, newTestEntry)

			kdb.persistenceContext.updateTestEntryShallow(newTestEntry, tc)
			kdb.persistenceContext.updateTestEntryTestEntry(newTestEntry, tc)

			shared.LogInfo("Creating new retry test entry: " + newTestEntry.toShortString())
		}
	}

	return true, nil
}

func (tc *TestCollection) isComplete() bool {

	return len(tc.queuedTests) == 0 && len(tc.runningTests) == 0 && len(tc.testsForRetry) == 0

}

func (tc *TestCollection) getAndIncrementNextTestEntryID() int {
	result := tc.nextTestEntryID

	tc.nextTestEntryID = tc.nextTestEntryID + 1

	return result
}

func newTestCollection(kubeObj apiv1alpha1.KoupletTest) *TestCollection {

	dateSubmitted := time.Now().UnixNano() / 1000000

	return &TestCollection{
		kubeObj:         kubeObj,
		testEntries:     make(map[int]*TestEntry),
		nextTestEntryID: 0,
		uuid:            string(kubeObj.UID), // Use the kube object UID
		dateSubmitted:   &dateSubmitted,
		status:          testCollectionStatusWaiting,
		bucket:          testCollectionBucketWaiting,
	}
}

func (tc *TestCollection) markAsPrimaryTestCollectionAndPersist(kdb *KoupletDB) {
	startTime := time.Now().UnixNano() / 1000000
	tc.startTime = &startTime
	kdb.persistenceContext.updateTestCollectionShallow(tc)
}

func (tc *TestCollection) markAsCompleteAndPersist(kdb *KoupletDB) {
	tc.bucket = testCollectionBucketCompleted
	tc.status = testCollectionStatusComplete

	endTime := time.Now().UnixNano() / 1000000
	tc.endTime = &endTime

	kdb.persistenceContext.updateTestCollectionShallow(tc)
}

func (tc *TestCollection) markAsRunningAndPersist(kdb *KoupletDB) {
	tc.status = testCollectionStatusRunning
	tc.bucket = testCollectionBucketRunning
	kdb.persistenceContext.updateTestCollectionShallow(tc)
}

// GetTestEntry ...
func (tc *TestCollection) GetTestEntry(id int) *TestEntry {

	return tc.testEntries[id]

}

func removeByUUID(uuid string, input []*TestCollection) []*TestCollection {

	result := []*TestCollection{}

	for _, testCollection := range input {

		if testCollection.uuid != uuid {
			result = append(result, testCollection)
		}

	}

	return result
}

// TestEntry ...
type TestEntry struct {
	id              int
	retryFromID     *int
	status          testEntryStatus
	result          *apiv1alpha1.KoupletTestJobResultEntry
	testEntry       apiv1alpha1.KoupletTestEntry
	clusterResource *apiv1alpha1.KoupletTestJobResultClusterResource
	processed       bool
	bucket          testEntryBucket
}

func (te *TestEntry) toShortString() string {

	result := fmt.Sprintf("id: %d", te.id)

	if te.retryFromID != nil {
		result += fmt.Sprintf(" retry-from: %d", *(te.retryFromID))
	}

	result += " [" + testEntryStatusToString(te.status) + "]"

	return result
}

func (te *TestEntry) toString() string {

	result := te.toShortString()

	if len(te.testEntry.Values) > 0 {
		result += "{"
		for _, val := range te.testEntry.Values {
			result += val.Name + " -> " + val.Value + ",  "
		}
		result += "}"

	}
	return result

}

func (te *TestEntry) markAsExpiredAndPersist(tc *TestCollection, kdb *KoupletDB) {
	te.processed = true
	te.status = testEntryStatusExpired
	kdb.persistenceContext.updateTestEntryShallow(te, tc)
}

func (te *TestEntry) markAsRunningAndPersist(tc *TestCollection, kdb *KoupletDB) {
	te.status = testEntryStatusRunning
	te.bucket = testEntryBucketRunning
	kdb.persistenceContext.updateTestEntryShallow(te, tc)
}

func (te *TestEntry) markAsCompleteAndPersist(tc *TestCollection, kdb *KoupletDB) {
	te.processed = true
	te.status = testEntryStatusComplete
	kdb.persistenceContext.updateTestEntryShallow(te, tc)
}

// KTContext ...
type KTContext struct {
	Client    *client.Client
	Namespace string
	Scheme    *runtime.Scheme
}

type testEntryStatus uint8

const (
	testEntryStatusWaiting = testEntryStatus(iota)
	testEntryStatusRunning
	testEntryStatusComplete
	testEntryStatusExpired
)

type testCollectionStatus uint8

const (
	testCollectionStatusWaiting = testCollectionStatus(iota)
	testCollectionStatusRunning
	testCollectionStatusComplete
)

type testCollectionBucket uint8

const (
	testCollectionBucketWaiting = testCollectionBucket(iota)
	testCollectionBucketRunning
	testCollectionBucketCompleted
)

type testEntryBucket uint8

const (
	testEntryBucketQueued = testEntryBucket(iota)
	testEntryBucketRunning
	testEntryBucketCompleted
	testEntryBucketRetry
)

func testEntryBucketToString(bucket testEntryBucket) string {

	switch bucket {
	case testEntryBucketQueued:
		return "Queued"
	case testEntryBucketRunning:
		return "Running"
	case testEntryBucketCompleted:
		return "Completed"
	case testEntryBucketRetry:
		return "Retry"
	default:
		shared.LogSevere(errors.New("Invalid testEntryBucket"))
		return ""
	}

}

func stringToTestEntryBucket(bucketStr string) (testEntryBucket, error) {

	switch bucketStr {
	case "Queued":
		return testEntryBucketQueued, nil
	case "Running":
		return testEntryBucketRunning, nil
	case "Completed":
		return testEntryBucketCompleted, nil
	case "Retry":
		return testEntryBucketRetry, nil
	default:
		err := errors.New("Invalid test entry bucket string: " + bucketStr)
		shared.LogSevere(err)
		return testEntryBucketQueued, err
	}
}

func stringToTestCollectionBucket(bucketStr string) (testCollectionBucket, error) {
	switch bucketStr {
	case "Waiting":
		return testCollectionBucketWaiting, nil
	case "Running":
		return testCollectionBucketRunning, nil
	case "Completed":
		return testCollectionBucketCompleted, nil
	default:
		err := errors.New("Invalid collection bucket string: " + bucketStr)
		shared.LogSevere(err)
		return testCollectionBucketWaiting, err
	}
}

func testCollectionBucketToString(val testCollectionBucket) string {

	switch val {
	case testCollectionBucketWaiting:
		return "Waiting"
	case testCollectionBucketRunning:
		return "Running"
	case testCollectionBucketCompleted:
		return "Completed"
	default:
		shared.LogSevere(errors.New("Invalid testCollectionBucket"))
		return ""
	}
}

func testCollectionStatusToString(val testCollectionStatus) string {
	if val == testCollectionStatusWaiting {
		return "Waiting"
	} else if val == testCollectionStatusRunning {
		return "Running"
	} else if val == testCollectionStatusComplete {
		return "Complete"
	} else {
		shared.LogSevere(errors.New("Invalid testCollectionStatus"))
		return ""
	}

}

func stringToTestCollectionStatus(val string) (testCollectionStatus, error) {
	switch val {
	case "Waiting":
		return testCollectionStatusWaiting, nil
	case "Running":
		return testCollectionStatusRunning, nil
	case "Complete":
		return testCollectionStatusComplete, nil
	default:
		return testCollectionStatusWaiting, errors.New("Invalid testCollectionStatus string: " + val)
	}
}

func stringToTestEntryStatus(val string) (testEntryStatus, error) {
	switch val {
	case "Waiting":
		return testEntryStatusWaiting, nil
	case "Running":
		return testEntryStatusRunning, nil
	case "Complete":
		return testEntryStatusComplete, nil
	case "Expired":
		return testEntryStatusExpired, nil
	default:
		err := errors.New("Invalid testEntryStatus string: " + val)
		shared.LogSevere(err)
		return testEntryStatusWaiting, err
	}
}

func testEntryStatusToString(val testEntryStatus) string {
	if val == testEntryStatusWaiting {
		return "Waiting"
	} else if val == testEntryStatusRunning {
		return "Running"
	} else if val == testEntryStatusComplete {
		return "Complete"
	} else if val == testEntryStatusExpired {
		return "Expired"
	} else {
		shared.LogSevere(errors.New("Invalid testEntryStatus"))
		return ""
	}

}

// DefaultTestsToRun ...
const DefaultTestsToRun = 3

// DefaultExpireTimeInSeconds ...
const DefaultExpireTimeInSeconds = 3600

// DefaultTestRetries ...
const DefaultTestRetries = 2

// DefaultTimeBetweenTestRunsInSeconds ...
const DefaultTimeBetweenTestRunsInSeconds = 120

// DefaultNumberOfNodes ...
const DefaultNumberOfNodes = 1

// LabelCollectionUUID ...
const LabelCollectionUUID = "collection-uuid"

// LabelTestEntryID ...
const LabelTestEntryID = "test-entry-id"
