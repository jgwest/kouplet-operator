/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	apiv1alpha1 "github.com/jgwest/kouplet-operator/api/v1alpha1"
	"github.com/jgwest/kouplet-operator/controllers/kouplettest"
	"github.com/jgwest/kouplet-operator/controllers/shared"
	batchv1 "k8s.io/api/batch/v1"
	kubeErrors "k8s.io/apimachinery/pkg/api/errors"
)

// KoupletTestReconciler reconciles a KoupletTest object
type KoupletTestReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

var koupletControllerDB = (*kouplettest.KoupletDB)(nil)

// ReconcileKoupletTest reconciles a KoupletTest object
type ReconcileKoupletTest struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

var nextTestReconcileTime = time.Now().Add(time.Second * time.Duration(-60))

func appendTestReconcileResult(input ctrl.Result, err error) (ctrl.Result, error) {

	if err != nil || input.Requeue {
		shared.LogInfo("Requeue already set")
		return input, err
	}

	// If we have not scheduled a reconcile in the last 5 seconds, then schedule one 10 seconds from now
	if nextTestReconcileTime.Before(time.Now()) {
		// log.Info("Scheduling reconcile")
		nextTestReconcileTime = time.Now().Add(time.Second * time.Duration(10))
		input.RequeueAfter = time.Second * time.Duration(20)

		shared.LogInfo(fmt.Sprintf("Added requeueAfter %v", nextTestReconcileTime))
	} else {
		shared.LogInfo(fmt.Sprintf("Did not add requeueAfter %v", nextTestReconcileTime))
	}

	return input, nil
}

//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=pods/log,verbs=get;watch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;watch
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=api.kouplet.com,resources=kouplettests,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=api.kouplet.com,resources=kouplettests/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=api.kouplet.com,resources=kouplettests/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the KoupletTest object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.0/pkg/reconcile
func (r *KoupletTestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	shared.LogInfo("Reconcile on kouplettest called")

	ktContext := kouplettest.KTContext{
		Client:    &r.Client,
		Namespace: req.NamespacedName.Namespace,
		Scheme:    r.Scheme,
	}

	if koupletControllerDB == nil {
		defaultOperatorPath := "/kouplet-persistence/test"

		// Run within /tmp if running outside container/image
		if _, err := os.Stat(defaultOperatorPath); os.IsNotExist(err) {
			defaultOperatorPath = "/tmp" + defaultOperatorPath
		}

		koupletControllerDB = kouplettest.NewKoupletDB(kouplettest.NewPersistenceContext(defaultOperatorPath), ktContext)
	}

	// Fetch the KoupletTest instance
	instance := &apiv1alpha1.KoupletTest{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err == nil {
		// Ensure the object exists in the DB
		koupletControllerDB.GetOrCreateCollection(*instance)
	} else {
		if kubeErrors.IsNotFound(err) {
			shared.LogDebug("Unable to locate KoupletTest '" + req.NamespacedName.Name + "': it could have been deleted during or after reconcile request.")
			// koupletDB.DeleteCollection(request.NamespacedName)

			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			// return appendReconcileResult(reconcile.Result{}, nil)
		} else {
			// Error reading the object - requeue the request.
			return appendTestReconcileResult(ctrl.Result{}, err)
		}
	}

	// Process all entries in the database (not specifically related to this reconcile request)
	err = koupletControllerDB.Process(ktContext)

	return appendTestReconcileResult(ctrl.Result{}, err)
}

// SetupWithManager sets up the controller with the Manager.
func (r *KoupletTestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1alpha1.KoupletTest{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}
