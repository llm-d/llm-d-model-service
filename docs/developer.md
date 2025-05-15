# Developer Docs

Clone the [ModelService GitHub repository](https://github.com/llm-d/llm-d-model-service) (or a fork of it) to take advantage of the `make` commands described below.  All commands are from the project root directory.

Execution of the ModelService controller requires access to a cluster.
A local cluster, such as a `kind` cluster, suffices for basic execution and development testing.
However, testing end-to-end with a large language model may not be possible if the cluster does not have sufficient resources or if the [inference gateway](https://github.com/llm-d/gateway-api-inference-extension) is not fully configured.

If a cluster is not available, you can do a dry-run to identify the Kubernetes resources that will be created for a given `ModelService CR`. See [ModelService Dry Run](#modelservice-dry-run) below.

## Prerequisites

### Install Kubernetes Gateway API Inference Extension CRDs

```shell
VERSION=v0.3.0
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api-inference-extension/releases/download/$VERSION/manifests.yaml
```

### Define Cluster Role for Endpoint Picker (EPP)

For the endpoint picker used in the [samples](https://github.com/llm-d/llm-d-model-service/tree/dev/samples), the `pod-read` cluster role defined [here](https://github.com/llm-d/gateway-api-inference-extension/blob/dev/config/manifests/inferencepool-resources.yaml#L84-L112) works.

### Install ModelService CRDs

```shell
make install
```

If successful, you should see something like:

```shell
% kubectl get crd | grep modelservice
modelservices.llm-d.ai                                            2025-05-08T13:37:32Z
```

## Local Execution

You can run the ModelService controller locally operating against the cluster defined by your current Kubernetes configuration.

```shell
make run EPP_CLUSTERROLE=pod-read
```

You can now create `ModelService` objects. See [samples](https://github.com/llm-d/llm-d-model-service/tree/dev/samples) for details.

To avoid long image and model downloads, you can create dummy model services such as those in[ `samples/test`](https://github.com/llm-d/llm-d-model-service/tree/dev/samples/test).

## Running in a Cluster

Deploy the controller to the cluster:

1. Create the target namespace `modelservice-system`

    By default, the ModelService controller is deployed to the `modelservice-system` namespace. To change the target namespace, create a kustomize overlay (see [`config/dev`](https://github.com/llm-d/llm-d-model-service/tree/dev/config/dev)).

2. Deploy the controller:

    ```shell
    make dev-deploy EPP_CLUSTERROLE=pod-read
    ```

    You should see a `modelservice-controller-manager` pod start in the `modelservice-system` namespace.

    If an image pull secret is required, you can specify it with the environment variable `IMAGE_PULL_SECRET`.

You can now create `ModelService` objects. See [samples](https://github.com/llm-d/llm-d-model-service/tree/dev/samples) for details.

## Uninstall

The controller and `ModelService` CRDs can be removed:

```shell
make uninstall && make undeploy 
```

Supporting resources like the endpoint picker cluster role, the inference gateway, and the Kubernetes Gateway APi Inference Extension CRDs can also be uninstalled.

## ModelService Dry-Run
View the components that ModelService will create given a `ModelService` CR and a base config `ConfigMap`. This command does not require cluster access.

In the `llm-d-model-service`project root directory:

```shell
go run main.go generate \
--epp-cluster-role=<name-of-endpoint-picker-cluster-role> \
--modelservice <path-to-msvc-cr> \
--baseconfig <path-to-baseconfig>
```

Note that because no cluster access is required, it is not necessary to create an endpoint picker cluster role resource.

For example:

```shell
go run main.go generate \
--epp-cluster-role=pod-read \
--modelservice samples/msvcs/granite3.2.yaml \
--baseconfig samples/baseconfigs/simple-baseconfig.yaml
```

will output the YAML manifest for the resources that ModelService will create in the cluster. Some fields that require cluster access to define, will not be included, such as `metadata.namespace`.

This feature purely for development purposes, and is intended to provide a quick way of debugging without a cluster. 