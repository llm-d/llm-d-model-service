package controller

import (
	"context"
	"fmt"
	"strings"
	"text/template"

	"dario.cat/mergo"
	msv1alpha1 "github.com/llm-d/llm-d-model-service/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	giev1alpha2 "sigs.k8s.io/gateway-api-inference-extension/api/v1alpha2"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/yaml"
)

// TODO: Rename BaseConfig struct as ChildResources struct
// Idea is that everything in ChildResources is a child spawned by
// MSVC

// BaseConfig holds information read from the base configmap
type BaseConfig struct {
	ConfigMaps        []corev1.ConfigMap          `json:"configMaps,omitempty"`
	PrefillDeployment *appsv1.Deployment          `json:"prefillDeployment,omitempty"`
	DecodeDeployment  *appsv1.Deployment          `json:"decodeDeployment,omitempty"`
	PrefillService    *corev1.Service             `json:"prefillService,omitempty"`
	DecodeService     *corev1.Service             `json:"decodeService,omitempty"`
	HTTPRoute         *gatewayv1.HTTPRoute        `json:"httpRoute,omitempty"`
	InferencePool     *giev1alpha2.InferencePool  `json:"inferencePool,omitempty"`
	InferenceModel    *giev1alpha2.InferenceModel `json:"inferenceModel,omitempty"`
	EPPDeployment     *appsv1.Deployment          `json:"eppDeployment,omitempty"`
	EPPService        *corev1.Service             `json:"eppService,omitempty"`
	EPPServiceAccount *corev1.ServiceAccount      `json:"eppServiceAccount,omitempty"`
	PDServiceAccount  *corev1.ServiceAccount      `json:"pdServiceAccount,omitempty"`
	EPPRoleBinding    *rbacv1.RoleBinding         `json:"eppRoleBinding,omitempty"`
}

// shouldCreateConfigMaps returns True if there is at least one ConfigMap to be created
func (childResource *BaseConfig) shouldCreateConfigMaps() bool {
	return len(childResource.ConfigMaps) > 0
}

// shouldCreatePrefillDeployment returns True if the prefill deployment needs to be created
func (childResource *BaseConfig) shouldCreatePrefillDeployment() bool {
	return childResource.PrefillDeployment != nil
}

// shouldCreatePrefillService returns True if the prefill deployment needs to be created
func (childResource *BaseConfig) shouldCreatePrefillService() bool {
	return childResource.shouldCreatePrefillDeployment() && childResource.PrefillService != nil
}

// shouldCreateDecodeDeployment returns True if the decode deployment needs to be created
func (childResource *BaseConfig) shouldCreateDecodeDeployment() bool {
	return childResource.DecodeDeployment != nil
}

// shouldCreateDecodeService returns True if the decode deployment needs to be created
func (childResource *BaseConfig) shouldCreateDecodeService() bool {
	return childResource.shouldCreateDecodeDeployment() && childResource.DecodeService != nil
}

// shouldCreatePDServiceAccount returns True if either prefill or decode deployment needs to be created
func (childResource *BaseConfig) shouldCreatePDServiceAccount() bool {
	return childResource.shouldCreatePrefillDeployment() || childResource.shouldCreateDecodeDeployment()
}

// shouldCreateEPPDeployment returns True if the EPP deployment needs to be created
func (childResource *BaseConfig) shouldCreateEPPDeployment() bool {
	return childResource.EPPDeployment != nil
}

// shouldCreateEPPService returns True if the EPP deployment needs to be created
func (childResource *BaseConfig) shouldCreateEPPService() bool {
	return childResource.shouldCreateEPPDeployment() && childResource.EPPService != nil
}

// shouldCreateEPPServiceAccount returns True if EPP deployment needs to be created
func (childResource *BaseConfig) shouldCreateEPPServiceAccount() bool {
	return childResource.shouldCreateEPPDeployment() && childResource.EPPServiceAccount != nil
}

// createEPPServiceAccount returns True if EPP deployment needs to be created
func (childResource *BaseConfig) shouldCreateEPPRoleBinding() bool {
	return childResource.shouldCreateEPPDeployment() && childResource.EPPRoleBinding != nil
}

// shouldCreateHTTPRoute returns True if HTTPRoute needs to be created
// should be created if there's an InferencePool or if HTTPRoute is specified in the baseconfig
func (childResource *BaseConfig) shouldCreateHTTPRoute() bool {
	return childResource.HTTPRoute != nil || childResource.shouldCreateInferencePool()
}

// shouldCreateInferencePool returns True if InferencePool needs to be created
func (childResource *BaseConfig) shouldCreateInferencePool() bool {
	return childResource.InferencePool != nil
}

// shouldCreateInferenceModel returns True if InferenceModel needs to be created
func (childResource *BaseConfig) shouldCreateInferenceModel() bool {
	return childResource.InferenceModel != nil
}

// InterpolateBaseConfigMap data strings using msvc template variable values
func InterpolateBaseConfigMap(ctx context.Context, cm *corev1.ConfigMap, msvc *msv1alpha1.ModelService) (*corev1.ConfigMap, error) {
	values := &TemplateVars{}
	err := values.from(ctx, msvc)
	if err != nil {
		log.FromContext(ctx).V(1).Error(err, "cannot get template variable values from msvc")
		return nil, err
	}

	functions := &TemplateFuncs{funcMap: template.FuncMap{}}
	functions.from(ctx, msvc)

	// interpolate base config data
	interpolated := cm.DeepCopy()
	for key, tmplStr := range interpolated.Data {
		// render first time with the user-exposed values;
		// these values can be used to interpolate user-defined base config templates
		rendering, err := renderTemplate(tmplStr, values, functions)
		if err != nil {
			log.FromContext(ctx).V(1).Error(err, "cannot construct child resource")
			return nil, err
		}

		interpolated.Data[key] = rendering
	}

	return interpolated, nil
}

