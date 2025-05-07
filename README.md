# ModelService

> The *ModelService* declaratively provisions and maintains the Kubernetes resources needed to serve a base model for inference.

A *ModelService* custom resource encapsulates the desired state of deployments and routing associated with a single base model. It automates the management of Kubernetes resources, including:

* Prefill and decode deployments

* Inference pool and model defined by [Gateway API Inference Extension](https://gateway-api-inference-extension.sigs.k8s.io)

* Endpoint picker (EPP) deployment and service

* Relevant RBAC permissions

A *ModelService* may optionally reference a **BaseConfig** — a Kubernetes ConfigMap that defines reusable, platform-managed presets for shared behavior across multiple base models.

Typically, platform operators define a small set of *BaseConfig* presets, and base model owners reference them in their respective *ModelService* resources.

The *ModelService* controller reconciles the cluster state to align with the configuration declared in the *ModelService* custom resource. This custom resource is the source of truth for resources it owns.

> ⚠️ Important: Do not manually modify resources owned by a *ModelService*. If your use case is not yet supported, please file an issue in the *ModelService* repository.

## Features

✅ Supports disaggregated prefill and decode workloads

🌐 Integrates with Gateway API Inference Extension for request routing

📈 Enables auto-scaling via HPA or custom controllers

🔧 Allows independent scaling and node affinity for prefill and decode deployments

📦 Supports model loading from:

* OCI images

* HuggingFace (public or private)

* Kubernetes PVCs

🧩 Supports value templating in both BaseConfig and ModelService

## How It Works

When a ModelService is reconciled:

1. **Templating**: Template variables in *BaseConfig* and *ModelService* are interpolated based on the *ModelService* spec.

2. **Merging**: A semantic merge overlays *ModelService* values on top of the selected *BaseConfig*.

3. **Resource Deployment**: The controller creates or updates the following:

* Inference deployments (prefill and decode)

* Routing resources (e.g., EPP deployment)

* RBAC authorizations

The result is a fully managed inference stack for the base model.

![model-service-arch](model-service-arch.png)

## Best Practices

* Use *BaseConfig* to capture platform-level defaults and shared configurations across multiple base models.

* Use *ModelService* to define behavior specific to a given base model, and override *BaseConfig* values only when necessary.

* Platform teams should install *Baseconfig* presets using the `llm-d` deployer.

* Base model owners should prefer using these presets to streamline onboarding of base models, rather than creating their own custom *BaseConfigs*.

## Docs

### [Samples](docs/samples.md)

### [User Guide](docs/userguide.md)

### [API Reference](docs/apireference.md)

### [Developer](docs/developer.md)
