package kouplettest

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	apiv1alpha1 "github.com/jgwest/kouplet/pkg/apis/api/v1alpha1"
	"github.com/jgwest/kouplet/pkg/controller/shared"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	kubernetes "k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Add creates a new KoupletTest Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileKoupletTest{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("kouplettest-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource KoupletTest
	err = c.Watch(&source.Kind{Type: &apiv1alpha1.KoupletTest{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource and requeue the owner KoupletTest
	err = c.Watch(&source.Kind{Type: &batchv1.Job{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &apiv1alpha1.KoupletTest{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileKoupletTest implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileKoupletTest{}

var koupletControllerDB = (*KoupletDB)(nil)

// ReconcileKoupletTest reconciles a KoupletTest object
type ReconcileKoupletTest struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

var nextReconcileTime = time.Now().Add(time.Second * time.Duration(-60))

func appendReconcileResult(input reconcile.Result, err error) (reconcile.Result, error) {

	if err != nil || input.Requeue {
		shared.LogInfo("Requeue already set")
		return input, err
	}

	// If we have not scheduled a reconcile in the last 5 seconds, then schedule one 10 seconds from now
	if nextReconcileTime.Before(time.Now()) {
		// log.Info("Scheduling reconcile")
		nextReconcileTime = time.Now().Add(time.Second * time.Duration(10))
		input.RequeueAfter = time.Second * time.Duration(20)

		shared.LogInfo(fmt.Sprintf("Added requeueAfter %v", nextReconcileTime))
	} else {
		shared.LogInfo(fmt.Sprintf("Did not add requeueAfter %v", nextReconcileTime))
	}

	return input, nil
}

// Reconcile reads that state of the cluster for a KoupletTest object and makes changes based on the state read
// and what is in the KoupletTest.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileKoupletTest) Reconcile(request reconcile.Request) (reconcile.Result, error) {

	shared.LogInfo("Reconcile on kouplettest called")

	ktContext := KTContext{
		&r.client,
		request.NamespacedName.Namespace,
		r.scheme,
	}

	if koupletControllerDB == nil {
		defaultOperatorPath := "/kouplet-persistence/test"

		// Run within /tmp if running outside container/image
		if _, err := os.Stat(defaultOperatorPath); os.IsNotExist(err) {
			defaultOperatorPath = "/tmp" + defaultOperatorPath
		}

		koupletControllerDB = newKoupletDB(newPersistenceContext(defaultOperatorPath), ktContext)
	}

	// Fetch the KoupletTest instance
	instance := &apiv1alpha1.KoupletTest{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err == nil {
		// Ensure the object exists in the DB
		koupletControllerDB.GetOrCreateCollection(*instance)
	} else {
		if kubeerrors.IsNotFound(err) {
			shared.LogDebug("Unable to locate KoupletTest '" + request.NamespacedName.Name + "': it could have been deleted during or after reconcile request.")
			// koupletDB.DeleteCollection(request.NamespacedName)

			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			// return appendReconcileResult(reconcile.Result{}, nil)
		} else {
			// Error reading the object - requeue the request.
			return appendReconcileResult(reconcile.Result{}, err)
		}
	}

	// Process all entries in the database (not specifically related to this reconcile request)
	err = koupletControllerDB.process(ktContext)

	return appendReconcileResult(reconcile.Result{}, err)
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
