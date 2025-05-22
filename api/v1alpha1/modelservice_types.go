package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	res "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// ModelService is the Schema for the modelservices API.
//
// +kubebuilder:resource:shortName=msvc,scope=Namespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Decouple Scaling",type=boolean,JSONPath=`.spec.decoupleScaling`
// +kubebuilder:printcolumn:name="Prefill READY",type=string,JSONPath=`.status.prefillReady`
// +kubebuilder:printcolumn:name="Prefill AVAIL",type=integer,JSONPath=`.status.prefillAvailable`
// +kubebuilder:printcolumn:name="Decode READY",type=string,JSONPath=`.status.decodeReady`
// +kubebuilder:printcolumn:name="Decode AVAIL",type=integer,JSONPath=`.status.decodeAvailable`
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
	// EndpointPicker is the endpoint picker (epp) portion of the spec
	//
	// +optional
	EndpointPicker *ModelServicePodSpec `json:"endpointPicker,omitempty"`
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

	// Entrypoint array. Not executed within a shell.
	// The container image's ENTRYPOINT is used if this is not provided.
	// Variable references $(VAR_NAME) are expanded using the container's environment. If a variable
	// cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced
	// to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. "$$(VAR_NAME)" will
	// produce the string literal "$(VAR_NAME)". Escaped references will never be expanded, regardless
	// of whether the variable exists or not. Cannot be updated.
	// More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell
	// +optional
	// +listType=atomic
	Command []string `json:"command,omitempty" protobuf:"bytes,3,rep,name=command"`

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

	// Boolean to indicate mounting the model artifacts to this container
	// For URIs with pvc:// prefix, a model-storage volume is created and mounted with the mountPath: /cache
	// For URIs with hf:// prefix, modelArtifact.authSecretName is used as the secret key reference,
	// and the value is mounted to an environment variable called HF_TOKEN
	// For URIs with oci:// prefix, an OCI volume with image reference (https://kubernetes.io/blog/2024/08/16/kubernetes-1-31-image-volume-source/)
	// is created and mounted with the mountPath oci-dir
	// default:false
	// +optional
	MountModelVolume bool `json:"mountModelVolume,omitempty"`
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

	// Ports is a list of named ports
	// These can be referenced by name in configuration of base configuration or model services
	// +optional
	Ports []Port `json:"ports,omitempty"`

	// GatewayRef is merged to baseconfig based on the Name field.
	// Directly from Gateway API: https://gateway-api.sigs.k8s.io/reference/spec/#commonroutespec
	// ParentRefs references the resources (usually Gateways) that a Route wants
	// to be attached to. Note that the referenced parent resource needs to
	// allow this for the attachment to be complete. For Gateways, that means
	// the Gateway needs to allow attachment from Routes of this kind and
	// namespace. For Services, that means the Service must either be in the same
	// namespace for a "producer" route, or the mesh implementation must support
	// and allow "consumer" routes for the referenced Service. ReferenceGrant is
	// not applicable for governing ParentRefs to Services - it is not possible to
	// create a "producer" route for a Service in a different namespace from the
	// Route.
	//
	// There are two kinds of parent resources with "Core" support:
	//
	// * Gateway (Gateway conformance profile)
	// * Service (Mesh conformance profile, ClusterIP Services only)
	//
	// This API may be extended in the future to support additional kinds of parent
	// resources.
	//
	// ParentRefs must be _distinct_. This means either that:
	//
	// * They select different objects.  If this is the case, then parentRef
	//   entries are distinct. In terms of fields, this means that the
	//   multi-part key defined by `group`, `kind`, `namespace`, and `name` must
	//   be unique across all parentRef entries in the Route.
	// * They do not select different objects, but for each optional field used,
	//   each ParentRef that selects the same object must set the same set of
	//   optional fields to different values. If one ParentRef sets a
	//   combination of optional fields, all must set the same combination.
	//
	// Some examples:
	//
	// * If one ParentRef sets `sectionName`, all ParentRefs referencing the
	//   same object must also set `sectionName`.
	// * If one ParentRef sets `port`, all ParentRefs referencing the same
	//   object must also set `port`.
	// * If one ParentRef sets `sectionName` and `port`, all ParentRefs
	//   referencing the same object must also set `sectionName` and `port`.
	//
	// It is possible to separately reference multiple distinct objects that may
	// be collapsed by an implementation. For example, some implementations may
	// choose to merge compatible Gateway Listeners together. If that is the
	// case, the list of routes attached to those resources should also be
	// merged.
	//
	// Note that for ParentRefs that cross namespace boundaries, there are specific
	// rules. Cross-namespace references are only valid if they are explicitly
	// allowed by something in the namespace they are referring to. For example,
	// Gateway has the AllowedRoutes field, and ReferenceGrant provides a
	// generic way to enable other kinds of cross-namespace reference.
	//
	// <gateway:experimental:description>
	// ParentRefs from a Route to a Service in the same namespace are "producer"
	// routes, which apply default routing rules to inbound connections from
	// any namespace to the Service.
	//
	// ParentRefs from a Route to a Service in a different namespace are
	// "consumer" routes, and these routing rules are only applied to outbound
	// connections originating from the same namespace as the Route, for which
	// the intended destination of the connections are a Service targeted as a
	// ParentRef of the Route.
	// </gateway:experimental:description>
	//
	// +optional
	// +kubebuilder:validation:MaxItems=32
	// <gateway:standard:validation:XValidation:message="sectionName must be specified when parentRefs includes 2 or more references to the same parent",rule="self.all(p1, self.all(p2, p1.group == p2.group && p1.kind == p2.kind && p1.name == p2.name && (((!has(p1.__namespace__) || p1.__namespace__ == '') && (!has(p2.__namespace__) || p2.__namespace__ == '')) || (has(p1.__namespace__) && has(p2.__namespace__) && p1.__namespace__ == p2.__namespace__ )) ? ((!has(p1.sectionName) || p1.sectionName == '') == (!has(p2.sectionName) || p2.sectionName == '')) : true))">
	// <gateway:standard:validation:XValidation:message="sectionName must be unique when parentRefs includes 2 or more references to the same parent",rule="self.all(p1, self.exists_one(p2, p1.group == p2.group && p1.kind == p2.kind && p1.name == p2.name && (((!has(p1.__namespace__) || p1.__namespace__ == '') && (!has(p2.__namespace__) || p2.__namespace__ == '')) || (has(p1.__namespace__) && has(p2.__namespace__) && p1.__namespace__ == p2.__namespace__ )) && (((!has(p1.sectionName) || p1.sectionName == '') && (!has(p2.sectionName) || p2.sectionName == '')) || (has(p1.sectionName) && has(p2.sectionName) && p1.sectionName == p2.sectionName))))">
	// <gateway:experimental:validation:XValidation:message="sectionName or port must be specified when parentRefs includes 2 or more references to the same parent",rule="self.all(p1, self.all(p2, p1.group == p2.group && p1.kind == p2.kind && p1.name == p2.name && (((!has(p1.__namespace__) || p1.__namespace__ == '') && (!has(p2.__namespace__) || p2.__namespace__ == '')) || (has(p1.__namespace__) && has(p2.__namespace__) && p1.__namespace__ == p2.__namespace__)) ? ((!has(p1.sectionName) || p1.sectionName == '') == (!has(p2.sectionName) || p2.sectionName == '') && (!has(p1.port) || p1.port == 0) == (!has(p2.port) || p2.port == 0)): true))">
	// <gateway:experimental:validation:XValidation:message="sectionName or port must be unique when parentRefs includes 2 or more references to the same parent",rule="self.all(p1, self.exists_one(p2, p1.group == p2.group && p1.kind == p2.kind && p1.name == p2.name && (((!has(p1.__namespace__) || p1.__namespace__ == '') && (!has(p2.__namespace__) || p2.__namespace__ == '')) || (has(p1.__namespace__) && has(p2.__namespace__) && p1.__namespace__ == p2.__namespace__ )) && (((!has(p1.sectionName) || p1.sectionName == '') && (!has(p2.sectionName) || p2.sectionName == '')) || ( has(p1.sectionName) && has(p2.sectionName) && p1.sectionName == p2.sectionName)) && (((!has(p1.port) || p1.port == 0) && (!has(p2.port) || p2.port == 0)) || (has(p1.port) && has(p2.port) && p1.port == p2.port))))">
	GatewayRefs []gatewayv1.ParentReference `json:"gatewayRefs,omitempty"`
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

