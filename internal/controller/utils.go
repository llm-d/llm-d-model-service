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
	"context"
	"fmt"
	"regexp"
	"strings"

	msv1alpha1 "github.com/neuralmagic/llm-d-model-service/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	giev1alpha2 "sigs.k8s.io/gateway-api-inference-extension/api/v1alpha2"
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
	sanitizedName, err := sanitizeName(modelService.Name + "-" + role)
	if err != nil {
		return "deployment-" + role
	}
	return sanitizedName
}

// infPoolName returns the name of the inference pool object
func infPoolName(modelService *msv1alpha1.ModelService) string {
	sanitizedName, err := sanitizeName(modelService.Name + "-inference-pool")
	if err != nil {
		return "inference-pool"
	}

	return sanitizedName
}

// eppDeploymentName returns the name of the epp deployment object
func eppDeploymentName(modelService *msv1alpha1.ModelService) string {
	sanitizedName, err := sanitizeName(modelService.Name + "-epp")
	if err != nil {
		return "epp-deployment"
	}

	return sanitizedName
}

// eppServiceName returns the name of the epp service object
func eppServiceName(modelService *msv1alpha1.ModelService) string {
	sanitizedName, err := sanitizeName(modelService.Name + "-epp-service")
	if err != nil {
		return "epp-service"
	}
	return sanitizedName
}

// pdServiceAccountName returns the name of the inference pool object
func pdServiceAccountName(modelService *msv1alpha1.ModelService) string {
	sanitizedName, err := sanitizeName(modelService.Name + "-sa")
	if err != nil {
		return "pd-sa"
	}
	return sanitizedName
}

// eppServiceAccountName returns the name of the eppServiceAccountName object
// defaults it to "epp-sa"
func eppServiceAccountName(modelService *msv1alpha1.ModelService) string {
	sanitizedName, err := sanitizeName(modelService.Name + "-epp-sa")
	if err != nil {
		return "epp-sa"
	}
	return sanitizedName
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

// sanitizeName converts an routing.ModelNAme into a valid Kubernetes label value
func sanitizeName(s string) (string, error) {
	// Convert to lower case and trim spaces
	s = strings.ToLower(strings.TrimSpace(s))

	// Replace any disallowed characters with `-`
	re := regexp.MustCompile(`[^a-z0-9-]+`)
	s = re.ReplaceAllString(s, "-")

	// Trim leading/trailing non-alphanumerics
	s = strings.Trim(s, "-._")

	// Enforce length limit
	if len(s) > 63 {
		s = s[:63]
	}

	// Final check
	if len(validation.IsValidLabelValue(s)) > 0 {
		return "", fmt.Errorf("cannot sanitize into a valid DNS compliant name")
	}

	return s, nil
}

func populateStatus(ctx context.Context, ms *msv1alpha1.ModelService, k8sClient client.Client,
	prefillDeployment, decodeDeployment, eppDeployment *appsv1.Deployment,
	inferenceModel *giev1alpha2.InferenceModel,
	inferencePool *giev1alpha2.InferencePool) {
	var conditions []metav1.Condition

	if inferenceModel != nil {
		ms.Status.InferenceModelRef = &inferenceModel.Name
	}

	if inferencePool != nil {
		ms.Status.InferencePoolRef = &inferencePool.Name
	}
	if prefillDeployment != nil {
		ms.Status.PrefillDeploymentRef = &prefillDeployment.Name
		prefillDeploymentFromCluster := &appsv1.Deployment{}
		// Mirror conditions with "Prefill" prefix
		err := k8sClient.Get(ctx, client.ObjectKey{Name: prefillDeployment.Name, Namespace: prefillDeployment.Namespace}, prefillDeploymentFromCluster)
		if err != nil {
			log.FromContext(ctx).Error(err, "unable to get prefill deployment")
		}
		for _, c := range prefillDeploymentFromCluster.Status.Conditions {
			conditions = append(conditions, metav1.Condition{
				Type:               "Prefill" + string(c.Type),
				Status:             metav1.ConditionStatus(c.Status),
				Reason:             c.Reason,
				Message:            c.Message,
				LastTransitionTime: c.LastUpdateTime,
			})
		}
	}

	if decodeDeployment != nil {
		ms.Status.DecodeDeploymentRef = &decodeDeployment.Name
		decodeDeploymentFromCluster := &appsv1.Deployment{}
		err := k8sClient.Get(ctx, client.ObjectKey{Name: decodeDeployment.Name, Namespace: decodeDeployment.Namespace}, decodeDeploymentFromCluster)
		if err != nil {
			log.FromContext(ctx).Error(err, "unable to get prefill deployment")
		}
		// Mirror conditions with "Decode" prefix
		for _, c := range decodeDeploymentFromCluster.Status.Conditions {
			conditions = append(conditions, metav1.Condition{
				Type:               "Decode" + string(c.Type),
				Status:             metav1.ConditionStatus(c.Status),
				Reason:             c.Reason,
				Message:            c.Message,
				LastTransitionTime: c.LastUpdateTime,
			})
		}
	}

	if eppDeployment != nil {
		ms.Status.DecodeDeploymentRef = &eppDeployment.Name
		eppDeploymentFromCluster := &appsv1.Deployment{}
		err := k8sClient.Get(ctx, client.ObjectKey{Name: eppDeployment.Name, Namespace: eppDeployment.Namespace}, eppDeploymentFromCluster)
		if err != nil {
			log.FromContext(ctx).Error(err, "unable to get prefill deployment")
		}
		// Mirror conditions with "Decode" prefix
		for _, c := range eppDeploymentFromCluster.Status.Conditions {
			conditions = append(conditions, metav1.Condition{
				Type:               "Epp" + string(c.Type),
				Status:             metav1.ConditionStatus(c.Status),
				Reason:             c.Reason,
				Message:            c.Message,
				LastTransitionTime: c.LastUpdateTime,
			})
		}
	}

	ms.Status.Conditions = conditions
}