// interpolateContainerArgs interpolates (init) container args
func interpolateContainerArgs(ctx context.Context, containerSpec *msv1alpha1.ContainerSpec, values *TemplateVars, functions *TemplateFuncs) (*msv1alpha1.ContainerSpec, error) {
	containerCopy := containerSpec.DeepCopy()
	for j, argStr := range containerSpec.Args {
		renderedArg, err := renderTemplate(argStr, values, functions)
		if err != nil {
			log.FromContext(ctx).V(1).Error(err, "error with template rendering, cannot render "+argStr)
			return nil, err
		}
		containerCopy.Args[j] = renderedArg
	}
	return containerCopy, nil
}

// interpolateContainerArgsForPDSpec interpolates container args using template variables
func interpolateContainerArgsForPDSpec(ctx context.Context, msvc *msv1alpha1.ModelService, role string, values *TemplateVars, functions *TemplateFuncs) (*msv1alpha1.PDSpec, error) {
	// Get the desired pdSpec
	var pdSpec msv1alpha1.PDSpec
	if role == PREFILL_ROLE {
		pdSpec = *msvc.Spec.Prefill
	} else {
		pdSpec = *msvc.Spec.Decode
	}
	pdSpecCopy := pdSpec.DeepCopy()

	// Interpolate args in pdSpec.initContainers
	for i, initContainer := range pdSpec.InitContainers {
		interpolatedInitContainer, err := interpolateContainerArgs(ctx, &initContainer, values, functions)
		if err != nil {
			return nil, err
		}
		pdSpecCopy.InitContainers[i] = *interpolatedInitContainer
	}

	// Interpolate the args in pdSpec.Container
	for i, container := range pdSpec.Containers {
		interpolatedContainer, err := interpolateContainerArgs(ctx, &container, values, functions)
		if err != nil {
			return nil, err
		}
		pdSpecCopy.Containers[i] = *interpolatedContainer
	}

	return pdSpecCopy, nil
}

// InterpolateModelService interpolates strings using msvc template variable values
func InterpolateModelService(ctx context.Context, msvc *msv1alpha1.ModelService) (*msv1alpha1.ModelService, error) {
	values := &TemplateVars{}
	err := values.from(ctx, msvc)
	if err != nil {
		log.FromContext(ctx).V(1).Error(err, "cannot get template variable values from msvc")
		return nil, err
	}

	functions := &TemplateFuncs{funcMap: template.FuncMap{}}
	functions.from(ctx, msvc)

	// interpolate container args
	msvcCopy := msvc.DeepCopy()

	// interpolate prefill section
	if msvc.Spec.Prefill != nil {
		interpolatedPrefill, err := interpolateContainerArgsForPDSpec(ctx, msvcCopy, PREFILL_ROLE, values, functions)
		if err != nil {
			return nil, err
		}
		msvcCopy.Spec.Prefill = interpolatedPrefill
	}

	// interpolate decode section
	if msvc.Spec.Decode != nil {
		interpolatedDecode, err := interpolateContainerArgsForPDSpec(ctx, msvcCopy, DECODE_ROLE, values, functions)
		if err != nil {
			return nil, err
		}
		msvcCopy.Spec.Decode = interpolatedDecode
	}

	return msvcCopy, nil
}

func (r *ModelServiceReconciler) getChildResourcesFromConfigMap(
	ctx context.Context,
	msvc *msv1alpha1.ModelService,
) (*BaseConfig, error) {

	// no configmap ref; return nil
	if msvc.Spec.BaseConfigMapRef == nil {
		return &BaseConfig{}, nil
	}

	cmName := msvc.Spec.BaseConfigMapRef.Name
	cmNamespace := msvc.Spec.BaseConfigMapRef.Namespace
	// if namespace is not specified, make it the namespace of the msvc
	if strings.TrimSpace(cmNamespace) == "" {
		cmNamespace = msvc.Namespace
	}

	// get the configmap
	var cm corev1.ConfigMap
	err := r.Get(ctx, types.NamespacedName{Name: cmName, Namespace: cmNamespace}, &cm)
	if err != nil {
		return nil, fmt.Errorf("failed to get ConfigMap: %w", err)
	}

	interpolated, err := InterpolateBaseConfigMap(ctx, &cm, msvc)
	if err != nil {
		return nil, err
	}

	return BaseConfigFromCM(interpolated)
}

// BaseConfigFromCM returns a BaseConfig object if the input
// configmap is a valid serialization
func BaseConfigFromCM(cm *corev1.ConfigMap) (*BaseConfig, error) {
	// populate baseconfig struct
	bc := &BaseConfig{}

	// generic deserialize func to deserialize any of the baseconfig fields
	deserialize := func(key string, target interface{}) error {
		raw, ok := cm.Data[key]
		if !ok || strings.TrimSpace(raw) == "" {
			return nil
		}
		return yaml.Unmarshal([]byte(raw), target)
	}

	// Decode each field of the baseconfig
	// TODO: Don't return too early; consolidate errors
	if err := deserialize("configMaps", &bc.ConfigMaps); err != nil {
		return nil, fmt.Errorf("failed to decode configMaps: %w", err)
	}
	if err := deserialize("prefillDeployment", &bc.PrefillDeployment); err != nil {
		return nil, fmt.Errorf("failed to decode prefillDeployment: %w", err)
	}
	if err := deserialize("decodeDeployment", &bc.DecodeDeployment); err != nil {
		return nil, fmt.Errorf("failed to decode decodeDeployment: %w", err)
	}
	if err := deserialize("prefillService", &bc.PrefillService); err != nil {
		return nil, fmt.Errorf("failed to decode prefillService: %w", err)
	}
	if err := deserialize("decodeService", &bc.DecodeService); err != nil {
		return nil, fmt.Errorf("failed to decode decodeService: %w", err)
	}
	if err := deserialize("httpRoute", &bc.HTTPRoute); err != nil {
		return nil, fmt.Errorf("failed to decode httpRoute: %w", err)
	}
	if err := deserialize("inferencePool", &bc.InferencePool); err != nil {
		return nil, fmt.Errorf("failed to decode inferencePool: %w", err)
	}
	if err := deserialize("inferenceModel", &bc.InferenceModel); err != nil {
		return nil, fmt.Errorf("failed to decode inferenceModel: %w", err)
	}
	if err := deserialize("eppDeployment", &bc.EPPDeployment); err != nil {
		return nil, fmt.Errorf("failed to decode eppDeployment: %w", err)
	}
	if err := deserialize("eppService", &bc.EPPService); err != nil {
		return nil, fmt.Errorf("failed to decode eppService: %w", err)
	}

	return bc, nil
}

