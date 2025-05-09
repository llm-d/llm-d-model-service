# Developer Docs

The ModelService controller is available in the GitHub repository [neuralmagic/llm-d-model-service](https://github.com/neuralmagic/llm-d-model-service). 
Clone the repository to take advantage of the `make` commands described below.

Execution of the ModelService controller requires access to a cluster.
A local cluster, such as a `kind` cluster suffices for basic execution and development testing.
However, full function may not be available if the cluster does not have sufficient resources or if the [inference gateway](https://github.com/neuralmagic/gateway-api-inference-extension) is not fully configured.

If a cluster is not available, you can do a dry-run to identify the Kubernetes resources that will be created for a given `ModelService CR`. See [ModelService Dry Run](#modelservice-dry-run) below.

## Prerequisites

### Install Kubernetes Gateway API Inference Extension CRDs

```shell
VERSION=v0.3.0
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api-inference-extension/releases/download/$VERSION/manifests.yaml
```

### Define Cluster Role for Endpoint Picker (EPP)

For the endpoint picker used in the [samples](https://github.com/neuralmagic/llm-d-model-service/tree/dev/samples), the `pod-read` cluster role defined [here](https://github.com/neuralmagic/gateway-api-inference-extension/blob/dev/config/manifests/inferencepool-resources.yaml#L84-L112) works.

### Install ModelService CRDs

```shell
make install
```

## Local Execution

You can run the ModelService controller locally operating against the cluster defined by your current Kubernetes configurtion.

```shell
make run EPP_CLUSTERRROLE=pod-read
```

You can now create `ModelService` objects. See [samples](https://github.com/neuralmagic/llm-d-model-service/tree/dev/samples) for details.

## Running in a Cluster

Deploy the controller to the cluster:

1. Create the target namespace `modelservice-system`

    By default, the ModelService controller is deployed to the `modelservice-system` namespace. To change the target namespace, create a kustomize overlay (see `config/dev-deploy`).

2. Deploy the controller:

    ```shell
    make dev-deploy EPP_CLUSTERRROLE=pod-read
    ```

If an image pull secret is required, you can specify it with the environment variable `IMAGE_PULL_SECRET`.

You can now create `ModelService` objects. See [samples](https://github.com/neuralmagic/llm-d-model-service/tree/dev/samples) for details.

## Uninstall

The controller and `ModelService` CRDs can be removed:

```shell
make uninstall && make undeploy 
```

## ModelService Dry-Run
View the components that ModelService will create given a `ModelService` CR and a base config `ConfigMap`. This command does not require cluster access.

In the `llm-d-model-service`project root directory:

```
go run main.go generate \
--epp-cluster-role=<name-of-endpoint-picker-cluster-role> \
--modelservice <path-to-msvc-cr> \
--baseconfig <path-to-baseconfig>
```

Note that because no cluster access is required, it is not necessary to create an endpoint picker cluster role resource.

For example:

```
go run main.go generate \
--epp-cluster-role=pod-read \
--modelservice samples/msvcs/granite3.2.yaml \
--baseconfig samples/baseconfigs/simple-baseconfig.yaml
```

will output the YAML manifest for the resources that ModelService will create in the cluster. Some fields that require cluster access to define, will not be included, such as `owner references` and `name`.

This feature purely for development purposes, and is intended to provide a quick way of debugging without a cluster. 