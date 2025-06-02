# Sample ModelService CRs and BaseConfigs

This folder contains example baseconfigs (ConfigMap) and ModelService CRs for various scenarios, including downloading models from Hugging Face or loading from a PVC. In particular, we provide a "universal" baseconfig that can be used for those scenarios. We will also show how to apply to an Kubernetes cluster and the results you can expect from applying the ModelService CR and its referenced baseconfig. 

## Prerequisite
Before you get started, ensure that you have the following install and running.

- Access to a Kubernetes cluster 
- ModelService controller running with required RBACs and image pull secrets for P/D and EPP deployments
- External CRDs install on cluster for routing
- Other components such as [Inference Gateway Extension](https://github.com/kubernetes-sigs/gateway-api-inference-extension)

If you need more guidance, refer to the [developer docs](../docs/developer.md) on how to properly install the ModelService controller and other components on your system.

## Table of Contents
Navigate to each scenario's README, which not only proivides detail commands to try, but also presents the expected output. We suggest that you go through this list in order, as the ModelService and BaseConfig definitions grow in complexity.

1. [Serving a base model on vLLM on one pod](./simple-model/)
2. [Serving a model with routing and P/D disaggregation support](./nixl-xpyd/README.md#scenario-2-serving-a-model-with-routing-and-pd-disaggregation-support)
3. [Serving a model with xPyD disaggregation](./nixl-xpyd/README.md#scenario-3-serving-a-model-with-xpyd-disaggregation)
4. [Loading a large model from a PVC](./pvc/) (disabled for now)
5. [test](./test/) are for local development usage only.

## Questions

[`llm-d-model-service`](https://github.com/llm-d/llm-d-model-service/) additional more patterns, details, and documentation on ModelService.