// MergeChildResources merges the MSVC resources into BaseConfig resources
// merging means MSVC controller is overwriting some fields, such as Name and Namespace for that resource
func (interpolatedBaseConfig *BaseConfig) MergeChildResources(ctx context.Context, modelService *msv1alpha1.ModelService, scheme *runtime.Scheme, rbacOptions *RBACOptions) *BaseConfig {
	log.FromContext(ctx).V(1).Info("attempting to update configmaps")
	// Step: update configmaps
	if interpolatedBaseConfig.ConfigMaps != nil {
		interpolatedBaseConfig.mergeConfigMaps(ctx, modelService, scheme)
	}

	log.FromContext(ctx).V(1).Info("attempting to update prefill deployment")
	// Step 3: update the child resources
	// Idea: updates do the mergo merge
	if modelService.Spec.Prefill != nil || interpolatedBaseConfig.PrefillDeployment != nil {
		interpolatedBaseConfig.mergePDDeployment(ctx, modelService, PREFILL_ROLE, scheme)
		if interpolatedBaseConfig.PrefillService != nil {
			interpolatedBaseConfig.mergePDService(ctx, modelService, PREFILL_ROLE, scheme)
		}
	}
	log.FromContext(ctx).V(1).Info("attempting to update decode deployment")
	if modelService.Spec.Decode != nil || interpolatedBaseConfig.DecodeDeployment != nil {
		interpolatedBaseConfig.mergePDDeployment(ctx, modelService, DECODE_ROLE, scheme)
		if interpolatedBaseConfig.DecodeService != nil {
			interpolatedBaseConfig.mergePDService(ctx, modelService, DECODE_ROLE, scheme)
		}
	}

	if interpolatedBaseConfig.PrefillDeployment != nil || interpolatedBaseConfig.DecodeDeployment != nil {
		// some pd pods are getting created; set SA and RB here
		interpolatedBaseConfig.setPDServiceAccount(ctx, modelService, scheme, rbacOptions)
	}

	if interpolatedBaseConfig.HTTPRoute != nil || interpolatedBaseConfig.InferencePool != nil {
		log.FromContext(ctx).V(1).Info("attempting to update HTTPRoute")
		interpolatedBaseConfig.mergeHTTPRoute(ctx, modelService, scheme)
	}

	if interpolatedBaseConfig.InferencePool != nil {
		log.FromContext(ctx).V(1).Info("attempting to update inference pool")
		interpolatedBaseConfig.mergeInferencePool(ctx, modelService, scheme)
	}

	if interpolatedBaseConfig.InferenceModel != nil {
		log.FromContext(ctx).V(1).Info("attempting to update inference model")
		interpolatedBaseConfig.mergeInferenceModel(ctx, modelService, scheme)
	}

	if interpolatedBaseConfig.EPPDeployment != nil {
		log.FromContext(ctx).V(1).Info("attempting to update epp deployment and service")
		interpolatedBaseConfig.mergeEppDeployment(ctx, modelService, scheme)
		if interpolatedBaseConfig.EPPService != nil {
			interpolatedBaseConfig.mergeEppService(ctx, modelService, scheme)
		}
		interpolatedBaseConfig.setEPPServiceAccount(ctx, modelService, rbacOptions, scheme)
		// this is role binding with a cluster role
		interpolatedBaseConfig.setEPPRoleBinding(ctx, modelService, rbacOptions, scheme)
	}

	return interpolatedBaseConfig
}

// mergeConfigMaps creates config maps for found in base config
func (childResource *BaseConfig) mergeConfigMaps(ctx context.Context, msvc *msv1alpha1.ModelService, scheme *runtime.Scheme) *BaseConfig {
	for i := range childResource.ConfigMaps {
		childResource.ConfigMaps[i].APIVersion = "v1"
		childResource.ConfigMaps[i].Kind = "ConfigMap"
		// if there's no namespace, set it to msvc's
		if strings.TrimSpace(childResource.ConfigMaps[i].Namespace) == "" {
			childResource.ConfigMaps[i].Namespace = msvc.Namespace
		}
		// Note: there seems to be a controllerutil bug here ...
		// Setting owner ref before setting namespace seems problematic
		err := controllerutil.SetOwnerReference(msvc, &childResource.ConfigMaps[i], scheme)
		if err != nil {
			log.FromContext(ctx).V(1).Error(err, "unable to set owner reference")
		}
	}
	return childResource
}

// getCommonLabels that are applicable to all resources owned by msvc
func getCommonLabels(ctx context.Context, msvc *msv1alpha1.ModelService) map[string]string {
	// Step 3: Define object meta
	// Sanitize modelName into a valid label
	// TODO: this is not a good approach. Confirm with routing team on what label they need
	modelLabel, err := sanitizeName(msvc.Spec.Routing.ModelName)
	if err != nil {
		log.FromContext(ctx).V(1).Error(err, "unable to sanitize model name")
	}

	return map[string]string{
		"llm-d.ai/inferenceServing": "true",
		"llm-d.ai/model":            modelLabel,
	}
}

