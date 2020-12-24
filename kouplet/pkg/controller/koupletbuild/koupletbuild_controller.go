package koupletbuild

import (
	"context"
	"fmt"
	"os"
	"time"

	apiv1alpha1 "github.com/jgwest/kouplet/pkg/apis/api/v1alpha1"
	"github.com/jgwest/kouplet/pkg/controller/shared"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	DefaultBuilderImageName = "jgwest/k-builder-util"
	DefaultBuilderImageTag  = "latest"
)

// Add creates a new KoupletBuild Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileKoupletBuild{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("koupletbuild-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource KoupletBuild
	err = c.Watch(&source.Kind{Type: &apiv1alpha1.KoupletBuild{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Pods and requeue the owner KoupletBuild
	err = c.Watch(&source.Kind{Type: &batchv1.Job{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &apiv1alpha1.KoupletBuild{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileKoupletBuild implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileKoupletBuild{}

// ReconcileKoupletBuild reconciles a KoupletBuild object
type ReconcileKoupletBuild struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

var nextReconcileTime = time.Now().Add(time.Second * -90)

func appendReconcileResult(input reconcile.Result, err error) (reconcile.Result, error) {

	if err != nil || input.Requeue {
		return input, err
	}

	// If we have not scheduled a reconcile in the last 5 seconds, then schedule one 10 seconds from now
	if nextReconcileTime.Before(time.Now()) {
		nextReconcileTime = time.Now().Add(time.Second * 45)
		input.RequeueAfter = time.Second * 90
	}

	return input, err
}

var controllerBuildDB = (*koupletBuildDB)(nil)

// Reconcile reads that state of the cluster for a KoupletBuild object and makes changes based on the state read
// and what is in the KoupletBuild.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileKoupletBuild) Reconcile(request reconcile.Request) (reconcile.Result, error) {

	shared.LogDebug("Reconciling KoupletBuild")

	kbContext := KBContext{
		client:    &r.client,
		scheme:    r.scheme,
		namespace: request.Namespace,
	}

	if controllerBuildDB == nil {

		defaultOperatorPath := "/kouplet-persistence/build"

		// Run within /tmp if running outside container/image
		if _, err := os.Stat(defaultOperatorPath); os.IsNotExist(err) {
			defaultOperatorPath = "/tmp" + defaultOperatorPath
		}

		controllerBuildDB = newKoupletBuildDB(newPersistenceContext(defaultOperatorPath), kbContext)
	}

	// Fetch the KoupletBuild instance
	instance := &apiv1alpha1.KoupletBuild{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return appendReconcileResult(reconcile.Result{}, nil)
		}
		// Error reading the object - requeue the request.
		return appendReconcileResult(reconcile.Result{}, err)
	}

	controllerBuildDB.process(*instance, kbContext)

	return appendReconcileResult(reconcile.Result{}, nil)
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

// KBContext ...
type KBContext struct {
	client    *client.Client
	namespace string
	scheme    *runtime.Scheme
}
