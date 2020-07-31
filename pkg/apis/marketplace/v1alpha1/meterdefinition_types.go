// Copyright 2020 IBM Corp.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	"github.com/operator-framework/operator-sdk/pkg/status"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TODO: Add open API validation

// MeterDefinitionSpec defines the desired metering spec
// +k8s:openapi-gen=true
type MeterDefinitionSpec struct {
	// MeterDomain defines the primary CRD domain of the meter
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	Group string `json:"group"`

	// MeterVersion defines the primary CRD version of the meter
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// TODO: delete this
	Version string `json:"version"`

	// MeterKind defines the primary CRD kind of the meter
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	Kind string `json:"kind"`

	// WorkloadVertex is the top most object of a workload. It allows
	// you to identify the upper bounds of your workloads. If you select
	// OperatorGroup it will use the OperatorGroup associated with the
	// Operator to select the namespaces. If you select Namespace, the
	// workloads will be filtered by labels or annotations.
	// +kubebuilder:validation:Enum=Namespace;OperatorGroup
	WorkloadVertex `json:"vertex,omitempty"`

	// VertexFilters are used when Namespace is selected. Can be omitted
	// if you select OperatorGroup
	VertexSelectors map[string]string `json:"vertexSelectors,omitempty"`

	// Workloads identify the workloads to meter.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +kubebuilder:validation:MinItems=1
	Workloads []Workload `json:"workloads,omitempty"`

	// ServiceMeters of the meterics you want to track.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	ServiceMeters []string `json:"serviceMeters,omitempty"`

	// PodMeters of the prometheus metrics you want to track.
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	PodMeters []string `json:"podMeters,omitempty"`

	// Services is the list of discovered services
	// +optional
	Services []*common.ServiceReference `json:"discoveredServices,omitempty"`

	// Pods is the list of discovered pods
	// +optional
	Pods []*common.PodReference `json:"discoveredPods,omitempty"`
}

const (
	VertexOperatorGroup WorkloadVertex = "OperatorGroup"
	VertexNamespace = "Namespace"
)
const (
	WorkloadTypePod WorkloadType = "Pod"
	WorkloadTypeService = "Service"
)

type WorkloadVertex string
type WorkloadType string

// Workload helps identify what to target for metering.
type Workload struct {
	// Name of the workload, must be unique in a meter definition.
	Name string `json:"name"`

	// OwningGVK is the name of the GVK to look for as the owner of all the
	// meterable assets. If omitted, the labels and annotations are used instead.
	Owner GroupVersionKind `json:"owner"`

	// WorkloadType identifies the type of workload to look for. This can be
	// pod or service right now.
  // +kubebuilder:validation:Enum=Pod;Service
	WorkloadType string `json:"type"`

	// Labels are used to filter to the correct workload.
	Labels      map[string]string `json:"labels,omitempty"`

	// Annotations are used to filter to the correct workload.
	Annotations map[string]string `json:"annotations,omitempty"`

	// MetricLabels are the labels to collect
	MetricLabels []MeterLabelQuery
}

// WorkloadStatus provides quick status to check if
// workloads are working correctly
type WorkloadStatus struct {
	// Name of the workload, must be unique in a meter definition.
	Name string `json:"name"`

	CurrentMetricValue string `json:"currentValue"`

	LastReadTime metav1.Time `json:"startTime"`
}

type GroupVersionKind struct {
	// ApiVersion of the CRD
	ApiVersion string `json:"apiVersion"`
	// Kind of the CRD
	Kind       string `json:"kind"`
}

// MeterLabelQuery helps define a meter label to build and search for
type MeterLabelQuery struct {
	// Label is the name of the meter
	Label string `json:"label"`

	// Query to use for the label
	// +kubebuilder:default={}
	Query string `json:"query,omitempty"`

	// Aggregation to use with the query
	// +kubebuilder:default=sum
	// +kubebuilder:validation:Enum:=sum;min;max;avg;count
	Aggregation string `json:"aggregation,omitempty"`
}

// MeterDefinitionStatus defines the observed state of MeterDefinition
// +k8s:openapi-gen=true
// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
type MeterDefinitionStatus struct {

	// Conditions represent the latest available observations of an object's state
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	Conditions status.Conditions `json:"conditions,omitempty"`

	// ServiceLabels of the meterics you want to track.
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	ServiceLabels []string `json:"serviceLabels"`

	// PodLabels of the prometheus kube-state metrics you want to track.
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	PodLabels []string `json:"podLabels"`

	// OperatorGroup is the identifier for the owning operatorGroup
	OperatorGroup common.NamespacedNameReference `json:"operatorGroup"`

	// ServiceMonitors is the list of service monitors being watched for
	// this meter definition
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	ServiceMonitors []*metav1.ObjectMeta `json:"serviceMonitors"`

	// Pods is the list of current pods being watched for
	// this meter definition
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	Pods []*metav1.ObjectMeta `json:"pods"`
}

// MeterDefinition is internal Meter Definitions defined by Operators from Red Hat Marketplace.
// This is an internal resource not meant to be modified directly.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
//
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=meterdefinitions,scope=Namespaced
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="(Internal) Meter Definitions"
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`ServiceMonitor,v1,"redhat-marketplace-operator"`
type MeterDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MeterDefinitionSpec   `json:"spec,omitempty"`
	Status MeterDefinitionStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MeterDefinitionList contains a list of MeterDefinition
type MeterDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MeterDefinition `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MeterDefinition{}, &MeterDefinitionList{})
}
