# Examples

Contains example values file and their rendered templates.

```
cd helm 
helm template [RELEASE-NAME] . -f [VALUES-FILEPATH]
```

1. `vllm-sim` in Kind 

    Make sure there is a gateway (Kgteway or Istio) deployed in the cluster named `llm-d-inference-gateway` or change values file accordingly.
    
    ```
    helm template llmd-sim . -f examples/values-vllm-sim.yaml > examples/output-vllm-sim.yaml
    ```
    
    Remove `protocol: tcp` in `initContainers` and `readinessProbe` and `livenessProbe` from epp deployment


2. `facebook/opt-125m`: downloads from Hugging Face 

    ```
    helm template facebook . -f examples/values-facebook.yaml > examples/output-facebook.yaml
    ```
    
    
Port forward the inference gateway 

```
k port-forward svc/llm-d-inference-gateway-istio 8000:80
```
    
Send a request

```
curl http://localhost:8000/v1/completions -vvv \
    -H "Content-Type: application/json" \
    -H "x-model-name: facebook/opt-125m" \
    -d '{
    "model": "facebook/opt-125m",
    "prompt": "Hello, "
}'
```