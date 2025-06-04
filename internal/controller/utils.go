package controller

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	msv1alpha1 "github.com/llm-d/llm-d-model-service/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// deploymentName returns the name that should be used for a deployment object
func deploymentName(modelService *msv1alpha1.ModelService, role string) string {
	sanitizedName, err := sanitizeName(modelService.Name + "-" + role)
	if err != nil {
		return "deployment-" + role
	}
	return sanitizedName
}

// httpRouteName returns the name of the inference pool object
func httpRouteName(modelService *msv1alpha1.ModelService) string {
	sanitizedName, err := sanitizeName(modelService.Name + "-http-route")
	if err != nil {
		return "http-route"
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
		if _, modelPathSlice, err := parsePVCURI(&modelService.Spec.ModelArtifacts); err == nil {
			modelPath := strings.Join(modelPathSlice, pathSep)
			// if uri is pvc://pvc-name/path/to/model
			// output is /model-cache/path/to/model
			mountedModelPath = modelStorageRoot + pathSep + modelPath
		}

	case HF:
		// The mountModelPath for HF is just the storage root, ie. model-cache
		mountedModelPath = modelStorageRoot

	// The mountModelPath for OCI is what comes after :: in the URI
	case OCI:
		if _, modelPathSlice, err := parseOCIURI(&modelService.Spec.ModelArtifacts); err == nil {
			// if uri is oci+native://image-with-tag:0.0.1::path/to/model
			// output is /model-cache/path/to/model
			joinedModelPath := strings.Join(modelPathSlice, pathSep)
			mountedModelPath = modelStorageRoot + pathSep + joinedModelPath
		}

	case UnknownURI:
		err = fmt.Errorf("unknown uri type, cannot compute the mountedModelPath")
	}

	return mountedModelPath, err
}

// isHFURI returns True if the URI begins with hf://
func isHFURI(uri string) bool {
	return strings.HasPrefix(uri, MODEL_ARTIFACT_URI_HF_PREFIX)
}

// isPVCURI returns True if the URI begins with pvc://
func isPVCURI(uri string) bool {
	return strings.HasPrefix(uri, MODEL_ARTIFACT_URI_PVC_PREFIX)
}

// isOCIURI returns True if the URI begins with oci+native://
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

// parsePVCURI returns parts from a valid pvc URI
// Returns:
// first string is the pvc name
// second string slice is the path to the model
// error if uri format is invalid
func parsePVCURI(modelArtifact *msv1alpha1.ModelArtifacts) (string, []string, error) {
	var pvcName string
	modelPath := []string{}
	if modelArtifact == nil {
		return pvcName, modelPath, fmt.Errorf("modelArtifact is nil")
	}

	uri := modelArtifact.URI
	if !isPVCURI(uri) {
		return pvcName, modelPath, fmt.Errorf("URI does not have pvc prefix: %s", uri)
	}

	parts := strings.Split(strings.TrimPrefix(uri, MODEL_ARTIFACT_URI_PVC_PREFIX), pathSep)
	if len(parts) < 2 {
		return pvcName, modelPath, fmt.Errorf("invalid pvc URI format: %s; need pvc://<pvc-name>/model/path", uri)
	}

	pvcName = parts[0]
	modelPath = parts[1:]

	return pvcName, modelPath, nil
}

// parseHFURI returns parts from a valid hf URI, or
// returns an error if the HF URI is invalid
// returns two strings:
// First string is the repo-id
// Second string is the model-id
func parseHFURI(modelArtifact *msv1alpha1.ModelArtifacts) (string, string, error) {
	var repoID string
	var modelID string
	if modelArtifact == nil {
		return repoID, modelID, fmt.Errorf("modelArtifact is nil")
	}

	uri := modelArtifact.URI
	if !isHFURI(uri) {
		return repoID, modelID, fmt.Errorf("URI does not have hf prefix: %s", uri)
	}

	parts := strings.Split(strings.TrimPrefix(uri, MODEL_ARTIFACT_URI_HF_PREFIX), pathSep)
	if len(parts) != 2 {
		return repoID, modelID, fmt.Errorf("invalid hf URI format: %s; need hf://<repo-id>/<model-id>", uri)
	}

	return parts[0], parts[1], nil
}

// parseOCIURI returns parts from a valid oci URI, or
// returns an error if the OCI URI is invalid
// returns two strings
// First string is the complete image identifier including tag
// Second string is the path to the model
func parseOCIURI(modelArtifact *msv1alpha1.ModelArtifacts) (string, []string, error) {
	var imageIdentifier string
	pathToModel := []string{}
	if modelArtifact == nil {
		return imageIdentifier, pathToModel, fmt.Errorf("modelArtifact is nil")
	}

	uri := modelArtifact.URI
	if !isOCIURI(uri) {
		return imageIdentifier, pathToModel, fmt.Errorf("URI does not have oci prefix: %s", uri)
	}

	// Split by ::
	parts := strings.Split(strings.TrimPrefix(uri, MODEL_ARTIFACT_URI_OCI_PREFIX), ociPathToModelSep)
	if len(parts) != 2 {
		return imageIdentifier, pathToModel, fmt.Errorf("invalid oci URI format: %s; need oci+native://<image identifier with tag>::/path/to/model. Please double check if you are missing %s which did not result in exactly two segments", uri, ociPathToModelSep)
	}

	imageIdentifier = parts[0]
	// Split by /
	pathToModel = strings.Split(strings.TrimPrefix(parts[1], pathSep), pathSep)

	return imageIdentifier, pathToModel, nil
}

