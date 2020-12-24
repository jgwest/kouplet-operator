package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KoupletTestSpec defines the desired state of KoupletTest
type KoupletTestSpec struct {
	Name    string             `json:"name"`
	Image   string             `json:"image"`
	Command []string           `json:"command,omitempty"`
	Env     []KoupletTestEnv   `json:"env,omitempty"`
	Tests   []KoupletTestEntry `json:"tests"`

	NumberOfNodes              *int `json:"numberOfNodes,omitempty"`
	DefaultTestExpireTime      *int `json:"defaultTestExpireTime,omitempty"`
	Timeout                    *int `json:"timeout,omitempty"`
	FailureRetries             *int `json:"failureRetries,omitempty"`
	MaxActiveTests             *int `json:"maxActiveTests,omitempty"`             // per node
	MinimumTimeBetweenTestRuns *int `json:"minimumTimeBetweenTestRuns,omitempty"` // per node

	ObjectStorageCredentials *KoupletObjectStorage `json:"objectStorageCredentials,omitempty"`
}

// KoupletObjectStorage ...
type KoupletObjectStorage struct {
	Endpoint              string `json:"endpoint"`
	CredentialsSecretName string `json:"credentialsSecretName"`
	BucketName            string `json:"bucketName"`
	BucketLocation        string `json:"bucketLocation"`
	// AccessKeyID     string `json:"accessKeyID"`
	// SecretAccessKey string `json:"secretAccessKey"`

}

// KoupletTestEnv contains a single environment variable key/value pair
type KoupletTestEnv struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// KoupletTestEntry specifies which tests to run.
type KoupletTestEntry struct {
	Timeout *int                    `json:"timeout,omitempty"`
	Env     []KoupletTestEnv        `json:"env,omitempty"`
	Values  []KoupletTestEntryKV    `json:"values"`
	Labels  []KoupletTestEntryLabel `json:"labels,omitempty"`
}

// KoupletTestEntryKV test container specific values
type KoupletTestEntryKV struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// KoupletTestEntryLabel are generic key/value pairs for end-user use
type KoupletTestEntryLabel struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// KoupletTestStatus defines the observed state of KoupletTest
type KoupletTestStatus struct {
	Status        string `json:"status"` // waiting, running, complete
	DateSubmitted *int64 `json:"dateSubmitted,omitempty"`
	Percent       *int   `json:"percent,omitempty"`

	StartTime *int64 `json:"startTime,omitempty"`
	EndTime   *int64 `json:"endTime,omitempty"`

	Results []KoupletTestJobResultEntry `json:"results"`
}

// KoupletTestJobResultEntry ...
type KoupletTestJobResultEntry struct {
	ID          int                  `json:"id"`
	Test        []KoupletTestEntryKV `json:"test"`
	Status      string               `json:"status"`
	Percent     *int                 `json:"percent,omitempty"`
	NumTests    *int                 `json:"numTests,omitempty"`
	NumErrors   *int                 `json:"numErrors,omitempty"`
	NumFailures *int                 `json:"numFailures,omitempty"`
	TestTime    *int64               `json:"testTime,omitempty"`

	Labels []KoupletTestEntryLabel `json:"labels,omitempty"`

	Result    *string `json:"result,omitempty"` // S3 URL with full JUnit text
	Log       *string `json:"log,omitempty"`    // S3 URL with full test log
	RetryFrom *int    `json:"retryFrom,omitempty"`

	ClusterResource *KoupletTestJobResultClusterResource `json:"clusterResource,omitempty"`
}

// KoupletTestJobResultClusterResource ...
type KoupletTestJobResultClusterResource struct {
	Namespace string `json:"namespace"`
	JobName   string `json:"jobName"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KoupletTest is the Schema for the kouplettests API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=kouplettests,scope=Namespaced
type KoupletTest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KoupletTestSpec   `json:"spec,omitempty"`
	Status KoupletTestStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KoupletTestList contains a list of KoupletTest
type KoupletTestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KoupletTest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KoupletTest{}, &KoupletTestList{})
}