// ModelServicePodSpec defines the specification for pod templates that will be created by ModelService.
type ModelServicePodSpec struct {
	// Replicas defines the desired number of replicas for this deployment.
	//
	// +optional
	// +nullable
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=1
	Replicas *int32 `json:"replicas,omitempty"`
	// Container holds vllm container container details that will be overridden from base config when present.
	//
	// +optional
	Containers []ContainerSpec `json:"containers,omitempty"`
	// InitContainers holds vllm init container details that will be overridden from base config when present.
	//
	// +optional
	InitContainers []ContainerSpec `json:"initContainers,omitempty"`
}

// PDSpec defines the specification for prefill and decode deployments created by ModelService.
type PDSpec struct {
	//
	ModelServicePodSpec `json:",inline"`
	// vllm
	// Parallelism specifies vllm parallelism that will be overridden from base config when present.
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
	// EppDeploymentRef identifies the epp deployment
	// if epp deployment is yet to be created,
	// this reference will be nil
	//
	EppDeploymentRef *string `json:"eppDeploymentRef,omitempty"`
	// HTTPRoute identifies the HTTPRoute resource
	// if HTTPRoute is yet to be created,
	// this reference will be nil
	//
	HTTPRouteRef *string `json:"httpRouteRef,omitempty"`
	// InferenceModelRef identifies the inference model resource
	// if inference model is yet to be created,
	// this reference will be nil
	//
	InferenceModelRef *string `json:"inferenceModelRef,omitempty"`
	// InferencePoolRef identifies the inference pool resource
	// if inference pool is yet to be created,
	// this reference will be nil
	//
	InferencePoolRef *string `json:"inferencePoolRef,omitempty"`
	//
	// PDServiceAccountRef identifies the service account for PD
	// if PDServiceAccountRef is yet to be created,
	// this reference will be nil
	//
	PDServiceAccountRef *string `json:"prefillServiceAccountRef,omitempty"`
	//
	// DecodeServiceAccountRef identifies the service account for decode
	// if DecodeServiceAccountRef is yet to be created,
	// this reference will be nil
	//
	DecodeServiceAccountRef *string `json:"decodeServiceAccountRef,omitempty"`
	//
	// EppRoleBinding identifies the rolebinding for Epp
	// if EppRoleBinding is yet to be created,
	// this reference will be nil
	//
	EppRoleBinding *string `json:"eppRoleBinding,omitempty"`
	//
	// ConfigMapNames identifies the configmap used for prefill and decode
	// if ConfigMapNames is yet to be created,
	// this reference will be an empty list
	//
	ConfigMapNames []string `json:"configMapNames,omitempty"`

	// READY and AVAILABLE for prefill
	PrefillReady     string `json:"prefillReady"` // e.g. "1/1"
	PrefillAvailable int32  `json:"prefillAvailable"`

	// READY and AVAILABLE for decode
	DecodeReady     string `json:"decodeReady"`
	DecodeAvailable int32  `json:"decodeAvailable"`

	// READY and AVAILABLE for Epp
	EppReady     string `json:"eppReady"`
	EppAvailable int32  `json:"eppAvailable"`

	// Combined deployment conditions from prefill and decode deployments
	// Condition types should be prefixed to indicate their origin
	// Example types: "PrefillAvailable", "DecodeProgressing", etc.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type Port struct {
	// Name that can be used in place of port number in templates
	// +required
	Name string `json:"name"`
	// Value of port
	// +kubebuilder:validation:Minimum=1
	// +required
	Port int32 `json:"port"`
}

func init() {
	SchemeBuilder.Register(&ModelService{}, &ModelServiceList{})
}
