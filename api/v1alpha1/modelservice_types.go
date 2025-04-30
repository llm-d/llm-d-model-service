/*
Copyright 2025.

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
	corev1 "k8s.io/api/core/v1"
	res "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ModelService is the Schema for the modelservices API.
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Decouple Scaling",type=boolean,JSONPath=`.spec.decoupleScaling`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type ModelService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ModelServiceSpec   `json:"spec,omitempty"`
	Status ModelServiceStatus `json:"status,omitempty"`
}

// ModelServiceSpec defines the desired state of ModelService
type ModelServiceSpec struct {
	// BaseConfigMapRef provides configuration needed to spawn objects owned by modelservice
	//
	// +optional
	BaseConfigMapRef *corev1.ObjectReference `json:"baseConfigMapRef,omitempty"`
	// Routing provides information needed to create configuration for routing
	//
	// +required
	Routing Routing `json:"routing"`
	// modelArtifacts provides information needed to download artifacts
	// needed to serve a model
	//
	// +required
	ModelArtifacts ModelArtifacts `json:"modelArtifacts"`
	// DecoupleScaling determines who owns the replica fields is the deployment objects
	// Set this to true if the intent is to autoscale with HPA, other autoscalers
	// Setting this to false will force the controller to manage deployment replicas based on
	// replica fields in this model service
	//
	// +optional
	DecoupleScaling bool `json:"decoupleScaling,omitempty"`
	// Decode is the decode portion of the spec
	//
	// +optional
	Decode *PDSpec `json:"decode,omitempty"`
	// Prefill is the prefill portion of the spec
	//
	// +optional
	Prefill *PDSpec `json:"prefill,omitempty"`
}

// ModelServiceList contains a list of ModelService
//
// +kubebuilder:object:root=true
type ModelServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ModelService `json:"items"`
}

// ContainerSpec defines container-level configuration.
type ContainerSpec struct {
	// Name of the container specified as a DNS_LABEL.
	// Each container in a pod must have a unique name (DNS_LABEL).
	// Cannot be updated.
	// +required
	Name string `json:"name"`
	// Image that is used to spawn container if present will override base config
	//
	// +optional
	Image *string `json:"image,omitempty"`
	// Arguments to the entrypoint.
	// The container image's CMD is used if this is not provided.
	// Variable references $(VAR_NAME) are expanded using the container's environment. If a variable
	// cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced
	// to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. "$$(VAR_NAME)" will
	// produce the string literal "$(VAR_NAME)". Escaped references will never be expanded, regardless
	// of whether the variable exists or not. Cannot be updated.
	// More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell
	// +optional
	// +listType=atomic
	Args []string `json:"args,omitempty"`
	// List of environment variables to set in the container.
	// Cannot be updated.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=name
	Env []corev1.EnvVar `json:"env,omitempty"`
	// List of sources to populate environment variables in the container.
	// The keys defined within a source must be a C_IDENTIFIER. All invalid keys
	// will be reported as an event when the container is starting. When a key exists in multiple
	// sources, the value associated with the last source will take precedence.
	// Values defined by an Env with a duplicate key will take precedence.
	// Cannot be updated.
	// +optional
	// +listType=atomic
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty"`
	// Compute Resources required by this container.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// Routing provides the information needed to configure routing
// for a base model. This include creation of InferenceModel.
type Routing struct {
	/*
		// CreateInferencePool indicates if inference pool resource will be created
		CreateInferencePool bool `json:"createInferencePool"`
	*/
	// ModelName is the model field within inference request
	// This should be unique across ModelService objects.
	//
	// If the name is reused, an error will be
	// shown on the status of a ModelService that attempted to reuse.
	// The oldest ModelService, based on creation timestamp, will be selected
	// to remain valid. In the event of a race condition, one will be selected
	// arbitrarily.
	//
	// refer to https://gateway-api-inference-extension.sigs.k8s.io
	// for relationship between model name, inference pool, and inference model
	//
	// From GIE:
	// ModelName is the name of the model as it will be set in the "model" parameter for an incoming request.
	// ModelNames must be unique for a referencing InferencePool
	// (names can be reused for a different pool in the same cluster).
	// The modelName with the oldest creation timestamp is retained, and the incoming
	// InferenceModel is sets the Ready status to false with a corresponding reason.
	// In the rare case of a race condition, one Model will be selected randomly to be considered valid, and the other rejected.
	// Names can be reserved without an underlying model configured in the pool.
	// This can be done by specifying a target model and setting the weight to zero,
	// an error will be returned specifying that no valid target model is found.
	//
	// +required
	// +kubebuilder:validation:MaxLength=256
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="modelName is immutable"
	ModelName string `json:"modelName"`
}