// getPodLabels adds a role on top of the common labels
func getPodLabels(ctx context.Context, msvc *msv1alpha1.ModelService, role string) map[string]string {
	labels := getCommonLabels(ctx, msvc)
	labels["llm-d.ai/role"] = role
	return labels
}

// mergeInferenceModel uses msvc fields to update childResource inference model
func (childResources *BaseConfig) mergeInferenceModel(ctx context.Context, msvc *msv1alpha1.ModelService, scheme *runtime.Scheme) *BaseConfig {
	// there's nothing to update
	if childResources.InferenceModel == nil {
		return childResources
	}

	im := childResources.InferenceModel

	im.APIVersion = "inference.networking.x-k8s.io/v1alpha2"
	im.Kind = "InferenceModel"
	im.Name = infModelName(msvc)
	im.Namespace = msvc.Namespace
	im.Labels = getCommonLabels(ctx, msvc)
	im.Spec.ModelName = msvc.Spec.Routing.ModelName
	im.Spec.PoolRef.Name = giev1alpha2.ObjectName(infPoolName(msvc))

	// Set owner reference for the merged service
	if err := controllerutil.SetOwnerReference(msvc, im, scheme); err != nil {
		log.FromContext(ctx).V(1).Error(err, "unable to set owner ref for inferencepool")
		return childResources
	}

	return childResources
}

// sanitizeSvcName returns the
func sanitizeSvcName(msvc *msv1alpha1.ModelService, role string) string {
	sanitizedName, err := sanitizeName(msvc.Name + "-service-" + role)
	if err != nil {
		// TODO: don't return a default name?
		return "default-service-" + role
	}

	return sanitizedName
}

func sanitizeModelName(msvc *msv1alpha1.ModelService) string {
	sanitizedModelName, err := sanitizeName(msvc.Spec.Routing.ModelName)
	if err != nil {
		// TODO: don't return a default model name?
		return "default-modelName"
	}

	return sanitizedModelName
}

// mergePDService uses msvc fields to update childResource P/D Service
func (childResource *BaseConfig) mergePDService(ctx context.Context, msvc *msv1alpha1.ModelService, role string, scheme *runtime.Scheme) *BaseConfig {

	// Get dest Service
	destService := corev1.Service{}

	if role == PREFILL_ROLE {
		if childResource.PrefillService != nil {
			destService = *childResource.PrefillService

		} else {
			// prefillService is not specified in baseConfig, so we are not going to create a service. Return
			return childResource
		}
	} else {
		if childResource.DecodeService != nil {
			destService = *childResource.DecodeService
		} else {
			// decodeService is not specified in baseConfig, so we are not going to create a service. Return
			return childResource
		}
	}

	destService.APIVersion = "v1"
	destService.Kind = "Service"

	// At this point, we are going to create a service for role
	// srcService contains ownerRef, name for service, and selector labels
	// srcService contains the stuff we want to override destService with

	srcService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sanitizeSvcName(msvc, role),
			Namespace: msvc.Namespace,
		},
		Spec: corev1.ServiceSpec{
			// If destService contains labels, it will do a override based on key
			// otherwise, just add to the map
			Selector: getPodLabels(ctx, msvc, role),
		},
	}

	// Mergo merge src into dst
	if err := mergo.Merge(&destService, srcService, mergo.WithOverride); err != nil {
		log.FromContext(ctx).V(1).Error(err, "problem with service merge for "+role)
		return childResource
	}

	// Set owner reference for the merged service
	if err := controllerutil.SetOwnerReference(msvc, &destService, scheme); err != nil {
		log.FromContext(ctx).V(1).Error(err, "unable to set owner ref for service "+role)
		return childResource
	}

	// Set the merged service for child resource
	if role == PREFILL_ROLE {
		childResource.PrefillService = &destService
	} else {
		childResource.DecodeService = &destService
	}

	return childResource
}

// mergePDDeployment uses msvc fields to update childResource prefill deployment
func (childResource *BaseConfig) mergePDDeployment(ctx context.Context, msvc *msv1alpha1.ModelService, role string, scheme *runtime.Scheme) *BaseConfig {
	pdSpec := &msv1alpha1.PDSpec{}
	if role == PREFILL_ROLE {
		if msvc.Spec.Prefill != nil {
			pdSpec = msvc.Spec.Prefill
		}
		if childResource.PrefillDeployment == nil {
			childResource.PrefillDeployment = &appsv1.Deployment{}
		}
	}
	if role == DECODE_ROLE {
		if msvc.Spec.Decode != nil {
			pdSpec = msvc.Spec.Decode
		}
		if childResource.DecodeDeployment == nil {
			childResource.DecodeDeployment = &appsv1.Deployment{}
		}
	}

	var err error

	// Compute fields needed
	podLabels := getPodLabels(ctx, msvc, role)
	var nodeAffinity *corev1.Affinity

	// AcceleratorTypes maybe nil... TODO: check
	na, err := AcceleratorTypesToNodeAffinity(pdSpec.AcceleratorTypes)
	if err == nil {
		nodeAffinity = &corev1.Affinity{
			NodeAffinity: na,
		}
	} else {
		log.FromContext(ctx).V(1).Error(err, "unable to get node affinity")
	}

	// Step 1: Create an empty deployment
	desiredDeployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},

		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName(msvc, role),
			Namespace: msvc.Namespace,

			// Define the labels for this PD deployment
			// Same as pod labels
			Labels: podLabels,
		},
		Spec: appsv1.DeploymentSpec{
			// Define template selector labels
			Selector: &metav1.LabelSelector{
				MatchLabels: podLabels,
			},

			// Define replicas
			// Decouple scaling will be handled in the merge
			Replicas: pdSpec.Replicas,

			// Define pod templates with our templates
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					// Define pod labels, must match selector labels
					Labels: podLabels,
				},
				Spec: corev1.PodSpec{
					// populate containers
					InitContainers: ConvertToContainerSliceWithVolumeMount(ctx, pdSpec.InitContainers, msvc),
					Containers:     ConvertToContainerSliceWithVolumeMount(ctx, pdSpec.Containers, msvc),

					// populate node affinity
					Affinity: nodeAffinity,

					// populate service account for PD pods
					ServiceAccountName: pdServiceAccountName(msvc),

					// populate volumes based on URI
					Volumes: getVolumeForPDDeployment(ctx, msvc),
				},
			},
		},
	}

	// Finally, set owner references
	err = controllerutil.SetOwnerReference(msvc, desiredDeployment, scheme)
	if err != nil {
		log.FromContext(ctx).V(1).Error(err, "unable to set owner reference")
	}

	// Finally, in Mergo merge...
	// We create a destination deployment object from baseconfig
	// We create a source deployment object from model service
	// We merge source into destination
	// We apply the merged destination

	var originalDeployment *appsv1.Deployment

	log.FromContext(ctx).V(1).Info("merging PD deployment", "role", role, "desiredDeployment (src)", desiredDeployment)
	if role == PREFILL_ROLE {
		originalDeployment = childResource.PrefillDeployment
	}
	if role == DECODE_ROLE {
		originalDeployment = childResource.DecodeDeployment
	}

	// Mergo merge
	log.FromContext(ctx).V(1).Info("merging PD deployment", "originalDeployment (dst)", originalDeployment)
	if err = mergo.Merge(
		originalDeployment,
		desiredDeployment,
		mergo.WithOverride,
		mergo.WithAppendSlice,
		mergo.WithTransformers(containerSliceTransformer{})); err != nil {
		log.FromContext(ctx).V(1).Error(err, "mergo error")
	} else {

		// Log errors
		// technically we can log using originalDeployment here, but be safe
		// and log what's directly stored in childResources.<ROLE>Deployment
		var mergedDeployment *appsv1.Deployment
		if role == DECODE_ROLE {
			mergedDeployment = childResource.PrefillDeployment
		} else {
			mergedDeployment = childResource.DecodeDeployment
		}
		log.FromContext(ctx).V(1).Info("merging was succesful", "merged deployment", mergedDeployment)
	}

	return childResource
}

