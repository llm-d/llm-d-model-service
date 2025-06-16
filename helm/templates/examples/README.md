# Examples

Contains example values file and their rendered templates.

```
cd helm 
helm template [RELEASE-NAME] . -f [VALUES-FILEPATH]
```

1. `facebook/opt-125m`: downloads from Hugging Face 

    ```
    cd helm 
    helm template facebook . -f templates/examples/values-facebook.yaml > templates/examples/output-facebook.yaml
    ```