// ModelArtifacts describes the source of the model
type ModelArtifacts struct {
	// URI is the model URI
	// Three types of URIs are support to enable models packaged as images (oci://<image-repo>/<image-name><:image-tag>),
	// models downloaded from HuggingFace (hf://<model-repo>/<model-name>)
	// and pre-existing models loaded from a volume-mounted PVC (pvc://model-path)
	//
	// +required
	URI string `json:"uri"`
	// Name of the authentication secret. Contains HF_TOKEN
	//
	// +optional
	AuthSecretName *string `json:"authSecretName,omitempty"`
	// Size of the model artifacts on disk
	// ensure Size is large enough when providing hf://... URI
	//
	// +optional
	Size *res.Quantity `json:"size,omitempty"`
}

// PDSpec defines the specification for prefill and decode deployments created by ModelService.
type PDSpec struct {
	// Replicas defines the desired number of replicas for this deployment.
	//
	// +optional
	// +nullable
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=1
	Replicas *int32 `json:"replicas,omitempty"`
	// vllm
	// Parallelism specifies vllm parallelism that will be overriden from base config when present.
	//
	// +optional
	Parallelism *Parallelism `json:"parallelism,omitempty"`
	// pod
	// AcceleratorTypes determines the set of accelerators on which
	// this pod will be run. Any matching accelerator type can be used
	// to place the model pods.This will override base config when present
	//
	// +optional
	AcceleratorTypes *AcceleratorTypes `json:"acceleratorTypes,omitempty"`

	// Container holds vllm container container details that will be overriden from base config when present.
	//
	// +optional
	Containers []ContainerSpec `json:"containers,omitempty"`
	// InitContainers holds vllm init container details that will be overriden from base config when present.
	//
	// +optional
	InitContainers []ContainerSpec `json:"vllmProxyContainer,omitempty"`
	// EmphemeralContainers holds vllm ephemeral container details that will be overriden from base config when present.
	//
	// +optional
	EphemeralContainers []ContainerSpec `json:"ephemeralContainers,omitempty"`
}

// ConvertToContainerSlice converts []Containers to []corev1.Container
func ConvertToContainerSlice(c []ContainerSpec) []corev1.Container {

	containerSlice := make([]corev1.Container, len(c))

	for i, containerSpec := range c {
		containerSlice[i] = corev1.Container{
			Name:      containerSpec.Name,
			Env:       containerSpec.Env,
			EnvFrom:   containerSpec.EnvFrom,
			Resources: containerSpec.Resources,
		}

		if c[i].Image != nil {
			containerSlice[i].Image = *c[i].Image
		}
	}

	return containerSlice
}

// Parallelism defines parallelism behavior for vllm.
type Parallelism struct {
	// // NodeParallelism is the number of required nodes
	// //
	// // +optional
	// // +nullable
	// // +kubebuilder:validation:Minimum=0
	// // +kubebuilder:default=1
	// NodeParallelism   *int32 // default 1

	// TensorParallelism corresponds to the same argument in vllm
	// This also corresponds to number of GPUs
	//
	// +optional
	// +nullable
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=1
	Tensor *int32 `json:"tensor,omitempty"`
}

// AcceleratorTypes specifies set of accelerators for scheduling.
type AcceleratorTypes struct {
	// node label key that identifies the accelerator type for the node
	// e.g., nvidia.com/gpu.product
	//
	// +required
	LabelKey string `json:"labelKey,omitempty"`
	// node label values that will be matched against for pod scheduling.
	// e.g., [A100, H100]
	//
	// +required
	// +kubebuilder:validation:MinItems=1
	LabelValues []string `json:"labelValues,omitempty"`
}

// ModelServiceStatus defines the observed state of ModelService
type ModelServiceStatus struct {
	// PrefillDeploymentRef identifies the prefill deployment
	// if prefill stanza is omitted, or if prefill deployment is yet to be created,
	// this reference will be nil
	//
	PrefillDeploymentRef *string `json:"prefillDeploymentRef,omitempty"`
	// DecodeDeploymentRef identifies the decode deployment
	// if decode deployment is yet to be created,
	// this reference will be nil
	//
	DecodeDeploymentRef *string `json:"decodeDeploymentRef,omitempty"`
	// InferenceModelRef identifies the inference model resource
	// if inference model is yet to be created,
	// this reference will be nil
	//
	InferenceModelRef *string `json:"inferenceModelRef,omitempty"`
	// Combined deployment conditions from prefill and decode deployments
	// Condition types should be prefixed to indicate their origin
	// Example types: "PrefillAvailable", "DecodeProgressing", etc.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

func init() {
	SchemeBuilder.Register(&ModelService{}, &ModelServiceList{})
}