// setPDServiceAccount defines a servicd account for the P and D deployments
func (childResource *BaseConfig) setPDServiceAccount(ctx context.Context, msvc *msv1alpha1.ModelService, scheme *runtime.Scheme, rbacOptions *RBACOptions) *BaseConfig {
	sa := &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      pdServiceAccountName(msvc),
			Namespace: msvc.Namespace,
		},
	}

	for _, name := range rbacOptions.PDPullSecrets {
		sa.ImagePullSecrets = append(sa.ImagePullSecrets, corev1.LocalObjectReference{Name: name})
	}

	// Set owner reference for service account
	// TODO: should childresource be returned when owner ref is not set?
	if err := controllerutil.SetOwnerReference(msvc, sa, scheme); err != nil {
		log.FromContext(ctx).V(1).Error(err, "unable to set owner ref for service account")
		return childResource
	}

	childResource.PDServiceAccount = sa

	return childResource
}

func (childResource *BaseConfig) setEPPServiceAccount(ctx context.Context, msvc *msv1alpha1.ModelService, rbacOptions *RBACOptions, scheme *runtime.Scheme) {
	eppServiceAccount := &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      eppServiceAccountName(msvc),
			Namespace: msvc.Namespace,
		},
	}

	for _, name := range rbacOptions.EPPPullSecrets {
		eppServiceAccount.ImagePullSecrets = append(eppServiceAccount.ImagePullSecrets, corev1.LocalObjectReference{Name: name})
	}

	childResource.EPPServiceAccount = eppServiceAccount

	// TODO: should childresource be returned when owner ref is not set?
	if err := controllerutil.SetOwnerReference(msvc, eppServiceAccount, scheme); err != nil {
		log.FromContext(ctx).V(1).Error(err, "unable to set owner ref for service account")
	}
}

func (childResource *BaseConfig) setEPPRoleBinding(ctx context.Context, msvc *msv1alpha1.ModelService, rbacOptions *RBACOptions, scheme *runtime.Scheme) {

	childResource.EPPRoleBinding = &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RoleBinding",
			APIVersion: "rbac.authorization.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      eppRolebindingName(msvc),
			Namespace: msvc.Namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				APIGroup:  "",
				Name:      eppServiceAccountName(msvc),
				Namespace: msvc.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			APIGroup: "rbac.authorization.k8s.io",
			Name:     rbacOptions.EPPClusterRole,
		},
	}

	// Set owner reference for EPPRoleBinding
	if err := controllerutil.SetOwnerReference(msvc, childResource.EPPRoleBinding, scheme); err != nil {
		log.FromContext(ctx).V(1).Error(err, "unable to set owner ref for epp rolebinding")
	}

}

func getInferencePoolLabels(ctx context.Context, msvc *msv1alpha1.ModelService) map[giev1alpha2.LabelKey]giev1alpha2.LabelValue {
	commonLabels := getCommonLabels(ctx, msvc)
	m := make(map[giev1alpha2.LabelKey]giev1alpha2.LabelValue, len(commonLabels))
	for k, v := range commonLabels {
		m[giev1alpha2.LabelKey(k)] = giev1alpha2.LabelValue(v)
	}
	return m
}

