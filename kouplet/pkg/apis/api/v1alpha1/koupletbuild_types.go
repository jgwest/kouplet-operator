package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KoupletBuildSpec defines the desired state of KoupletBuild
type KoupletBuildSpec struct {
	Urls    []string       `json:"urls"`
	GitRepo KoupletGitRepo `json:"gitRepo"`
	// Image is the target name of the container image, that will built, tagged, and pushed to the remote registry
	Image             string                   `json:"image"`
	ContainerRegistry KoupletContainerRegistry `json:"containerRegistry"`
	BuildContraints   *KoupletBuildConstraints `json:"buildContraints,omitempty"`
	// BuilderImage is the container image of the build Job
	BuilderImage string `json:"builderImage"`
}

// KoupletBuildConstraints ...
type KoupletBuildConstraints struct {
	MaxOverallBuildTimeInSeconds *int `json:"maxOverallBuildTimeInSeconds,omitempty"`
	MaxNumberofBuildAttempts     *int `json:"maxNumberofBuildAttempts,omitempty"`
	MaxJobBuildTimeInSeconds     *int `json:"maxJobBuildTimeInSeconds,omitempty"`
	MaxJobWaitStartTimeInSeconds *int `json:"maxJobWaitStartTimeInSeconds,omitempty"`
}

// KoupletGitRepo ...
type KoupletGitRepo struct {
	URL                   string `json:"url"`
	SubPath               string `json:"subpath,omitempty"`
	CredentialsSecretName string `json:"credentialsSecretName,omitempty"`
}

// KoupletContainerRegistry ...
type KoupletContainerRegistry struct {
	CredentialsSecretName string `json:"credentialsSecretName"`
}

// KoupletBuildStatus defines the observed state of KoupletBuild
type KoupletBuildStatus struct {
	Status                 string                `json:"status"`
	ActiveBuildAttempts    []KoupletBuildAttempt `json:"activeBuildAttempts,omitempty"`
	CompletedBuildAttempts []KoupletBuildAttempt `json:"completedBuildAttempts,omitempty"`
}

// KoupletBuildAttempt ...
type KoupletBuildAttempt struct {
	JobName           string `json:"jobName"`
	JobUID            string `json:"jobUID"`
	JobQueueTime      int64  `json:"jobQueueTime"`
	JobStartTime      *int64 `json:"jobStartTime,omitempty"`
	JobCompletionTime *int64 `json:"jobCompletionTime,omitempty"`
	JobSucceeded      *bool  `json:"jobSucceeded,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KoupletBuild is the Schema for the koupletbuilds API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=koupletbuilds,scope=Namespaced
type KoupletBuild struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KoupletBuildSpec   `json:"spec,omitempty"`
	Status KoupletBuildStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KoupletBuildList contains a list of KoupletBuild
type KoupletBuildList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KoupletBuild `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KoupletBuild{}, &KoupletBuildList{})
}
