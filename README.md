# Model Service

> A Modelservice manages the inference workloads and routing resources for a given base-model. 

A *Modelservice* provides declarative updates for prefill and decode deployments, inference pool, inference model, the endpoint picker (epp) deployment and service associated with the inference pool, and the RBAC resources associated with them.

You describe the desired state of the base-model in a *Modelservice* and in an optional *Baseconfig* config map that is referred to by the Modelservice. The `Modelservice` controller changes the actual state of the base-model to the desired state. 

> Note: Do not manage the objects owned by a ModelService. Consider opening an issue in the main llm-d repository if your use case is not covered below.

![model-service-arch](model-service-arch.png)

## Reconcilation

The values in `baseconfig`, and certain values in the `modelservice` resource can be templated. When the `modelservice` resource is reconciled:

1. Template variables in `baseconfig` and `modelservice` are dynamically interpolated based on the `modelservice` spec.
2. A semantic merge takes place between `baseconfig` and `modelservice`.
3. Inference workloads, routing resources, and RBACs authorizations needed for running the base-model are created or updated in the cluster.

> Note on best-practice: `Baseconfig` is intended to capture configuration that is common across a collection of base-models. `Modelservice` is intended to capture configuration specific to a single base-model, and extend or selectively override the values in `Baseconfig` it refers to. The platform owner is expected to install `llm-d` with a collection of `Baseconfig` presets. Inference owners are expected to take advantage of these presets to serve their base-models using the simplified `Modelservice` spec.

## Key Features

- Enables disaggregated prefill
- Supports creation of [Gateway API Inference Extension](https://gateway-api-inference-extension.sigs.k8s.io) resources for routing
- Supports auto-scaling of prefill and decode deployments with HPA and/or other auto-scalers
- Supports independent scaling of prefill and decode instances
- Supports independent node affinities for prefill and decode instances
- Supports model loading from OCI images, HuggingFace public and private registries, and PVCs
- Supports templating for `baseconfig` values and certain `modelservice` values.

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

### ModelService dry run
View the components that ModelService will create given a ModelService CR and a base config ConfigMap. 

Make sure you are at the root directory of `llm-d-model-service`

```
cd llm-d-model-service
go run main.go generate --modelservice <path-to-msvc-cr> --baseconfig <path-to-baseconfig>
```

For example

```
go run main.go generate -m samples/facebook/msvc.yaml -b samples/facebook/baseconfig.yaml > output.yaml
```

And `output.yaml` will contain the YAML manifest for the resources that ModelService will create in the cluster. This feature purely for development purposes, and is intended to provide a quick way of debugging without a cluster. Note that some fields will not be included, such as `owner references` and `name` which require a cluster.