func (childResources *BaseConfig) mergeEppDeployment(ctx context.Context, msvc *msv1alpha1.ModelService, scheme *runtime.Scheme) *BaseConfig {

	if childResources == nil || childResources.EPPDeployment == nil {
		return childResources
	}

	eppLabels := map[string]string{
		"llm-d.ai/epp": eppDeploymentName(msvc),
	}
	dest := *childResources.EPPDeployment

	src := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      eppDeploymentName(msvc),
			Namespace: msvc.Namespace,
		},
	}

	src.Labels = eppLabels

	src.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: eppLabels,
	}

	src.Spec.Template.ObjectMeta = metav1.ObjectMeta{
		Labels: eppLabels,
	}

	modelSrvPodSpec := &msv1alpha1.ModelServicePodSpec{}
	if msvc.Spec.EndpointPicker != nil {
		modelSrvPodSpec = msvc.Spec.EndpointPicker
	}
	src.Spec.Replicas = modelSrvPodSpec.Replicas
	src.Spec.Template.Spec.Containers = ConvertToContainerSlice(modelSrvPodSpec.Containers)
	src.Spec.Template.Spec.InitContainers = ConvertToContainerSlice(modelSrvPodSpec.InitContainers)

	// set epp service account name
	src.Spec.Template.Spec.ServiceAccountName = eppServiceAccountName(msvc)

	err := mergo.Merge(&dest, src, mergo.WithOverride, mergo.WithAppendSlice, mergo.WithTransformers(containerSliceTransformer{}))
	if err != nil {
		log.FromContext(ctx).V(1).Error(err, "problem with epp deployment merge")
		return childResources
	}

	// Set owner reference for the merged service
	if err := controllerutil.SetOwnerReference(msvc, &dest, scheme); err != nil {
		log.FromContext(ctx).V(1).Error(err, "unable to set owner ref for inferencepool")
		return childResources
	}
	log.FromContext(ctx).V(1).Info("deployment", "post-merge-label", dest.Labels, "post-merge-spec", dest.Spec)
	// Set the merged epp deployment in the child resource
	childResources.EPPDeployment = &dest

	return childResources
}

func (childResources *BaseConfig) mergeEppService(ctx context.Context, msvc *msv1alpha1.ModelService, scheme *runtime.Scheme) *BaseConfig {
	if childResources == nil || childResources.EPPService == nil {
		return childResources
	}
	eppLabels := map[string]string{
		"llm-d.ai/epp": eppDeploymentName(msvc),
	}
	dest := *childResources.EPPService
	src := corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      eppServiceName(msvc),
			Namespace: msvc.Namespace,
			Labels:    eppLabels,
		},
	}

	src.Spec.Selector = eppLabels
	if err := mergo.Merge(&dest, src, mergo.WithOverride); err != nil {
		log.FromContext(ctx).V(1).Error(err, "problem with epp service merge")
		return childResources
	}
	if err := controllerutil.SetOwnerReference(msvc, &dest, scheme); err != nil {
		log.FromContext(ctx).V(1).Error(err, "unable to set owner ref for inferencepool")
		return childResources
	}

	// Set the merged epp service in the child resource
	childResources.EPPService = &dest
	return childResources
}

// mergeHTTPRoute uses msvc fields to update childResource HTTPRoute resource.
func (childResources *BaseConfig) mergeHTTPRoute(ctx context.Context, msvc *msv1alpha1.ModelService, scheme *runtime.Scheme) *BaseConfig {

	if childResources == nil || childResources.HTTPRoute == nil {
		return childResources
	}

	// Get dest HTTPRoute
	dest := *childResources.HTTPRoute

	group := gatewayv1.Group("inference.networking.x-k8s.io")
	kind := gatewayv1.Kind("InferencePool")

	// At this point, we are going to create the desired HTTPRoute
	// with the parentRefs supplied by the user and
	// with InferencePool being a backendRef (we are adding this)
	src := &gatewayv1.HTTPRoute{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HTTPRoute",
			APIVersion: "gateway.networking.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      httpRouteName(msvc),
			Namespace: msvc.Namespace,
		},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: msvc.Spec.Routing.GatewayRefs,
			},
			Rules: []gatewayv1.HTTPRouteRule{
				{
					BackendRefs: []gatewayv1.HTTPBackendRef{
						{
							// InferencePool is added as a backendRef
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Group: &group,
									Kind:  &kind,
									Name:  gatewayv1.ObjectName(infPoolName(msvc)),
								},
							},
						},
					},
				},
			},
		},
	}

	// Mergo merge src into dst
	if err := mergo.Merge(&dest,
		src,
		mergo.WithOverride,
		mergo.WithAppendSlice,
		mergo.WithTransformers(compositeTransformer{
			transformers: []mergo.Transformers{
				// merge parentRef and backendRef slices
				parentRefSliceTransformer{},
				backendRefTransformer{},
			},
		}),
	); err != nil {
		log.FromContext(ctx).V(1).Error(err, "problem with httproute merge")
		return childResources
	}

	// Set owner reference for the merged service
	if err := controllerutil.SetOwnerReference(msvc, &dest, scheme); err != nil {
		log.FromContext(ctx).V(1).Error(err, "unable to set owner ref for httproute")
		return childResources
	}

	// Set the merged inferncepool in the child resource
	childResources.HTTPRoute = &dest

	return childResources
}

// mergeInferencePool uses msvc fields to update childResource InferencePool resource.
func (childResources *BaseConfig) mergeInferencePool(ctx context.Context, msvc *msv1alpha1.ModelService, scheme *runtime.Scheme) *BaseConfig {

	if childResources == nil || childResources.InferencePool == nil {
		return childResources
	}

	// Get dest Service
	dest := *childResources.InferencePool

	// At this point, we are going to create a service for role
	// srcService contains ownerRef, name for service, and selector labels
	// srcService contains the stuff we want to override destService with
	src := &giev1alpha2.InferencePool{
		TypeMeta: metav1.TypeMeta{
			Kind:       "InferencePool",
			APIVersion: "inference.networking.x-k8s.io/v1alpha2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      infPoolName(msvc),
			Namespace: msvc.Namespace,
		},
		Spec: giev1alpha2.InferencePoolSpec{
			Selector: getInferencePoolLabels(ctx, msvc),
			EndpointPickerConfig: giev1alpha2.EndpointPickerConfig{
				ExtensionRef: &giev1alpha2.Extension{
					ExtensionReference: giev1alpha2.ExtensionReference{
						Name: giev1alpha2.ObjectName(eppServiceName(msvc)),
					},
				},
			},
		},
	}

	// Mergo merge src into dst
	if err := mergo.Merge(&dest, src, mergo.WithOverride); err != nil {
		log.FromContext(ctx).V(1).Error(err, "problem with inferencepool merge")
		return childResources
	}

	// Set owner reference for the merged service
	if err := controllerutil.SetOwnerReference(msvc, &dest, scheme); err != nil {
		log.FromContext(ctx).V(1).Error(err, "unable to set owner ref for inferencepool")
		return childResources
	}

	// Set the merged inferncepool in the child resource
	childResources.InferencePool = &dest

	return childResources
}

