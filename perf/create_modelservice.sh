#!/bin/bash

# Base name for the ModelService
BASE_NAME="facebook-opt-125m-nixl"

echo "Please make sure universal base config is applied to the cluster"

# Loop to create and apply 100 instances
for i in $(seq 1 1000); do
  NAME="${BASE_NAME}-${i}"
  cat <<EOF | kubectl apply -f -
apiVersion: llm-d.ai/v1alpha1
kind: ModelService
metadata:
  name: ${NAME}
spec:
  decoupleScaling: false

  baseConfigMapRef:
    name: universal-base-config

  routing: 
    modelName: facebook/opt-125m
    ports:
    - name: app_port
      port: 8000
    - name: internal_port
      port: 8200

  modelArtifacts:
    uri: hf://facebook/opt-125m

  decode:
    replicas: 1
    acceleratorTypes:
      labelKey: nvidia.com/gpu.product
      labelValues:
        - NVIDIA-A100-SXM4-80GB
    containers:
    - name: "vllm"
      args:
        - "{{ .HFModelName }}"

  prefill:
    replicas: 1
    acceleratorTypes:
      labelKey: nvidia.com/gpu.product
      labelValues:
        - NVIDIA-A100-SXM4-80GB
    containers:
    - name: "vllm"
      args:
        - "{{ .HFModelName }}"
EOF
done
