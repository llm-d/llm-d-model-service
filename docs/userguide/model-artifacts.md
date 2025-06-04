# Model Artifacts

The `modelArtifacts` section under the `spec` of a `ModelService` defines how model files, such as weights and metadata configurations, are retrieved and loaded into inference backends like vLLM. This abstraction simplifies the process by allowing users to specify the model source without needing to configure low-level details like environment variables, volumes, or volume mounts.

## Purpose

The `ModelService` controller automates configurations, enabling users to focus solely on specifying the model source. Without `ModelService`, users must manually configure vLLM arguments, environment variables, and pod/container specifications. This requires a deep understanding of both vLLM and the composition of model artifacts. 

## Model Artifact Sources and Behaviors

The `modelArtifacts.uri` field determines the source of the model artifacts. Each supported prefix results in specific behaviors in the prefill and decode deployments. The following sources are supported:

### 1. Downloading a Model Directly from Hugging Face

If the `uri` begins with the `hf://` prefix, the model is downloaded directly from Hugging Face into an `emptyDir` volume.

#### URI Format

The repo and model ID must match exactly to the IDs found on the Hugging Face model registry, as required by vLLM.

`hf://<repo-id>/<model-id>`  

Example: `hf://facebook/opt-125m`

#### Additional Fields

- **`authSecretName`**: Specifies the Kubernetes Secret containing the `HF_TOKEN` for gated models.
- **`size`**: Defines the size of the `emptyDir` volume.

#### Behavior

- An `emptyDir` volume named `model-storage` is created.
- Containers with `mountModelVolume: true` will have a `volumeMount` at `/model-cache`.
- The `HF_HOME` environment variable is set to `/model-cache`.
- If `authSecretName` is provided, the `HF_TOKEN` environment variable is created.

#### Example Deployment Snippet

```yaml
volumes:
  - name: model-storage
    emptyDir: {}
containers:
  - name: vllm
    env:
      - name: HF_HOME
        value: /model-cache
      - name: HF_TOKEN
        valueFrom:
          secretKeyRef:
            name: hf-secret
            key: HF_TOKEN
    volumeMounts:
      - mountPath: /model-cache
        name: model-storage
```

#### Template variables 

Various template variables are exposed as a result of using the `"hf://"` prefix, namely

- `{{ .HFModelName }}`: this is `<repo-id>/<model-id>` in the URI, which might be useful for vLLM arguments. Note that this is different from `{{ .ModelName }}`, which is the `spec.routing.modelName`, used for client requests 
- `{{ .MountedModelPath }}`: this is equal to `/model-cache`

### 2. Loading a model directly from a PVC

Downloading large models from Hugging Face can take a significant amount of time. If a PVC containing the model files is already pre-populated, then mounting this path and supplying that to vLLM can drastically shorten the engine's warm up time. 

#### URI format 

`"pvc://<pvc-name>/<path/to/model>"`

Example: `"pvc://granite-pvc/path/to/granite"`

#### Behavior 

- A read-only PVC volume with the name `model-storage` is created for the deployment 
- A read-only `volumeMount` with the `mountPath: model-cache` is created for each container where `mountModelVolume: true`


#### Example Deployment Snippet

```yaml
volumes:
  - name: model-storage
    persistentVolumeClaim:
      claimName: granite-pvc
      readOnly: true
containers:
  - name: vllm
    volumeMounts:
      - mountPath: /model-cache
        name: model-storage
```

#### Template variables

Various template variable are exposed as a result of using the `"pvc://"` prefix, with `.MountedModelPath` being particularly useful if vLLM arguments require it.

- `{{ .MountedModelPath }}`: this is equal to `/model-cache/<path/to/model>` where `</path/to/model>` comes from the URI. In the above example, `{{ .MountedModelPath }}` interpolates to `/model-cache/path/to/granite`

### 3. Loading the model as OCI arifacts using an image volume

Model artifacts can be built into images and consumed by Kubernetes volumes. This is called [image volume](https://kubernetes.io/docs/tasks/configure-pod-container/image-volumes/), and is in beta state as of Kubernetes v1.33. If the cluster has `ImageVolume` enabled as a feature gate, then model owners can mount models from OCI images. 

#### URI Format 

`"oci+native://<image-with-tag>::<path/to/model>"`

Example: `"oci+native://redhat/granite-7b-lab-gguf:1.0::/"`

(This OCI image comes from https://hub.docker.com/r/redhat/granite-7b-lab-gguf)

#### Additional Fields 

- `pullPolicy` (one of: `IfNotPresent`, `Always`, and `Never`): the [pull policy](https://kubernetes.io/docs/concepts/containers/images/#image-pull-policy) of the OCI image. If specified, this is the pull policy supplied to the `volume.image.pullPolicy`

#### Behavior 

- A image volume with the name `model-storage` is created for the deployment. The reference to the image is `<image-with-tag>`
- A read-only `volumeMount` with the `mountPath: model-cache` is created for each container where `mountModelVolume: true`


#### Example Deployment Snippet

```yaml
volumes:
  - name: model-storage
    image:
      reference: redhat/granite-7b-lab-gguf:1.0
containers:
  - name: vllm
    volumeMounts:
      - mountPath: /model-cache
        name: model-storage
        readOnly: true 
```

#### Template variables

Various template variable are exposed as a result of using the `"oci+native://"` prefix, with `.MountedModelPath` being particularly useful if vLLM arguments require it.

- `{{ .MountedModelPath }}`: this is equal to `/model-cache/<path/to/model>` where `</path/to/model>` comes from the URI. In the above example, `{{ .MountedModelPath }}` interpolates to `/model-cache` because `<path/to/model> = "/"`