// getVolumeMountForContainer returns a VolumeMount for a container where MountModelVolume: true
func getVolumeMountsForContainer(ctx context.Context, msvc *msv1alpha1.ModelService) []corev1.VolumeMount {

	volumeMounts := []corev1.VolumeMount{}
	var desiredVolumeMount *corev1.VolumeMount
	uriType := UriType(msvc.Spec.ModelArtifacts.URI)

	switch uriType {

	// The volume mount for PVC and OCI is the same
	// volumeMounts:
	// - name: model-storage
	//   mountPath: /model-cache
	//   readOnly: true
	case PVC, OCI:
		desiredVolumeMount = &corev1.VolumeMount{
			Name:      modelStorageVolumeName,
			MountPath: modelStorageRoot,
			ReadOnly:  true,
		}
	// Volume mount is the same, except that HF volume is not readOnly
	// so that models can be downloaded into the mountPath
	case HF:
		desiredVolumeMount = &corev1.VolumeMount{
			Name:      modelStorageVolumeName,
			MountPath: modelStorageRoot,
		}
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
	// Return a volume with persistentVolumeClaim
	case PVC:
		if pvcName, _, err := parsePVCURI(&msvc.Spec.ModelArtifacts); err != nil {
			log.FromContext(ctx).V(1).Error(err, "uri: "+msvc.Spec.ModelArtifacts.URI)
		} else {
			desiredVolume = &corev1.Volume{
				Name: modelStorageVolumeName,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: pvcName,
						ReadOnly:  true,
					},
				},
			}
		}
	// Return an emptyDir volume with ModelArtifacts.Size
	case HF:
		if _, _, err := parseHFURI(&msvc.Spec.ModelArtifacts); err != nil {
			log.FromContext(ctx).V(1).Error(err, "uri: "+msvc.Spec.ModelArtifacts.URI)
		} else {
			desiredVolume = &corev1.Volume{
				Name: modelStorageVolumeName,
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						SizeLimit: msvc.Spec.ModelArtifacts.Size,
					},
				},
			}
		}

	// Return a volume with image reference
	case OCI:
		if image, _, err := parseOCIURI(&msvc.Spec.ModelArtifacts); err != nil {
			log.FromContext(ctx).V(1).Error(err, "uri: "+msvc.Spec.ModelArtifacts.URI)
		} else {
			desiredVolume = &corev1.Volume{
				Name: modelStorageVolumeName,
				VolumeSource: corev1.VolumeSource{
					Image: &corev1.ImageVolumeSource{
						Reference:  image,
						PullPolicy: msvc.Spec.ModelArtifacts.PullPolicy,
					},
				},
			}
		}
	case UnknownURI:
		// do nothing
		log.FromContext(ctx).V(1).Error(fmt.Errorf("uri type is unknown, cannot populate volumes"), "uri type: "+msvc.Spec.ModelArtifacts.URI)
	}

	if desiredVolume != nil {
		volumes = append(volumes, *desiredVolume)
	}

	return volumes
}

// getEnvsForContainer returns the desired list of env vars for the container for the given URI type
// For hf URIs, it returns an EnvVar which has a reference to a secretKey, provided by ModelArtifacts,
// with HF_TOKEN in that secret
// Other URI types do not need the controller to add any EnvVars
func getEnvsForContainer(ctx context.Context, msvc *msv1alpha1.ModelService) []corev1.EnvVar {
	envs := []corev1.EnvVar{}

	uriType := UriType(msvc.Spec.ModelArtifacts.URI)

	switch uriType {
	// only HF case needs a EnvVar, PVC and OCI don't
	case HF:
		// Add a HF_TOKEN env
		if msvc.Spec.ModelArtifacts.AuthSecretName != nil {
			hfTokenEnv := corev1.EnvVar{
				Name: ENV_HF_TOKEN,
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: *msvc.Spec.ModelArtifacts.AuthSecretName,
						},
						Key: ENV_HF_TOKEN,
					},
				},
			}
			envs = append(envs, hfTokenEnv)
		}

		// Add a HF_HOME env which points to the mountPath
		if mountedModelPath, err := mountedModelPath(msvc); err == nil {
			hfHomeEnv := corev1.EnvVar{
				Name:  ENV_HF_HOME,
				Value: mountedModelPath,
			}
			envs = append(envs, hfHomeEnv)
		} else {
			log.FromContext(ctx).V(1).Error(err, "cannot parse hf uri to get the mountedModelPath")
		}

	case UnknownURI:
		// do nothing
		log.FromContext(ctx).V(1).Error(fmt.Errorf("uri type is unknown, cannot populate volumes"), "uri type: "+msvc.Spec.ModelArtifacts.URI)
	}

	return envs
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

// convertToContainerSlice converts []Containers to []corev1.Container
// Note we lose information about MountModelVolume, ok for EndpointPicker container
// but not ok for PDSpec containers
func convertToContainerSlice(c []msv1alpha1.ContainerSpec) []corev1.Container {

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

// convertToContainerSliceWithURIInfo converts []Containers to []corev1.Container
// with the relevant volumeMount and env for containers where mountModelPath is true
// c is the targeted container slice (can be initContainer or Container)
// msvc is the msvc so we can get the URI and populate the relevant volumeMount and env
func convertToContainerSliceWithURIInfo(ctx context.Context, c []msv1alpha1.ContainerSpec, msvc *msv1alpha1.ModelService) []corev1.Container {

	containerSlice := convertToContainerSlice(c)
	for i := range c {
		if c[i].MountModelVolume {
			containerSlice[i].Env = getEnvsForContainer(ctx, msvc)
			containerSlice[i].VolumeMounts = getVolumeMountsForContainer(ctx, msvc)
		}
	}

	return containerSlice
}
