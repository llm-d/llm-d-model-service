# Getting started on `e2e-solution`

0. Log in to OC cluster using login command
1. Set up modelservice controller using: 

```
make -s summitdemo-manifest \
EPP_CLUSTERROLE=endpoint-picker \
IMAGE_PULL_SECRET=quay-secret-llm-d \
LOG_LEVEL=info \
IMG=quay.io/llm-d/llm-d-model-service:0.0.6 \
> summitdemo.yaml

oc apply -f summitdemo.yaml
```

2. Set up `inference-gateway`

```
oc adm policy add-scc-to-user anyuid -z inference-gateway
oc adm policy add-scc-to-user privileged -z inference-gateway -n e2e-solution
```

Modify `kustomiztion.yaml` (namespace) and HTTPRoute (inferencepool) and apply: https://github.com/neuralmagic/llm-d-routing-sidecar/tree/dev/test/config/gateway


3. Port-forward the gateway and send a request

```
oc port-forward svc/inference-gateway 8000:80

curl  http://localhost:8000/v1/completions \
    -H "Content-Type: application/json" \
    -d '{
    "model": "facebook/opt-125m",
    "prompt": "Author-contribution statements and acknowledgements in research papers should state clearly and specifically whether, and to what extent, the authors used AI technologies such as ChatGPT in the preparation of their manuscript and analysis. They should also indicate which LLMs were used. This will alert editors and reviewers to scrutinize manuscripts more carefully for potential biases, inaccuracies and improper source crediting. Likewise, scientific journals should be transparent about their use of LLMs, for example when selecting submitted manuscripts. Mention the large language model based product mentioned in the paragraph above:"
}'
```