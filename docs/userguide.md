# User Guide

This guide presents the core concepts and configuration patterns for serving base models using the `ModelService` Custom Resource Definition (CRD). It is intended for both platform operators and model owners.

## [Core Concepts](userguide/core-concepts.md)

Understand how `ModelService` fits into the Kubernetes ecosystem, what resources it manages, and how its declarative workflow simplifies inference infrastructure.

---

## Topics

1. **[Model Name](userguide/model-name.md)**
   How inference clients refer to your model using OpenAI-compatible APIs.

2. **[Model Artifacts](userguide/model-artifacts.md)**
   Load models from Hugging Face, PVCs, or OCI images and mount them into serving containers.

3. **[Templating Reference](userguide/templating-reference.md)**
   Use Go templates in `ModelService` and `BaseConfig` to dynamically generate configurations for child resources.

4. **[Decouple Scaling](userguide/decouple-scaling.md)**
   Let HPA or custom controllers manage replica counts for prefill and decode deployments.

5. **[Accelerator Types](userguide/accelerator-types.md)**
   Target specific GPU types using node labels to ensure models run on the right hardware.

6. **[Semantic Merge](userguide/semantic-merge.md)**
   Learn how values in `ModelService` override or augment those defined in `BaseConfig`.

7. **[Child Resources](userguide/resources-owned.md)**
   Explore all Kubernetes resources owned and managed by a `ModelService`.

---

For more details, see:

📄 [Install Guide](install.md) — how to install the ModelService controller

📘 [API Reference](apireference.md) — full CRD schema and field definitions
