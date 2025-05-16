package controller

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/llm-d/llm-d-model-service/api/v1alpha1"
	msv1alpha1 "github.com/llm-d/llm-d-model-service/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const modelStorageVolumeName = "model-storage"
const modelStorageRoot = "/cache"
const pathSep = "/"
const DECODE_ROLE = "decode"
const PREFILL_ROLE = "prefill"
const MODEL_ARTIFACT_URI_PVC = "pvc"
const MODEL_ARTIFACT_URI_HF = "hf"
const MODEL_ARTIFACT_URI_OCI = "oci"
const MODEL_ARTIFACT_URI_PVC_PREFIX = MODEL_ARTIFACT_URI_PVC + "://"
const MODEL_ARTIFACT_URI_HF_PREFIX = MODEL_ARTIFACT_URI_HF + "://"
const MODEL_ARTIFACT_URI_OCI_PREFIX = MODEL_ARTIFACT_URI_OCI + "://"
const ENV_HF_TOKEN = "HF_TOKEN"

type URIType string

const (
	PVC        URIType = "pvc"
	HF         URIType = "hf"
	OCI        URIType = "oci"
	UnknownURI URIType = "unknown"
)

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

// eppServiceAccountName returns the name of the eppServiceAccountName object
// defaults it to "epp-sa"
func eppRolebindingName(modelService *msv1alpha1.ModelService) string {
	sanitizedName, err := sanitizeName(modelService.Name + "-epp-rolebinding")
	if err != nil {
		return "epp-rolebinding"
	}
	return sanitizedName
}

// infModelName returns the name of the inference model object
func infModelName(modelService *msv1alpha1.ModelService) string {
	return modelService.Name
}

// mountedModelPath returns the mounted model path for the specific URI type
func mountedModelPath(modelService *msv1alpha1.ModelService) (string, error) {
	var err error
	mountedModelPath := ""
	uri := modelService.Spec.ModelArtifacts.URI
	switch UriType(uri) {
	case PVC:
		if parts, err := parsePVCURI(&modelService.Spec.ModelArtifacts); err == nil {
			modelPath := strings.Join(parts[1:], pathSep)
			// if uri is pvc://pvc-name/path/to/model
			// output is /cache/path/to/model
			mountedModelPath = modelStorageRoot + pathSep + modelPath
		}
	// TODOs for HF and OIC
	// case HF:
	// case OCI:

	case UnknownURI:
		err = fmt.Errorf("unknown uri type, cannot compute the mountedModelPath")
	}

	return mountedModelPath, err
}

func isHFURI(uri string) bool {
	return strings.HasPrefix(uri, MODEL_ARTIFACT_URI_HF_PREFIX)
}

func isPVCURI(uri string) bool {
	return strings.HasPrefix(uri, MODEL_ARTIFACT_URI_PVC_PREFIX)
}

func isOCIURI(uri string) bool {
	return strings.HasPrefix(uri, MODEL_ARTIFACT_URI_OCI_PREFIX)
}

// UriType returns the type of URI
func UriType(uri string) URIType {
	if isHFURI(uri) {
		return HF
	}

	if isPVCURI(uri) {
		return PVC
	}

	if isOCIURI(uri) {
		return OCI
	}

	return UnknownURI
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

// getVolumeMountForContainer returns a VolumeMount for a container where MountModelVolume: true
func getVolumeMountsForContainer(ctx context.Context, msvc *msv1alpha1.ModelService) []corev1.VolumeMount {

	volumeMounts := []corev1.VolumeMount{}
	var desiredVolumeMount *corev1.VolumeMount
	uriType := UriType(msvc.Spec.ModelArtifacts.URI)

	switch uriType {
	case PVC:
		// don't need the parts
		if _, err := parsePVCURI(&msvc.Spec.ModelArtifacts); err == nil {
			desiredVolumeMount = &corev1.VolumeMount{
				Name:      modelStorageVolumeName,
				MountPath: modelStorageRoot,
				ReadOnly:  true,
			}
		} else {
			log.FromContext(ctx).V(1).Error(err, "uri type: "+msvc.Spec.ModelArtifacts.URI)
		}
	// TODO for these
	// case HF:
	// case OCI:
	case UnknownURI:
		// do nothing
		log.FromContext(ctx).V(1).Error(fmt.Errorf("uri type is unknown, cannot populate volume mounts"), "uri type: "+msvc.Spec.ModelArtifacts.URI)
	}

	if desiredVolumeMount != nil {
		volumeMounts = append(volumeMounts, *desiredVolumeMount)
	}

	return volumeMounts
}

// getVolumeForPDDeployment returns a Volume for ModelArtifacts.URI
func getVolumeForPDDeployment(ctx context.Context, msvc *msv1alpha1.ModelService) []corev1.Volume {

	volumes := []corev1.Volume{}
	var desiredVolume *corev1.Volume
	uriType := UriType(msvc.Spec.ModelArtifacts.URI)

	switch uriType {
	case PVC:
		if parts, err := parsePVCURI(&msvc.Spec.ModelArtifacts); err == nil {
			pvcName := parts[0]
			desiredVolume = &corev1.Volume{
				Name: modelStorageVolumeName,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: pvcName,
						ReadOnly:  true,
					},
				},
			}
		} else {
			log.FromContext(ctx).V(1).Error(err, "uri type: "+msvc.Spec.ModelArtifacts.URI)
		}
	// TODO for these
	// case HF:
	// case OCI:
	case UnknownURI:
		// do nothing
		log.FromContext(ctx).V(1).Error(fmt.Errorf("uri type is unknown, cannot populate volumes"), "uri type: "+msvc.Spec.ModelArtifacts.URI)
	}

	if desiredVolume != nil {
		volumes = append(volumes, *desiredVolume)
	}

	return volumes
}

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

// ConvertToContainerSlice converts []Containers to []corev1.Container
// Note we lose information about MountModelVolume, ok for EndpointPicker container
// but not ok for PDSpec containers
func ConvertToContainerSlice(c []msv1alpha1.ContainerSpec) []corev1.Container {

	containerSlice := make([]corev1.Container, len(c))

	for i, containerSpec := range c {
		containerSlice[i] = corev1.Container{
			Name:      containerSpec.Name,
			Command:   containerSpec.Command,
			Args:      containerSpec.Args,
			Env:       containerSpec.Env,
			EnvFrom:   containerSpec.EnvFrom,
			Resources: containerSpec.Resources,
		}

		if containerSpec.Image != nil {
			containerSlice[i].Image = *containerSpec.Image
		}
	}

	return containerSlice
}

// ConvertToContainerSliceWithVolumeMount converts []Containers to []corev1.Container
// c is the targeted container slice (can be initContainer or Container)
// msvc is the msvc so we can get the URI and populate the relevant volumeMount
func ConvertToContainerSliceWithVolumeMount(ctx context.Context, c []v1alpha1.ContainerSpec, msvc *msv1alpha1.ModelService) []corev1.Container {

	containerSlice := ConvertToContainerSlice(c)
	for i := range c {
		if c[i].MountModelVolume {
			containerSlice[i].VolumeMounts = getVolumeMountsForContainer(ctx, msvc)
		}
	}

	return containerSlice
}
