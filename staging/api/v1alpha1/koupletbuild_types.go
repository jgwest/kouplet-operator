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

// KoupletBuildAttempt ...
type KoupletBuildAttempt struct {
	JobName           string `json:"jobName"`
	JobUID            string `json:"jobUID"`
	JobQueueTime      int64  `json:"jobQueueTime"`
	JobStartTime      *int64 `json:"jobStartTime,omitempty"`
	JobCompletionTime *int64 `json:"jobCompletionTime,omitempty"`
	JobSucceeded      *bool  `json:"jobSucceeded,omitempty"`
}

// KoupletBuildStatus defines the observed state of KoupletBuild
type KoupletBuildStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Status                 string                `json:"status"`
	ActiveBuildAttempts    []KoupletBuildAttempt `json:"activeBuildAttempts,omitempty"`
	CompletedBuildAttempts []KoupletBuildAttempt `json:"completedBuildAttempts,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// KoupletBuild is the Schema for the koupletbuilds API
type KoupletBuild struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KoupletBuildSpec   `json:"spec,omitempty"`
	Status KoupletBuildStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// KoupletBuildList contains a list of KoupletBuild
type KoupletBuildList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KoupletBuild `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KoupletBuild{}, &KoupletBuildList{})
}
