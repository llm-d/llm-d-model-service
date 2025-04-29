# Model Service

`ModelService` is a Kubernetes operator (CRD + controller) that enables the creation of vllm pods and routing resources for a given model.

## Key Features

- Simple Kubernetes API for platform engineers
- Enables disaggregated prefill
- Supports creation of [Gateway API Inference Extension](https://gateway-api-inference-extension.sigs.k8s.io) resources for routing
- Supports auto-scaling with HPA
- Supports independent scaling of prefill and decode instances
- Supports independent node affinities for prefill and decode instances
- Supports model loading from OCI images, HuggingFace public and private registries, and PVCs

## Design

![model-service-arch](model-service-arch.png)

## Samples

Refer to the [`samples` folder](samples).

## Run `ModelService` locally

### Create kind cluster

```sh
kind create cluster
```
### Install InferenceModels and InferencePool CRDs

```sh
VERSION=v0.3.0
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api-inference-extension/releases/download/$VERSION/manifests.yaml
```

### Running controller

```sh
make install && make run
```

### Uninstall

```sh
make uninstall && make undeploy 
```

### Delete cluster
```sh
kind delete cluster
```

