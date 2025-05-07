# Developer Docs

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