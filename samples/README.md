# Sample ModelService CRs and BaseConfigs

This folder contains example baseconfigs (ConfigMap) and ModelService CRs for various scenarios, including downloading models from Hugging Face or loading from a PVC. In particular, we provide a "universal" baseconfig that can be used for those scenarios. We will also show how to apply to an OpenShift cluster and the results you can expect from applying the ModelService CR and its referenced baseconfig. 

👉 [baseconfigs](./baseconfigs/)

👉 [msvcs](./msvcs/)

👉 [test](./test/) (for local development usage only)

## Prerequisite
Before you get started, ensure that you have the following install and running.

- Access to an OpenShift cluster 
- ModelService controller running with required RBACs and image pull secrets for P/D and EPP deployments
- External CRDs install on cluster for routing
- Other components such as [Inference Gateway Extension](https://github.com/kubernetes-sigs/gateway-api-inference-extension)

If you need more guidance, refer to the [developer docs](../docs/developer.md) on how to properly install the ModelService controller and other components on your system.

## Scenarios 
*Note: we have set `acceleratorTypes` to A100-80GB, please reset as per resurces available in your cluster*
### Scenario 1: serving a base model on vLLM on one pod
A simple use case is online serving a model on vLLM using one deployment. We will serve [`ibm-granite/granite-3.3-2b-base`](https://huggingface.co/ibm-granite/granite-3.3-2b-base) which can be downloaded from Hugging Face without the need for a token.

- [msvcs/granite3.2.yaml](./msvcs/granite3.2.yaml)
- [baseconfigs/simple-baseconfig.yaml](./baseconfigs/simple-baseconfig.yaml)

The `simple-baseconfig` contains just one section for `decodeDeployment`, which is a deployment template that spins up one vLLM container. It also specifies a volume and volume mount for downloading the model. The `decodeService` section is optional.

*Note: the term `decodeDeployment` might be misleading. There is no P/D disagreegation in this example. Using `prefillDeployment` will achieve the same result. We just need a deployment template for serving the model.*

Applying the baseconfig and CR to an OpenShift cluster, you should expect a deployment and service getting created. 

```
oc apply -f samples/baseconfigs/simple-baseconfig.yaml
oc apply -f samples/msvcs/granite3.2.yaml
```

You may port-forward the pod or service at port 8000 (because those are the ports for the vLLM container and decode service specified in the baseconfig) and query the vLLM container. The following command port-forwards the service.

```
oc port-forward svc/granite-base-model-service-decode 8000:8000
curl  http://localhost:8000/v1/completions \
    -H "Content-Type: application/json" \
    -d '{
    "model": "ibm-granite/granite-3.3-2b-base",
    "prompt": "New York is"
}'
```

### Scenario 2: serving a model with routing and P/D disaggregation support
The platform owner may create another baseconfig used to serve models with routing enabled, useful for P/D disaggregation. We will continue to use a model that can be downloaded from Hugging Face: `facebook/opt-125m`.

- [msvcs/facebook-nixl.yaml](./msvcs/facebook-nixl.yaml)
- [baseconfigs/simple-baseconfig.yaml](./baseconfigs/universal-baseconfig.yaml)

Applying the baseconfig and CR to an OpenShift cluster, you should expect a deployment and service getting created. 

```
oc apply -f samples/baseconfigs/universal-baseconfig.yaml
oc apply -f samples/msvcs/facebook-nixl.yaml
```

You should expect to see the following resources created for this scenario:

- Model components:
  - A decode deployment
  - A prefill deployment
- Routing components:
  - An InferencePool
  - An InferenceModel
  - An EPP deployment 
- Networking components 
  - A service for decode deployment
  - A service for prefill deployment
  - A service for EPP deployment
- RBAC components 
  - A service account for P/D deployments (for custom image pulls)
  - A service account for EPP deployment 
  - A rolebinding for EPP deployment 

You may port-forward the services or pods and query vLLM directly. Optionally, if inference-gateway is installed in the cluster, use that which will route to the EPP. 

```
oc port-forward svc/inference-gateway 8000:<inference-gateway-port>
curl  http://localhost:8000/v1/completions \
    -H "Content-Type: application/json" \
    -d '{
    "model": "facebook/opt-125m",
    "prompt": "Author-contribution statements and acknowledgements in research papers should state clearly and specifically whether, and to what extent, the authors used AI technologies such as ChatGPT in the preparation of their manuscript and analysis. They should also indicate which LLMs were used. This will alert editors and reviewers to scrutinize manuscripts more carefully for potential biases, inaccuracies and improper source crediting. Likewise, scientific journals should be transparent about their use of LLMs, for example when selecting submitted manuscripts. Mention the large language model based product mentioned in the paragraph above:"
}'
```

### Scenario 3: serving a model with xPyD disaggregation
Previously, we have looked at MSVCs which have just one replica for decode and prefill workloads. ModelService can help you achieve xPyD disaggregation, and all that is required is using different `replica` in the prefill and decode specs. 

Note that in this scenario, we are using the same baseconfig used in the last scenario, because there is really no difference in terms of the base configuration between the two other than model-specific behaviors such as replica count and model name.

- [msvcs/xpyd.yaml](./msvcs/xpyd.yaml)
- [baseconfigs/universal-baseconfig.yaml](./baseconfigs/universal-baseconfig.yaml)

```
oc apply -f samples/msvcs/xpyd.yaml
```

and you should see the corresponding number of pods spin up for each deployment.

### Scenario 4: loading a large model from a PVC 
Downloading a model from Hugging Face takes a long time for large models like [`meta-llama/Llama-4-Scout-17B-16E`](https://huggingface.co/meta-llama/Llama-4-Scout-17B-16E), and one way to circumvent the long container creation time is to download a model to a PVC ahead of time and mount the PVC in the vLLM container. We have provided a baseconfig with the volume mounts configured, and all that is needed in the ModelService CR is to specify the path to which the model can be found.

- [msvcs/llama4.yaml](./msvcs/llama4.yaml)
- [baseconfigs/universal-baseconfig-pvc.yaml](./baseconfigs/universal-baseconfig-pvc.yaml)

```
oc apply -f samples/baseconfigs/universal-baseconfig-pvc.yaml
oc apply -f samples/msvcs/llama4.yaml
```

This should drastically shorten the wait time for pod creation. 