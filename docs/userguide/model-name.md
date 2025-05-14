# Model Name

The `modelName` field under the `routing` section of a `ModelService` specifies how clients refer to a model during inference. This name is used by OpenAI-compatible APIs and must be **globally unique** across all `ModelService` resources in the cluster, unless they target different gateways.

## Purpose

This field acts as the public-facing identifier for the model. When an inference client sends a request, it includes this name in the "model" field of the API request body.

### Client request

```json
{
  "model": "facebook/opt-125m",
  "prompt": "What is the capital of France?"
}
```

### ModelService configuration

```yaml
spec:
  routing:
    modelName: facebook/opt-125m
```

The gateway ensures that each model name maps to one and only one live base model across the cluster.

## Conflict Resolution

If multiple `ModelService` resources attempt to register the same `modelName`, the controller may arbitrarily select one owner, after attempting to apply the following rules.

* The oldest resource (based on creation timestamp) is retained as the valid owner.

* The newer conflicting resource will:

  * Have its inference model marked as not ready.

  * Emit an appropriate status error indicating the conflict.
