// +build !ignore_autogenerated

// Code generated by operator-sdk. DO NOT EDIT.

package v1alpha1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KoupletBuild) DeepCopyInto(out *KoupletBuild) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KoupletBuild.
func (in *KoupletBuild) DeepCopy() *KoupletBuild {
	if in == nil {
		return nil
	}
	out := new(KoupletBuild)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *KoupletBuild) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KoupletBuildAttempt) DeepCopyInto(out *KoupletBuildAttempt) {
	*out = *in
	if in.JobStartTime != nil {
		in, out := &in.JobStartTime, &out.JobStartTime
		*out = new(int64)
		**out = **in
	}
	if in.JobCompletionTime != nil {
		in, out := &in.JobCompletionTime, &out.JobCompletionTime
		*out = new(int64)
		**out = **in
	}
	if in.JobSucceeded != nil {
		in, out := &in.JobSucceeded, &out.JobSucceeded
		*out = new(bool)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KoupletBuildAttempt.
func (in *KoupletBuildAttempt) DeepCopy() *KoupletBuildAttempt {
	if in == nil {
		return nil
	}
	out := new(KoupletBuildAttempt)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KoupletBuildConstraints) DeepCopyInto(out *KoupletBuildConstraints) {
	*out = *in
	if in.MaxOverallBuildTimeInSeconds != nil {
		in, out := &in.MaxOverallBuildTimeInSeconds, &out.MaxOverallBuildTimeInSeconds
		*out = new(int)
		**out = **in
	}
	if in.MaxNumberofBuildAttempts != nil {
		in, out := &in.MaxNumberofBuildAttempts, &out.MaxNumberofBuildAttempts
		*out = new(int)
		**out = **in
	}
	if in.MaxJobBuildTimeInSeconds != nil {
		in, out := &in.MaxJobBuildTimeInSeconds, &out.MaxJobBuildTimeInSeconds
		*out = new(int)
		**out = **in
	}
	if in.MaxJobWaitStartTimeInSeconds != nil {
		in, out := &in.MaxJobWaitStartTimeInSeconds, &out.MaxJobWaitStartTimeInSeconds
		*out = new(int)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KoupletBuildConstraints.
func (in *KoupletBuildConstraints) DeepCopy() *KoupletBuildConstraints {
	if in == nil {
		return nil
	}
	out := new(KoupletBuildConstraints)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KoupletBuildList) DeepCopyInto(out *KoupletBuildList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]KoupletBuild, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KoupletBuildList.
func (in *KoupletBuildList) DeepCopy() *KoupletBuildList {
	if in == nil {
		return nil
	}
	out := new(KoupletBuildList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *KoupletBuildList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KoupletBuildSpec) DeepCopyInto(out *KoupletBuildSpec) {
	*out = *in
	if in.Urls != nil {
		in, out := &in.Urls, &out.Urls
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	out.GitRepo = in.GitRepo
	out.ContainerRegistry = in.ContainerRegistry
	if in.BuildContraints != nil {
		in, out := &in.BuildContraints, &out.BuildContraints
		*out = new(KoupletBuildConstraints)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KoupletBuildSpec.
func (in *KoupletBuildSpec) DeepCopy() *KoupletBuildSpec {
	if in == nil {
		return nil
	}
	out := new(KoupletBuildSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KoupletBuildStatus) DeepCopyInto(out *KoupletBuildStatus) {
	*out = *in
	if in.ActiveBuildAttempts != nil {
		in, out := &in.ActiveBuildAttempts, &out.ActiveBuildAttempts
		*out = make([]KoupletBuildAttempt, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.CompletedBuildAttempts != nil {
		in, out := &in.CompletedBuildAttempts, &out.CompletedBuildAttempts
		*out = make([]KoupletBuildAttempt, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KoupletBuildStatus.
func (in *KoupletBuildStatus) DeepCopy() *KoupletBuildStatus {
	if in == nil {
		return nil
	}
	out := new(KoupletBuildStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KoupletContainerRegistry) DeepCopyInto(out *KoupletContainerRegistry) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KoupletContainerRegistry.
func (in *KoupletContainerRegistry) DeepCopy() *KoupletContainerRegistry {
	if in == nil {
		return nil
	}
	out := new(KoupletContainerRegistry)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KoupletGitRepo) DeepCopyInto(out *KoupletGitRepo) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KoupletGitRepo.
func (in *KoupletGitRepo) DeepCopy() *KoupletGitRepo {
	if in == nil {
		return nil
	}
	out := new(KoupletGitRepo)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KoupletObjectStorage) DeepCopyInto(out *KoupletObjectStorage) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KoupletObjectStorage.
func (in *KoupletObjectStorage) DeepCopy() *KoupletObjectStorage {
	if in == nil {
		return nil
	}
	out := new(KoupletObjectStorage)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KoupletTest) DeepCopyInto(out *KoupletTest) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KoupletTest.
func (in *KoupletTest) DeepCopy() *KoupletTest {
	if in == nil {
		return nil
	}
	out := new(KoupletTest)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *KoupletTest) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KoupletTestEntry) DeepCopyInto(out *KoupletTestEntry) {
	*out = *in
	if in.Timeout != nil {
		in, out := &in.Timeout, &out.Timeout
		*out = new(int)
		**out = **in
	}
	if in.Env != nil {
		in, out := &in.Env, &out.Env
		*out = make([]KoupletTestEnv, len(*in))
		copy(*out, *in)
	}
	if in.Values != nil {
		in, out := &in.Values, &out.Values
		*out = make([]KoupletTestEntryKV, len(*in))
		copy(*out, *in)
	}
	if in.Labels != nil {
		in, out := &in.Labels, &out.Labels
		*out = make([]KoupletTestEntryLabel, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KoupletTestEntry.
func (in *KoupletTestEntry) DeepCopy() *KoupletTestEntry {
	if in == nil {
		return nil
	}
	out := new(KoupletTestEntry)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KoupletTestEntryKV) DeepCopyInto(out *KoupletTestEntryKV) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KoupletTestEntryKV.
func (in *KoupletTestEntryKV) DeepCopy() *KoupletTestEntryKV {
	if in == nil {
		return nil
	}
	out := new(KoupletTestEntryKV)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KoupletTestEntryLabel) DeepCopyInto(out *KoupletTestEntryLabel) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KoupletTestEntryLabel.
func (in *KoupletTestEntryLabel) DeepCopy() *KoupletTestEntryLabel {
	if in == nil {
		return nil
	}
	out := new(KoupletTestEntryLabel)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KoupletTestEnv) DeepCopyInto(out *KoupletTestEnv) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KoupletTestEnv.
func (in *KoupletTestEnv) DeepCopy() *KoupletTestEnv {
	if in == nil {
		return nil
	}
	out := new(KoupletTestEnv)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KoupletTestJobResultClusterResource) DeepCopyInto(out *KoupletTestJobResultClusterResource) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KoupletTestJobResultClusterResource.
func (in *KoupletTestJobResultClusterResource) DeepCopy() *KoupletTestJobResultClusterResource {
	if in == nil {
		return nil
	}
	out := new(KoupletTestJobResultClusterResource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KoupletTestJobResultEntry) DeepCopyInto(out *KoupletTestJobResultEntry) {
	*out = *in
	if in.Test != nil {
		in, out := &in.Test, &out.Test
		*out = make([]KoupletTestEntryKV, len(*in))
		copy(*out, *in)
	}
	if in.Percent != nil {
		in, out := &in.Percent, &out.Percent
		*out = new(int)
		**out = **in
	}
	if in.NumTests != nil {
		in, out := &in.NumTests, &out.NumTests
		*out = new(int)
		**out = **in
	}
	if in.NumErrors != nil {
		in, out := &in.NumErrors, &out.NumErrors
		*out = new(int)
		**out = **in
	}
	if in.NumFailures != nil {
		in, out := &in.NumFailures, &out.NumFailures
		*out = new(int)
		**out = **in
	}
	if in.TestTime != nil {
		in, out := &in.TestTime, &out.TestTime
		*out = new(int64)
		**out = **in
	}
	if in.Labels != nil {
		in, out := &in.Labels, &out.Labels
		*out = make([]KoupletTestEntryLabel, len(*in))
		copy(*out, *in)
	}
	if in.Result != nil {
		in, out := &in.Result, &out.Result
		*out = new(string)
		**out = **in
	}
	if in.Log != nil {
		in, out := &in.Log, &out.Log
		*out = new(string)
		**out = **in
	}
	if in.RetryFrom != nil {
		in, out := &in.RetryFrom, &out.RetryFrom
		*out = new(int)
		**out = **in
	}
	if in.ClusterResource != nil {
		in, out := &in.ClusterResource, &out.ClusterResource
		*out = new(KoupletTestJobResultClusterResource)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KoupletTestJobResultEntry.
func (in *KoupletTestJobResultEntry) DeepCopy() *KoupletTestJobResultEntry {
	if in == nil {
		return nil
	}
	out := new(KoupletTestJobResultEntry)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KoupletTestList) DeepCopyInto(out *KoupletTestList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]KoupletTest, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KoupletTestList.
func (in *KoupletTestList) DeepCopy() *KoupletTestList {
	if in == nil {
		return nil
	}
	out := new(KoupletTestList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *KoupletTestList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KoupletTestSpec) DeepCopyInto(out *KoupletTestSpec) {
	*out = *in
	if in.Command != nil {
		in, out := &in.Command, &out.Command
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Env != nil {
		in, out := &in.Env, &out.Env
		*out = make([]KoupletTestEnv, len(*in))
		copy(*out, *in)
	}
	if in.Tests != nil {
		in, out := &in.Tests, &out.Tests
		*out = make([]KoupletTestEntry, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.NumberOfNodes != nil {
		in, out := &in.NumberOfNodes, &out.NumberOfNodes
		*out = new(int)
		**out = **in
	}
	if in.DefaultTestExpireTime != nil {
		in, out := &in.DefaultTestExpireTime, &out.DefaultTestExpireTime
		*out = new(int)
		**out = **in
	}
	if in.Timeout != nil {
		in, out := &in.Timeout, &out.Timeout
		*out = new(int)
		**out = **in
	}
	if in.FailureRetries != nil {
		in, out := &in.FailureRetries, &out.FailureRetries
		*out = new(int)
		**out = **in
	}
	if in.MaxActiveTests != nil {
		in, out := &in.MaxActiveTests, &out.MaxActiveTests
		*out = new(int)
		**out = **in
	}
	if in.MinimumTimeBetweenTestRuns != nil {
		in, out := &in.MinimumTimeBetweenTestRuns, &out.MinimumTimeBetweenTestRuns
		*out = new(int)
		**out = **in
	}
	if in.ObjectStorageCredentials != nil {
		in, out := &in.ObjectStorageCredentials, &out.ObjectStorageCredentials
		*out = new(KoupletObjectStorage)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KoupletTestSpec.
func (in *KoupletTestSpec) DeepCopy() *KoupletTestSpec {
	if in == nil {
		return nil
	}
	out := new(KoupletTestSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KoupletTestStatus) DeepCopyInto(out *KoupletTestStatus) {
	*out = *in
	if in.DateSubmitted != nil {
		in, out := &in.DateSubmitted, &out.DateSubmitted
		*out = new(int64)
		**out = **in
	}
	if in.Percent != nil {
		in, out := &in.Percent, &out.Percent
		*out = new(int)
		**out = **in
	}
	if in.StartTime != nil {
		in, out := &in.StartTime, &out.StartTime
		*out = new(int64)
		**out = **in
	}
	if in.EndTime != nil {
		in, out := &in.EndTime, &out.EndTime
		*out = new(int64)
		**out = **in
	}
	if in.Results != nil {
		in, out := &in.Results, &out.Results
		*out = make([]KoupletTestJobResultEntry, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KoupletTestStatus.
func (in *KoupletTestStatus) DeepCopy() *KoupletTestStatus {
	if in == nil {
		return nil
	}
	out := new(KoupletTestStatus)
	in.DeepCopyInto(out)
	return out
}
