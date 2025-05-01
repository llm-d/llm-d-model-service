/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"fmt"
	"regexp"
	"strings"

	msv1alpha1 "github.com/neuralmagic/llm-d-model-service/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
)

const modelStorageVolumeName = "model-storage"
const modelStorageRoot = "/cache"
const pathSep = "/"
const DECODE_ROLE = "decode"
const PREFILL_ROLE = "prefill"
const MODEL_ARTIFACT_URI_PVC = "pvc"
const MODEL_ARTIFACT_URI_HF = "hf"
const MODEL_ARTIFACT_URI_PVC_PREFIX = MODEL_ARTIFACT_URI_PVC + "://"
const MODEL_ARTIFACT_URI_HF_PREFIX = MODEL_ARTIFACT_URI_HF + "://"
const ENV_HF_TOKEN = "HF_TOKEN"

// deploymentName returns the name that should be used for a deployment object
func deploymentName(modelService *msv1alpha1.ModelService, role string) string {
	return modelService.Name + "-" + role
}

// infPoolName returns the name of the inference pool object
func infPoolName(modelService *msv1alpha1.ModelService) string {
	return modelService.Name
}

// eppDeploymentName returns the name of the epp deployment object
func eppDeploymentName(modelService *msv1alpha1.ModelService) string {
	return modelService.Name + "-epp"
}

// eppServiceName returns the name of the epp service object
func eppServiceName(modelService *msv1alpha1.ModelService) string {
	return modelService.Name + "-epp-service"
}

// infModelName returns the name of the inference model object
func infModelName(modelService *msv1alpha1.ModelService) string {
	return modelService.Name
}

func isHFURI(uri string) bool {
	return strings.HasPrefix(uri, MODEL_ARTIFACT_URI_HF_PREFIX)
}

func isPVCURI(uri string) bool {
	return strings.HasPrefix(uri, MODEL_ARTIFACT_URI_PVC_PREFIX)
}

// parsePVCURI returns parts from a valid pvc URI, or
// returns an error if the PVC URI is invalid
func parsePVCURI(modelArtifact *msv1alpha1.ModelArtifacts) ([]string, error) {
	if modelArtifact == nil {
		return nil, fmt.Errorf("modelArtifact is nil")
	}

	uri := modelArtifact.URI
	if !isPVCURI(uri) {
		return nil, fmt.Errorf("URI does not have pvc prefix: %s", uri)
	}

	parts := strings.Split(strings.TrimPrefix(uri, MODEL_ARTIFACT_URI_PVC_PREFIX), pathSep)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid pvc URI format: %s; need pvc://<pvc-name>/model/path", uri)
	}

	return parts, nil
}

// getVolumeMountFromModelArtifacts returns a VolumeMount for a URI of the form pvc://...
func getVolumeMountFromModelArtifacts(modelArtifact *msv1alpha1.ModelArtifacts) (*corev1.VolumeMount, error) {
	_, err := parsePVCURI(modelArtifact)
	if err != nil {
		return nil, err
	}

	return &corev1.VolumeMount{
		Name:      modelStorageVolumeName,
		MountPath: modelStorageRoot,
		ReadOnly:  true,
	}, nil
}

// getVolumeFromModelArtifacts returns a Volume for a URI of the form pvc://...
func getVolumeFromModelArtifacts(modelArtifact *msv1alpha1.ModelArtifacts) (*corev1.Volume, error) {
	parts, err := parsePVCURI(modelArtifact)
	if err != nil {
		return nil, err
	}

	pvcName := parts[0]

	return &corev1.Volume{
		Name: modelStorageVolumeName,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvcName,
				ReadOnly:  true,
			},
		},
	}, nil
}

/*
// getVLLMContainer returns the vllmContainer with volumeMount populated
func getVLLMContainer(modelArtifact *msv1alpha1.ModelArtifacts, pdSpec *msv1alpha1.PDSpec) (*corev1.Container, error) {
	vllmContainer := &corev1.Container{}

	// TODO handle modelService.Spec.Decode.Parallelism

	// If ModelArtifcat.URI is hf:// add an environment variable and a secret
	if isHFURI(modelArtifact.URI) {
		configureContainerForHF(modelArtifact, vllmContainer)
	}

	if isPVCURI(modelArtifact.URI) {
		// add volume from modelService.Spec.ModelArtifact
		volumeMount, err := getVolumeMountFromModelArtifacts(modelArtifact)
		if err != nil {
			return nil, err
		}
		vllmContainer.VolumeMounts = append(vllmContainer.VolumeMounts, *volumeMount)
	}
	return vllmContainer, nil
}
*/

// SanitizeModelName converts an routing.ModelNAme into a valid Kubernetes label value
func SanitizeModelName(s string) (string, error) {
	// Convert to lower case and trim spaces
	s = strings.ToLower(strings.TrimSpace(s))

	// Replace any disallowed characters with `-`
	re := regexp.MustCompile(`[^a-z0-9_.-]`)
	s = re.ReplaceAllString(s, "-")

	// Trim leading/trailing non-alphanumerics
	s = strings.Trim(s, "-._")

	// Enforce length limit
	if len(s) > 63 {
		s = s[:63]
	}

	// Final check
	if len(validation.IsValidLabelValue(s)) > 0 {
		return "", fmt.Errorf("cannot sanitize modelName into a valid label for deployment")
	}

	return s, nil
}
