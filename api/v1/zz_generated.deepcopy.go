//go:build !ignore_autogenerated

// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

// Code generated by controller-gen. DO NOT EDIT.

package v1

import (
	"github.com/fluxcd/pkg/apis/kustomize"
	"github.com/fluxcd/pkg/apis/meta"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Cluster) DeepCopyInto(out *Cluster) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Cluster.
func (in *Cluster) DeepCopy() *Cluster {
	if in == nil {
		return nil
	}
	out := new(Cluster)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CommonMetadata) DeepCopyInto(out *CommonMetadata) {
	*out = *in
	if in.Annotations != nil {
		in, out := &in.Annotations, &out.Annotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Labels != nil {
		in, out := &in.Labels, &out.Labels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CommonMetadata.
func (in *CommonMetadata) DeepCopy() *CommonMetadata {
	if in == nil {
		return nil
	}
	out := new(CommonMetadata)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ComponentImage) DeepCopyInto(out *ComponentImage) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ComponentImage.
func (in *ComponentImage) DeepCopy() *ComponentImage {
	if in == nil {
		return nil
	}
	out := new(ComponentImage)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Dependency) DeepCopyInto(out *Dependency) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Dependency.
func (in *Dependency) DeepCopy() *Dependency {
	if in == nil {
		return nil
	}
	out := new(Dependency)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Distribution) DeepCopyInto(out *Distribution) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Distribution.
func (in *Distribution) DeepCopy() *Distribution {
	if in == nil {
		return nil
	}
	out := new(Distribution)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FluxComponentStatus) DeepCopyInto(out *FluxComponentStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FluxComponentStatus.
func (in *FluxComponentStatus) DeepCopy() *FluxComponentStatus {
	if in == nil {
		return nil
	}
	out := new(FluxComponentStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FluxDistributionStatus) DeepCopyInto(out *FluxDistributionStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FluxDistributionStatus.
func (in *FluxDistributionStatus) DeepCopy() *FluxDistributionStatus {
	if in == nil {
		return nil
	}
	out := new(FluxDistributionStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FluxInstance) DeepCopyInto(out *FluxInstance) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FluxInstance.
func (in *FluxInstance) DeepCopy() *FluxInstance {
	if in == nil {
		return nil
	}
	out := new(FluxInstance)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *FluxInstance) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FluxInstanceList) DeepCopyInto(out *FluxInstanceList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]FluxInstance, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FluxInstanceList.
func (in *FluxInstanceList) DeepCopy() *FluxInstanceList {
	if in == nil {
		return nil
	}
	out := new(FluxInstanceList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *FluxInstanceList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FluxInstanceSpec) DeepCopyInto(out *FluxInstanceSpec) {
	*out = *in
	out.Distribution = in.Distribution
	if in.Components != nil {
		in, out := &in.Components, &out.Components
		*out = make([]Component, len(*in))
		copy(*out, *in)
	}
	if in.CommonMetadata != nil {
		in, out := &in.CommonMetadata, &out.CommonMetadata
		*out = new(CommonMetadata)
		(*in).DeepCopyInto(*out)
	}
	if in.Cluster != nil {
		in, out := &in.Cluster, &out.Cluster
		*out = new(Cluster)
		**out = **in
	}
	if in.Sharding != nil {
		in, out := &in.Sharding, &out.Sharding
		*out = new(Sharding)
		(*in).DeepCopyInto(*out)
	}
	if in.Storage != nil {
		in, out := &in.Storage, &out.Storage
		*out = new(Storage)
		**out = **in
	}
	if in.Kustomize != nil {
		in, out := &in.Kustomize, &out.Kustomize
		*out = new(Kustomize)
		(*in).DeepCopyInto(*out)
	}
	if in.Wait != nil {
		in, out := &in.Wait, &out.Wait
		*out = new(bool)
		**out = **in
	}
	if in.MigrateResources != nil {
		in, out := &in.MigrateResources, &out.MigrateResources
		*out = new(bool)
		**out = **in
	}
	if in.Sync != nil {
		in, out := &in.Sync, &out.Sync
		*out = new(Sync)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FluxInstanceSpec.
func (in *FluxInstanceSpec) DeepCopy() *FluxInstanceSpec {
	if in == nil {
		return nil
	}
	out := new(FluxInstanceSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FluxInstanceStatus) DeepCopyInto(out *FluxInstanceStatus) {
	*out = *in
	out.ReconcileRequestStatus = in.ReconcileRequestStatus
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Components != nil {
		in, out := &in.Components, &out.Components
		*out = make([]ComponentImage, len(*in))
		copy(*out, *in)
	}
	if in.Inventory != nil {
		in, out := &in.Inventory, &out.Inventory
		*out = new(ResourceInventory)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FluxInstanceStatus.
func (in *FluxInstanceStatus) DeepCopy() *FluxInstanceStatus {
	if in == nil {
		return nil
	}
	out := new(FluxInstanceStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FluxReconcilerStats) DeepCopyInto(out *FluxReconcilerStats) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FluxReconcilerStats.
func (in *FluxReconcilerStats) DeepCopy() *FluxReconcilerStats {
	if in == nil {
		return nil
	}
	out := new(FluxReconcilerStats)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FluxReconcilerStatus) DeepCopyInto(out *FluxReconcilerStatus) {
	*out = *in
	out.Stats = in.Stats
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FluxReconcilerStatus.
func (in *FluxReconcilerStatus) DeepCopy() *FluxReconcilerStatus {
	if in == nil {
		return nil
	}
	out := new(FluxReconcilerStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FluxReport) DeepCopyInto(out *FluxReport) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FluxReport.
func (in *FluxReport) DeepCopy() *FluxReport {
	if in == nil {
		return nil
	}
	out := new(FluxReport)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *FluxReport) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FluxReportList) DeepCopyInto(out *FluxReportList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]FluxReport, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FluxReportList.
func (in *FluxReportList) DeepCopy() *FluxReportList {
	if in == nil {
		return nil
	}
	out := new(FluxReportList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *FluxReportList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FluxReportSpec) DeepCopyInto(out *FluxReportSpec) {
	*out = *in
	out.Distribution = in.Distribution
	if in.ComponentsStatus != nil {
		in, out := &in.ComponentsStatus, &out.ComponentsStatus
		*out = make([]FluxComponentStatus, len(*in))
		copy(*out, *in)
	}
	if in.ReconcilersStatus != nil {
		in, out := &in.ReconcilersStatus, &out.ReconcilersStatus
		*out = make([]FluxReconcilerStatus, len(*in))
		copy(*out, *in)
	}
	if in.SyncStatus != nil {
		in, out := &in.SyncStatus, &out.SyncStatus
		*out = new(FluxSyncStatus)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FluxReportSpec.
func (in *FluxReportSpec) DeepCopy() *FluxReportSpec {
	if in == nil {
		return nil
	}
	out := new(FluxReportSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FluxReportStatus) DeepCopyInto(out *FluxReportStatus) {
	*out = *in
	out.ReconcileRequestStatus = in.ReconcileRequestStatus
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FluxReportStatus.
func (in *FluxReportStatus) DeepCopy() *FluxReportStatus {
	if in == nil {
		return nil
	}
	out := new(FluxReportStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FluxSyncStatus) DeepCopyInto(out *FluxSyncStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FluxSyncStatus.
func (in *FluxSyncStatus) DeepCopy() *FluxSyncStatus {
	if in == nil {
		return nil
	}
	out := new(FluxSyncStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Kustomize) DeepCopyInto(out *Kustomize) {
	*out = *in
	if in.Patches != nil {
		in, out := &in.Patches, &out.Patches
		*out = make([]kustomize.Patch, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Kustomize.
func (in *Kustomize) DeepCopy() *Kustomize {
	if in == nil {
		return nil
	}
	out := new(Kustomize)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ResourceInventory) DeepCopyInto(out *ResourceInventory) {
	*out = *in
	if in.Entries != nil {
		in, out := &in.Entries, &out.Entries
		*out = make([]ResourceRef, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResourceInventory.
func (in *ResourceInventory) DeepCopy() *ResourceInventory {
	if in == nil {
		return nil
	}
	out := new(ResourceInventory)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ResourceRef) DeepCopyInto(out *ResourceRef) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResourceRef.
func (in *ResourceRef) DeepCopy() *ResourceRef {
	if in == nil {
		return nil
	}
	out := new(ResourceRef)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ResourceSet) DeepCopyInto(out *ResourceSet) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResourceSet.
func (in *ResourceSet) DeepCopy() *ResourceSet {
	if in == nil {
		return nil
	}
	out := new(ResourceSet)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ResourceSet) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in ResourceSetInput) DeepCopyInto(out *ResourceSetInput) {
	{
		in := &in
		*out = make(ResourceSetInput, len(*in))
		for key, val := range *in {
			var outVal *apiextensionsv1.JSON
			if val == nil {
				(*out)[key] = nil
			} else {
				inVal := (*in)[key]
				in, out := &inVal, &outVal
				*out = new(apiextensionsv1.JSON)
				(*in).DeepCopyInto(*out)
			}
			(*out)[key] = outVal
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResourceSetInput.
func (in ResourceSetInput) DeepCopy() ResourceSetInput {
	if in == nil {
		return nil
	}
	out := new(ResourceSetInput)
	in.DeepCopyInto(out)
	return *out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ResourceSetInputFilter) DeepCopyInto(out *ResourceSetInputFilter) {
	*out = *in
	if in.Labels != nil {
		in, out := &in.Labels, &out.Labels
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResourceSetInputFilter.
func (in *ResourceSetInputFilter) DeepCopy() *ResourceSetInputFilter {
	if in == nil {
		return nil
	}
	out := new(ResourceSetInputFilter)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ResourceSetInputProvider) DeepCopyInto(out *ResourceSetInputProvider) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResourceSetInputProvider.
func (in *ResourceSetInputProvider) DeepCopy() *ResourceSetInputProvider {
	if in == nil {
		return nil
	}
	out := new(ResourceSetInputProvider)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ResourceSetInputProvider) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ResourceSetInputProviderList) DeepCopyInto(out *ResourceSetInputProviderList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ResourceSetInputProvider, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResourceSetInputProviderList.
func (in *ResourceSetInputProviderList) DeepCopy() *ResourceSetInputProviderList {
	if in == nil {
		return nil
	}
	out := new(ResourceSetInputProviderList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ResourceSetInputProviderList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ResourceSetInputProviderSpec) DeepCopyInto(out *ResourceSetInputProviderSpec) {
	*out = *in
	if in.SecretRef != nil {
		in, out := &in.SecretRef, &out.SecretRef
		*out = new(meta.LocalObjectReference)
		**out = **in
	}
	if in.CertSecretRef != nil {
		in, out := &in.CertSecretRef, &out.CertSecretRef
		*out = new(meta.LocalObjectReference)
		**out = **in
	}
	if in.DefaultValues != nil {
		in, out := &in.DefaultValues, &out.DefaultValues
		*out = make(ResourceSetInput, len(*in))
		for key, val := range *in {
			var outVal *apiextensionsv1.JSON
			if val == nil {
				(*out)[key] = nil
			} else {
				inVal := (*in)[key]
				in, out := &inVal, &outVal
				*out = new(apiextensionsv1.JSON)
				(*in).DeepCopyInto(*out)
			}
			(*out)[key] = outVal
		}
	}
	in.Filter.DeepCopyInto(&out.Filter)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResourceSetInputProviderSpec.
func (in *ResourceSetInputProviderSpec) DeepCopy() *ResourceSetInputProviderSpec {
	if in == nil {
		return nil
	}
	out := new(ResourceSetInputProviderSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ResourceSetInputProviderStatus) DeepCopyInto(out *ResourceSetInputProviderStatus) {
	*out = *in
	out.ReconcileRequestStatus = in.ReconcileRequestStatus
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.ExportedInputs != nil {
		in, out := &in.ExportedInputs, &out.ExportedInputs
		*out = make([]ResourceSetInput, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = make(ResourceSetInput, len(*in))
				for key, val := range *in {
					var outVal *apiextensionsv1.JSON
					if val == nil {
						(*out)[key] = nil
					} else {
						inVal := (*in)[key]
						in, out := &inVal, &outVal
						*out = new(apiextensionsv1.JSON)
						(*in).DeepCopyInto(*out)
					}
					(*out)[key] = outVal
				}
			}
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResourceSetInputProviderStatus.
func (in *ResourceSetInputProviderStatus) DeepCopy() *ResourceSetInputProviderStatus {
	if in == nil {
		return nil
	}
	out := new(ResourceSetInputProviderStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ResourceSetList) DeepCopyInto(out *ResourceSetList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ResourceSet, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResourceSetList.
func (in *ResourceSetList) DeepCopy() *ResourceSetList {
	if in == nil {
		return nil
	}
	out := new(ResourceSetList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ResourceSetList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ResourceSetSpec) DeepCopyInto(out *ResourceSetSpec) {
	*out = *in
	if in.CommonMetadata != nil {
		in, out := &in.CommonMetadata, &out.CommonMetadata
		*out = new(CommonMetadata)
		(*in).DeepCopyInto(*out)
	}
	if in.Inputs != nil {
		in, out := &in.Inputs, &out.Inputs
		*out = make([]ResourceSetInput, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = make(ResourceSetInput, len(*in))
				for key, val := range *in {
					var outVal *apiextensionsv1.JSON
					if val == nil {
						(*out)[key] = nil
					} else {
						inVal := (*in)[key]
						in, out := &inVal, &outVal
						*out = new(apiextensionsv1.JSON)
						(*in).DeepCopyInto(*out)
					}
					(*out)[key] = outVal
				}
			}
		}
	}
	if in.Resources != nil {
		in, out := &in.Resources, &out.Resources
		*out = make([]*apiextensionsv1.JSON, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(apiextensionsv1.JSON)
				(*in).DeepCopyInto(*out)
			}
		}
	}
	if in.DependsOn != nil {
		in, out := &in.DependsOn, &out.DependsOn
		*out = make([]Dependency, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResourceSetSpec.
func (in *ResourceSetSpec) DeepCopy() *ResourceSetSpec {
	if in == nil {
		return nil
	}
	out := new(ResourceSetSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ResourceSetStatus) DeepCopyInto(out *ResourceSetStatus) {
	*out = *in
	out.ReconcileRequestStatus = in.ReconcileRequestStatus
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Inventory != nil {
		in, out := &in.Inventory, &out.Inventory
		*out = new(ResourceInventory)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResourceSetStatus.
func (in *ResourceSetStatus) DeepCopy() *ResourceSetStatus {
	if in == nil {
		return nil
	}
	out := new(ResourceSetStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Sharding) DeepCopyInto(out *Sharding) {
	*out = *in
	if in.Shards != nil {
		in, out := &in.Shards, &out.Shards
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Sharding.
func (in *Sharding) DeepCopy() *Sharding {
	if in == nil {
		return nil
	}
	out := new(Sharding)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Storage) DeepCopyInto(out *Storage) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Storage.
func (in *Storage) DeepCopy() *Storage {
	if in == nil {
		return nil
	}
	out := new(Storage)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Sync) DeepCopyInto(out *Sync) {
	*out = *in
	if in.Interval != nil {
		in, out := &in.Interval, &out.Interval
		*out = new(metav1.Duration)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Sync.
func (in *Sync) DeepCopy() *Sync {
	if in == nil {
		return nil
	}
	out := new(Sync)
	in.DeepCopyInto(out)
	return out
}
