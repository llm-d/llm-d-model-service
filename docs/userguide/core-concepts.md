# Core Concepts

The ModelService custom resource provides a unified declarative API for serving a base model in Kubernetes. It supports inference workloads with prefill/decode disaggregation, reusable configuration presets, and seamless integration with the Gateway API Inference Extension (GIE).

When a ModelService resource is reconciled, it creates and maintains the following resources.

## Workload resources

* Prefill deployment and service
* Decode deployment and service
* Configmaps for prefill/decode

## Routing

* Inference pool
* Inference model
* Endpoint picker (EPP) deployment and service 
* Configmaps for EPP

## Access control

* Service account for prefill/decode and EPP
* RoleBinding for EPP

These resources are optional and fully configurable. Their creation, omission, and configuration is controlled through BaseConfig and ModelService specifications. When the resources are created, the parent ModelService that triggered their creation is set as their owner; this facilitates correctness of the reconciliation logic, garbage collection and status tracking.

The following sample illustrates the core concepts in the ModelService spec. Further details are covered under individual topics below.

```yaml
apiVersion: llm-d.ai/v1alpha1
kind: ModelService
metadata:
  name: facebook-opt-125m
  # `ModelService` is a namespace scoped resource
  namespace: my-ns
spec:
  # `baseConfigMapRef.name` is the name of the Kubernetes configmap that provides default configurations for the resources spawned by this `ModelService`. 
  
  # configuration derived from this `ModelService` will be semantically merged with the contents of the referenced `BaseConfig` to produce the final resource configuration. This allows model owners to override platform defaults only when necessary.

  # the contents of this `BaseConfig` configmap can be templated
  baseConfigMapRef:
    name: generic-base-config

  # `routing.modelName` is name of the model used by OpenAI compatible inference clients in their queries.
  routing:
    modelName: facebook/opt-125m

  # `modelArtifacts.uri` describes the source of the model. In this example, it is sourced from Hugging Face (as indicated by the hf:// prefix in the URI), the owner of the Hugging Face repo is `facebook`, and the model ID within is `opt-125m`.
  modelArtifacts:
    # if `uri` is prefixed with hf://, it will create an emptyDir volume in prefill/decode pods, that can be mounted by the model serving container in the pod.
    uri: hf://facebook/opt-125m

  # `prefill` and `decode` sections enable disaggregated prefill architecture for model serving; these sections are optional.
  decode:
    # number of decode pods
    replicas: 1
    # a list of containers
    containers:
    - name: vllm
      # Templated arguments.
      # .HFModelName expands to "facebook/opt-125m"
      # For all variables, see the templating reference.
      args:
      # hint: add quote while using templating to avoid subtle yaml parsing issues
      - "{{ .HFModelName }}"
      # if `mountModelVolume` is set to true, the volume meant for model storage will be mounted by this container; in this example, this will be an emptyDir volume into which this container will download the model from Hugging Face.
      mountModelVolume: true
```

This minimal example demonstrates a model deployment using a Hugging Face model. For more on templating, merging, and advanced features, continue with the topics below.

## Topics