// invokeCreateOrUpdate decides whether to invoke a createOrUpdate call for each child resource
func (childResource *BaseConfig) invokeCreateOrUpdate(ctx context.Context, r *ModelServiceReconciler, msvc *msv1alpha1.ModelService) []error {

	var results []error

	// CreateOrUpdate ConfigMaps
	if childResource.shouldCreateConfigMaps() {
		results = append(results, createOrUpdateConfigMaps(ctx, r, childResource.ConfigMaps)...)
	}

	// CreateOrUpdate PD deployments, services, and SA
	if childResource.shouldCreatePrefillDeployment() {
		results = append(results, createOrUpdatePDDeployment(ctx, r, childResource.PrefillDeployment, msvc.Spec.DecoupleScaling))
	}

	if childResource.shouldCreatePrefillService() {
		results = append(results, createOrUpdateService(ctx, r, childResource.PrefillService))
	}

	if childResource.shouldCreateDecodeDeployment() {
		results = append(results, createOrUpdatePDDeployment(ctx, r, childResource.DecodeDeployment, msvc.Spec.DecoupleScaling))
	}

	if childResource.shouldCreateDecodeService() {
		results = append(results, createOrUpdateService(ctx, r, childResource.DecodeService))
	}

	if childResource.shouldCreatePDServiceAccount() {
		results = append(results, createOrUpdateServiceAccount(ctx, r, childResource.PDServiceAccount))
	}

	// CreateOrUpdate routing components and RBACs for each if required
	if childResource.shouldCreateEPPDeployment() {
		results = append(results, createOrUpdateDeployment(ctx, r, childResource.EPPDeployment))
	}

	if childResource.shouldCreateEPPService() {
		results = append(results, createOrUpdateService(ctx, r, childResource.EPPService))
	}

	if childResource.shouldCreateEPPServiceAccount() {
		results = append(results, createOrUpdateServiceAccount(ctx, r, childResource.EPPServiceAccount))
	}

	if childResource.shouldCreateEPPRoleBinding() {
		results = append(results, createOrUpdateRoleBinding(ctx, r, childResource.EPPRoleBinding))
	}

	if childResource.shouldCreateHTTPRoute() {
		results = append(results, createOrUpdateHTTPRoute(ctx, r, childResource.HTTPRoute))
	}

	if childResource.shouldCreateInferencePool() {
		results = append(results, createOrUpdateInferencePool(ctx, r, childResource.InferencePool))
	}

	if childResource.shouldCreateInferenceModel() {
		results = append(results, createOrUpdateInferenceModel(ctx, r, childResource.InferenceModel))
	}

	// Keep only the actual errors
	var nonNilErrors []error
	for _, e := range results {
		if e != nil {
			nonNilErrors = append(nonNilErrors, e)
		}
	}

	return nonNilErrors
}

// genericCreateOrUpdate is a generic function that creates or updates an object in the cluster
// desiredObjectState is the desired state of the object
// emptyObject is an empty object that can be used to retrieve the current object in the cluster, if present,
// and update it to the desired state
// the mutate func for createOrUpdate is a merge between the currentObject and desiredObject
// the merge ensures that annotations and labels are preseved while the spec is updated to the desired staet
func genericCreateOrUpdate(ctx context.Context, r *ModelServiceReconciler, desiredObjectState client.Object, emptyObject client.Object) error {
	var err error

	desiredObjName := desiredObjectState.GetName()
	desiredObjNamespace := desiredObjectState.GetNamespace()

	// emptyObject is the object to look for in the cluster
	log.FromContext(ctx).V(1).Info("looking to createOrUpdate object in cluster: ", "obj name", desiredObjName, "obj namespace", desiredObjNamespace, "obj kind", desiredObjectState.GetObjectKind())

	// Set the empty object with the name and namespace so we can look it up in the cluster
	// and populate emptyObject with the current state of the object
	emptyObject.SetName(desiredObjName)
	emptyObject.SetNamespace(desiredObjNamespace)

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, emptyObject, func() error {
		// Mergo merge with override means labels, annotations (maps) key-value pairs are preseved
		// while other fields are overriden if not nil in desiredObjectState
		mergeErr := mergo.Merge(emptyObject, desiredObjectState, mergo.WithOverride)
		if mergeErr != nil {
			log.FromContext(ctx).V(1).Error(err, "attemping to merge inside createOrUpdate, but failed for object "+emptyObject.GetName())
		} else {
			log.FromContext(ctx).V(1).Info("successfully merged object in cluster" + emptyObject.GetName())
		}

		return mergeErr
	})

	log.FromContext(ctx).V(1).Info("performed createOrUpdate", "obj name", emptyObject.GetName(), "operation", op)
	if err != nil {
		log.FromContext(ctx).V(1).Error(err, "createOrUpdate failed", "obj name", emptyObject.GetName())
	}

	return err
}

