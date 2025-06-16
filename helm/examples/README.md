# Examples

Contains example values file and their rendered templates.

```
cd helm 
helm template [RELEASE-NAME] . -f [VALUES-FILEPATH]
```

1. `facebook/opt-125m`: downloads from Hugging Face 

    ```
    cd helm 
    helm template facebook . -f examples/values-facebook.yaml > examples/output-facebook.yaml
    ```
    
    
    
curl http://localhost:8000/v1/completions -vvv \
    -H "Content-Type: application/json" \
    -H "x-model-name: facebook/opt-125m" \
    -d '{
    "model": "facebook/opt-125m",
    "prompt": "Hello, "
}'