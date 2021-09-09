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
	"os"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	apiv1alpha1 "github.com/jgwest/kouplet-operator/api/v1alpha1"
	"github.com/jgwest/kouplet-operator/controllers/koupletbuild"
	"github.com/jgwest/kouplet-operator/controllers/shared"
)

// KoupletBuildReconciler reconciles a KoupletBuild object
type KoupletBuildReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

var nextBuildReconcileTime = time.Now().Add(time.Second * -90)

func appendBuildReconcileResult(input reconcile.Result, err error) (reconcile.Result, error) {

	if err != nil || input.Requeue {
		return input, err
	}

	// If we have not scheduled a reconcile in the last 5 seconds, then schedule one 10 seconds from now
	if nextBuildReconcileTime.Before(time.Now()) {
		nextBuildReconcileTime = time.Now().Add(time.Second * 45)
		input.RequeueAfter = time.Second * 90
	}

	return input, err
}

var controllerBuildDB = (*koupletbuild.KoupletBuildDB)(nil)

//+kubebuilder:rbac:groups=api.kouplet.com,resources=koupletbuilds,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=api.kouplet.com,resources=koupletbuilds/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=api.kouplet.com,resources=koupletbuilds/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the KoupletBuild object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.0/pkg/reconcile
func (r *KoupletBuildReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	// 	_ = r.Log.WithValues("koupletbuild", req.NamespacedName)

	shared.LogDebug("Reconciling KoupletBuild")

	kbContext := koupletbuild.KBContext{
		Client:    &r.Client,
		Scheme:    r.Scheme,
		Namespace: req.Namespace,
	}

	if controllerBuildDB == nil {

		defaultOperatorPath := "/kouplet-persistence/build"

		// Run within /tmp if running outside container/image
		if _, err := os.Stat(defaultOperatorPath); os.IsNotExist(err) {
			defaultOperatorPath = "/tmp" + defaultOperatorPath
		}

		controllerBuildDB = koupletbuild.NewKoupletBuildDB(koupletbuild.NewPersistenceContext(defaultOperatorPath), kbContext)
	}

	// Fetch the KoupletBuild instance
	instance := &apiv1alpha1.KoupletBuild{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return appendBuildReconcileResult(reconcile.Result{}, nil)
		}
		// Error reading the object - requeue the request.
		return appendBuildReconcileResult(reconcile.Result{}, err)
	}

	controllerBuildDB.Process(*instance, kbContext)

	return appendBuildReconcileResult(reconcile.Result{}, nil)
}

// SetupWithManager sets up the controller with the Manager.
func (r *KoupletBuildReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1alpha1.KoupletBuild{}).
		Complete(r)
}