// createOrUpdatePDDeploymentInCluster is a function for creating or updating a PD deployment in the cluster,
// taking into account decoupling scaling
// generally mirrors genericCreateOrUpdate
// TODO: can we consolidate the two funcs into a more generic function?
func createOrUpdatePDDeploymentInCluster(ctx context.Context, r *ModelServiceReconciler, desiredObjectState appsv1.Deployment, emptyObject *appsv1.Deployment, decoupleScaling bool) error {
	var err error

	desiredObjName := desiredObjectState.GetName()
	desiredObjNamespace := desiredObjectState.GetNamespace()

	// emptyObject is the object to look for in the cluster
	log.FromContext(ctx).V(1).Info("looking to createOrUpdate object in cluster: ", "obj name", desiredObjName, "obj namespace", desiredObjNamespace, "obj kind", desiredObjectState.GetObjectKind())

	// Set the empty object with the name and namespace so we can look it up in the cluster
	// and populate emptyObject with the current state of the object
	emptyObject.SetName(desiredObjName)
	emptyObject.SetNamespace(desiredObjNamespace)

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, emptyObject, func() error {
		log.FromContext(ctx).V(1).Info("initial replica count", "replica", emptyObject.Spec.Replicas)

		// We should only update replica if decoupleScaling is False
		if !emptyObject.GetCreationTimestamp().Time.IsZero() && decoupleScaling {
			log.FromContext(ctx).V(1).Info("scaling is decoupled, setting deployment replica value to incluster replica count")
			desiredObjectState.Spec.Replicas = nil
		}

		// Mergo merge with override means labels, annotations (maps) key-value pairs are preseved
		// while other fields are overriden if not nil in desiredObjectState
		mergeErr := mergo.Merge(emptyObject, desiredObjectState, mergo.WithOverride)
		if mergeErr != nil {
			log.FromContext(ctx).V(1).Error(err, "attemping to merge inside createOrUpdate, but failed for object "+emptyObject.GetName())
		} else {
			log.FromContext(ctx).V(1).Info("successfully merged PD deployment in cluster" + emptyObject.GetName())
			log.FromContext(ctx).V(1).Info("successfully merged PD deployment in cluster", "replica", emptyObject.Spec.Replicas)
		}

		return mergeErr
	})

	log.FromContext(ctx).V(1).Info("performed createOrUpdate", "obj name", emptyObject.GetName(), "operation", op)
	if err != nil {
		log.FromContext(ctx).V(1).Error(err, "createOrUpdate failed", "obj name", emptyObject.GetName())
	}

	return err
}

// createOrUpdateConfigMaps creates or updates multiple of ConfigMaps in the cluster
func createOrUpdateConfigMaps(ctx context.Context, r *ModelServiceReconciler, desiredConfigMaps []corev1.ConfigMap) []error {
	var errors []error
	for _, desiredConfigMap := range desiredConfigMaps {
		emptyConfigMap := corev1.ConfigMap{}
		errors = append(errors, genericCreateOrUpdate(ctx, r, &desiredConfigMap, &emptyConfigMap))
	}
	return errors
}

// createOrUpdateDeployment creates or updates a deployment object in the cluster
func createOrUpdateDeployment(ctx context.Context, r *ModelServiceReconciler, desiredDeployment *appsv1.Deployment) error {
	emptyDeployment := appsv1.Deployment{}
	return genericCreateOrUpdate(ctx, r, desiredDeployment, &emptyDeployment)
}

// createOrUpdatePDDeployment creates or updates a PD deployment object in the cluster
// specifically, takes into account decoupleScaling
func createOrUpdatePDDeployment(ctx context.Context, r *ModelServiceReconciler, desiredDeployment *appsv1.Deployment, decoupleScaling bool) error {
	emptyDeployment := appsv1.Deployment{}
	return createOrUpdatePDDeploymentInCluster(ctx, r, *desiredDeployment, &emptyDeployment, decoupleScaling)
}

// createOrUpdateService creates or updates a service object in the cluster
func createOrUpdateService(ctx context.Context, r *ModelServiceReconciler, desiredService *corev1.Service) error {
	emptyService := corev1.Service{}
	return genericCreateOrUpdate(ctx, r, desiredService, &emptyService)
}

// createOrUpdateServiceAccount creates or updates a serviceAccount object in the cluster
func createOrUpdateServiceAccount(ctx context.Context, r *ModelServiceReconciler, desiredServiceAccount *corev1.ServiceAccount) error {
	emptyServiceAccount := corev1.ServiceAccount{}
	return genericCreateOrUpdate(ctx, r, desiredServiceAccount, &emptyServiceAccount)
}

// createOrUpdateRoleBinding creates or updates a RoleBinding object in the cluster
func createOrUpdateRoleBinding(ctx context.Context, r *ModelServiceReconciler, desiredRoleBinding *rbacv1.RoleBinding) error {
	emptyRoleBinding := rbacv1.RoleBinding{}
	return genericCreateOrUpdate(ctx, r, desiredRoleBinding, &emptyRoleBinding)
}

// createOrUpdateHTTPRoute creates or updates a HTTPRoute object in the cluster
func createOrUpdateHTTPRoute(ctx context.Context, r *ModelServiceReconciler, desiredInferenceModel *gatewayv1.HTTPRoute) error {
	emptyHTTPRoute := gatewayv1.HTTPRoute{}
	return genericCreateOrUpdate(ctx, r, desiredInferenceModel, &emptyHTTPRoute)
}

// createOrUpdateInferencePool creates or updates a InferencePool object in the cluster
func createOrUpdateInferencePool(ctx context.Context, r *ModelServiceReconciler, desiredInferencePool *giev1alpha2.InferencePool) error {
	emptyInferencePool := giev1alpha2.InferencePool{}
	return genericCreateOrUpdate(ctx, r, desiredInferencePool, &emptyInferencePool)
}

// createOrUpdateInferenceModel creates or updates a InferenceModel object in the cluster
func createOrUpdateInferenceModel(ctx context.Context, r *ModelServiceReconciler, desiredInferenceModel *giev1alpha2.InferenceModel) error {
	emptyInferenceModel := giev1alpha2.InferenceModel{}
	return genericCreateOrUpdate(ctx, r, desiredInferenceModel, &emptyInferenceModel)
}
