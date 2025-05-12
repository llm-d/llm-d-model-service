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
	"strings"
	"text/template"

	"dario.cat/mergo"
	msv1alpha1 "github.com/neuralmagic/llm-d-model-service/api/v1alpha1"
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
	InferencePool     *giev1alpha2.InferencePool  `json:"inferencePool,omitempty"`
	InferenceModel    *giev1alpha2.InferenceModel `json:"inferenceModel,omitempty"`
	EPPDeployment     *appsv1.Deployment          `json:"eppDeployment,omitempty"`
	EPPService        *corev1.Service             `json:"eppService,omitempty"`
	EPPServiceAccount *corev1.ServiceAccount      `json:"eppServiceAccount,omitempty"`
	PDServiceAccount  *corev1.ServiceAccount      `json:"pdServiceAccount,omitempty"`
	EPPRoleBinding    *rbacv1.RoleBinding         `json:"eppRoleBinding,omitempty"`
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

	// Step 1: Create an empty deployment
	depl := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
	}

	// Step 3: Define object meta
	// Sanitize modelName into a valid label
	// TODO: this is not a good approach. Confirm with routing team on what label they need

	labels := map[string]string{
		"llm-d.ai/inferenceServing": "true",
		"llm-d.ai/model":            sanitizeModelName(msvc),
		"llm-d.ai/role":             role,
	}

	log.FromContext(ctx).V(1).Info("model service", "type meta", msvc.TypeMeta)

	depl.ObjectMeta = metav1.ObjectMeta{
		Name:      deploymentName(msvc, role),
		Namespace: msvc.Namespace,
		Labels:    labels,
	}
	err = controllerutil.SetOwnerReference(msvc, depl, scheme)
	if err != nil {
		log.FromContext(ctx).V(1).Error(err, "unable to set owner reference")
	}

	// Step 4b. Define selectors for pods
	depl.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: labels,
	}

	// Step 4c. Define pod templates with our templates
	depl.Spec.Template.ObjectMeta = metav1.ObjectMeta{
		Labels: labels,
	}

	// Step 4: populate containers
	depl.Spec.Template.Spec.Containers = ConvertToContainerSlice(pdSpec.Containers)
	depl.Spec.Template.Spec.InitContainers = ConvertToContainerSlice(pdSpec.InitContainers)

	// Step 5: populate nodeaffinity
	// AcceleratorTypes maybe nil... TODO: check
	na, err := AcceleratorTypesToNodeAffinity(pdSpec.AcceleratorTypes)
	if err == nil {
		depl.Spec.Template.Spec.Affinity = &corev1.Affinity{
			NodeAffinity: na,
		}
	} else {
		log.FromContext(ctx).V(1).Error(err, "unable to get node affinity")
	}

	// Step 6: populate replicas
	// ToDo: handle decouple scaling as part of apply logic
	// ToDo: decouple scaling
	depl.Spec.Replicas = pdSpec.Replicas

	// Step 8: TODO: change volume, volume mounts based on modelArtifacts
	if isPVCURI(msvc.Spec.ModelArtifacts.URI) {
		// Set volumes according to ModelArtifacts specs
		volume, err := getVolumeFromModelArtifacts(&msvc.Spec.ModelArtifacts)
		if err != nil {
			// The modelArtifact is invalid
			log.FromContext(ctx).V(1).Error(err, "unable to get volume")
		}
		if volume != nil {
			depl.Spec.Template.Spec.Volumes = append(depl.Spec.Template.Spec.Volumes, *volume)
		}
	}

	// set service account for pd pods
	depl.Spec.Template.Spec.ServiceAccountName = pdServiceAccountName(msvc)

	// Step 9: We need to figure out the volume mount story in
	// addition to the volume story above ...

	// In Mergo merge...
	// We create a destination deployment object from baseconfig
	// We create a source deployment object from model service
	// We merge source into destination
	// We apply the merged destination
	if role == PREFILL_ROLE {
		log.FromContext(ctx).V(1).Info("mergo info", "role", role, "dst", childResource.PrefillDeployment, "src", depl)
		err = mergo.Merge(childResource.PrefillDeployment, depl, mergo.WithOverride, mergo.WithAppendSlice, mergo.WithTransformers(containerSliceTransformer{}))
		if err != nil {
			log.FromContext(ctx).V(1).Error(err, "mergo error")
		}
	}
	if role == DECODE_ROLE {
		log.FromContext(ctx).V(1).Info("mergo info", "role", role, "dst", childResource.DecodeDeployment, "src", depl)
		log.FromContext(ctx).V(1).Info("before mergo info -- details", "role", role, "dst.spec", childResource.DecodeDeployment.Spec, "src.spec", depl.Spec)
		err = mergo.Merge(childResource.DecodeDeployment, depl, mergo.WithOverride, mergo.WithAppendSlice, mergo.WithTransformers(containerSliceTransformer{}))
		log.FromContext(ctx).V(1).Info("after mergo info -- details", "role", role, "dst.spec", childResource.DecodeDeployment.Spec)
		if err != nil {
			log.FromContext(ctx).V(1).Error(err, "mergo error")
		}
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

// createOrUpdate all the child resources
func (childResource *BaseConfig) createOrUpdate(ctx context.Context, r *ModelServiceReconciler, decoupleScaling bool) error {
	// create or update configmaps
	log.FromContext(ctx).V(1).Info("attempting to createOrUpdate configmaps")
	childResource.createOrUpdateConfigMaps(ctx, r)

	log.FromContext(ctx).V(1).Info("attempting to createOrUpdate prefill deployment")
	childResource.createOrUpdatePDDeployment(ctx, r, PREFILL_ROLE, decoupleScaling)

	log.FromContext(ctx).V(1).Info("attempting to createOrUpdate decode deployment")
	childResource.createOrUpdatePDDeployment(ctx, r, DECODE_ROLE, decoupleScaling)

	// Create or update services only if corresponding deployment exists in childResources
	childResource.createOrUpdateServiceForDeployment(ctx, r, PREFILL_ROLE)
	childResource.createOrUpdateServiceForDeployment(ctx, r, DECODE_ROLE)

	// create of update inference pool
	childResource.createOrUpdateInferencePool(ctx, r)

	// create of update inference model
	childResource.createOrUpdateInferenceModel(ctx, r)

	if childResource.EPPDeployment != nil {
		err := childResource.createEppDeployment(ctx, r.Client)
		if err != nil {
			log.FromContext(ctx).V(1).Error(err, "unable to create epp deployment")
		}
	}

	if childResource.EPPService != nil {
		err := childResource.createEppService(ctx, r.Client)
		if err != nil {
			log.FromContext(ctx).V(1).Error(err, "unable to create epp service")
		}
	}

	if childResource.EPPServiceAccount != nil {
		err := childResource.createEppServiceAccount(ctx, r.Client)
		if err != nil {
			log.FromContext(ctx).V(1).Error(err, "unable to create epp service account")
		}
	}

	if childResource.EPPRoleBinding != nil {
		err := childResource.createEppRoleBinding(ctx, r.Client)
		if err != nil {
			log.FromContext(ctx).V(1).Error(err, "unable to create epp service account")
		}
	}

	if childResource.PDServiceAccount != nil {
		err := childResource.createPDServiceAccount(ctx, r.Client)
		if err != nil {
			log.FromContext(ctx).V(1).Error(err, "unable to create epp service account")
		}
	}

	return nil
}

func (childResource *BaseConfig) createOrUpdateInferenceModel(ctx context.Context, r *ModelServiceReconciler) {
	// nothing to do without inf model
	if childResource == nil || childResource.InferenceModel == nil {
		return
	}

	infModelInCluster := &giev1alpha2.InferenceModel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      childResource.InferenceModel.Name,
			Namespace: childResource.InferenceModel.Namespace,
		},
	}

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, infModelInCluster, func() error {
		infModelInCluster.Labels = childResource.InferenceModel.Labels
		infModelInCluster.OwnerReferences = childResource.InferenceModel.OwnerReferences
		infModelInCluster.Spec = childResource.InferenceModel.Spec
		return nil
	})

	if err != nil {
		if op != controllerutil.OperationResultNone {
			log.FromContext(ctx).V(1).Error(err, "unable to create inference model")
		}
	}
}

func (childResource *BaseConfig) createOrUpdateConfigMaps(ctx context.Context, r *ModelServiceReconciler) {
	for i := range childResource.ConfigMaps {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      childResource.ConfigMaps[i].Name,
				Namespace: childResource.ConfigMaps[i].Namespace,
			}}

		op, err := controllerutil.CreateOrUpdate(ctx, r.Client, cm, func() error {
			cm.OwnerReferences = childResource.ConfigMaps[i].OwnerReferences
			cm.Labels = childResource.ConfigMaps[i].Labels
			cm.Data = childResource.ConfigMaps[i].Data
			return nil
		})
		if err != nil {
			if op != controllerutil.OperationResultNone {
				log.FromContext(ctx).V(1).Error(err, "unable to create configmap")
			}
		}
	}
}

func (childResource *BaseConfig) createOrUpdatePDDeployment(ctx context.Context, r *ModelServiceReconciler, role string, decoupleScaling bool) {

	var desiredDeployment *appsv1.Deployment

	if role == PREFILL_ROLE {
		desiredDeployment = childResource.PrefillDeployment
	} else {
		desiredDeployment = childResource.DecodeDeployment
	}

	if desiredDeployment != nil {
		deploymentInCluster := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      desiredDeployment.Name,
				Namespace: desiredDeployment.Namespace,
			}}

		op, err := controllerutil.CreateOrUpdate(ctx, r.Client, deploymentInCluster, func() error {
			deploymentInCluster.OwnerReferences = desiredDeployment.OwnerReferences
			deploymentInCluster.Labels = desiredDeployment.Labels
			inClusterReplicas := deploymentInCluster.Spec.Replicas
			deploymentInCluster.Spec = desiredDeployment.Spec
			if !deploymentInCluster.CreationTimestamp.IsZero() && decoupleScaling {
				log.FromContext(ctx).V(1).Info("scaling is decoupled, setting deployment replica value to incluster replica count")
				deploymentInCluster.Spec.Replicas = inClusterReplicas
			}
			return nil
		})
		log.FromContext(ctx).V(1).Info("from CreateOrUpdate", "op", op)
		if err != nil {
			if op != controllerutil.OperationResultNone {
				log.FromContext(ctx).V(1).Error(err, "unable to create deployment for "+role, "operation", op)
			}
		}
	}
}

func (childResource *BaseConfig) createOrUpdateServiceForDeployment(ctx context.Context, r *ModelServiceReconciler, role string) {

	var service *corev1.Service
	var deployment *appsv1.Deployment

	if role == PREFILL_ROLE {
		service = childResource.PrefillService
		deployment = childResource.PrefillDeployment
	} else {
		service = childResource.DecodeService
		deployment = childResource.DecodeDeployment
	}

	// Create service for the deployment iff deployment exists and service is defined in baseConfig
	if deployment != nil && service != nil {
		log.FromContext(ctx).V(1).Info("service for "+role, "service", service)

		svcInCluster := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      service.Name,
				Namespace: service.Namespace,
			}}

		op, err := controllerutil.CreateOrUpdate(ctx, r.Client, svcInCluster, func() error {
			svcInCluster.Labels = service.Labels
			svcInCluster.OwnerReferences = service.OwnerReferences
			svcInCluster.Spec = service.Spec
			return nil
		})

		if err != nil {
			if op != controllerutil.OperationResultNone {
				log.FromContext(ctx).V(1).Error(err, "unable to create service for "+role)
			}
		}
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

	// set epp service account name
	src.Spec.Template.Spec.ServiceAccountName = eppServiceAccountName(msvc)

	if err := mergo.Merge(&dest, src, mergo.WithOverride); err != nil {
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

func (childResource *BaseConfig) createOrUpdateInferencePool(ctx context.Context, r *ModelServiceReconciler) {
	// nothing to do without inf model
	if childResource == nil || childResource.InferencePool == nil {
		return
	}

	inferencePoolInCluster := &giev1alpha2.InferencePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      childResource.InferencePool.Name,
			Namespace: childResource.InferencePool.Namespace,
		},
	}
	log.FromContext(ctx).V(1).Info("merged inf pool", "data", childResource.InferencePool)
	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, inferencePoolInCluster, func() error {
		inferencePoolInCluster.Labels = childResource.InferenceModel.Labels
		inferencePoolInCluster.OwnerReferences = childResource.InferenceModel.OwnerReferences
		inferencePoolInCluster.Spec = childResource.InferencePool.Spec
		return nil
	})

	if err != nil {
		if op != controllerutil.OperationResultNone {
			log.FromContext(ctx).V(1).Error(err, "unable to create inference pool")
		}
	}

}

// createEppDeployment spawns epp deployment from immutable configmap
func (childResource *BaseConfig) createEppDeployment(ctx context.Context, kubeClient client.Client) error {

	if childResource == nil || childResource.EPPDeployment == nil {
		return nil
	}

	deploymentTobeCreatedOrUpdated := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			//baseconfig should have corret epp args for
			// poolname and namespaces
			// poolname format: <model-name>-modelservice
			// eg: facebook-opt-125m-model-service
			// namespace should be the namespace of msvc
			Name:      childResource.EPPDeployment.Name,
			Namespace: childResource.EPPDeployment.Namespace,
		},
	}

	op, err := controllerutil.CreateOrUpdate(ctx, kubeClient, deploymentTobeCreatedOrUpdated, func() error {
		deploymentTobeCreatedOrUpdated.Labels = childResource.EPPDeployment.Labels
		deploymentTobeCreatedOrUpdated.OwnerReferences = childResource.EPPDeployment.OwnerReferences
		deploymentTobeCreatedOrUpdated.Spec = childResource.EPPDeployment.Spec
		return nil
	})

	if err != nil {
		if op != controllerutil.OperationResultNone {
			log.FromContext(ctx).V(1).Error(err, "unable to create epp deployment from immutable base configmap ")
			return err
		}
	}
	return nil
}

// createEppDeployment spawns epp service from immutable configmap
func (childResource *BaseConfig) createEppService(ctx context.Context, kubeClient client.Client) error {

	if childResource == nil || childResource.EPPService == nil {
		return nil
	}

	serviceTobeCreatedOrUpdated := corev1.Service{ObjectMeta: metav1.ObjectMeta{
		Name:      childResource.EPPService.Name,
		Namespace: childResource.EPPService.Namespace,
	}}

	op, err := controllerutil.CreateOrUpdate(ctx, kubeClient, &serviceTobeCreatedOrUpdated, func() error {
		serviceTobeCreatedOrUpdated.Labels = childResource.EPPService.Labels
		serviceTobeCreatedOrUpdated.OwnerReferences = childResource.EPPService.OwnerReferences
		serviceTobeCreatedOrUpdated.Spec = childResource.EPPService.Spec
		return nil
	})

	if err != nil {
		if op != controllerutil.OperationResultNone {
			log.FromContext(ctx).V(1).Error(err, "unable to create epp service from immutable base configmap ")
			return err
		}
	}
	return nil
}

// createEppDeployment spawns epp service from immutable configmap
func (childResource *BaseConfig) createEppServiceAccount(ctx context.Context, kubeClient client.Client) error {

	if childResource == nil || childResource.EPPServiceAccount == nil {
		return nil
	}

	saTobeCreatedOrUpdated := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      childResource.EPPServiceAccount.Name,
			Namespace: childResource.EPPDeployment.Namespace,
		},
	}

	op, err := controllerutil.CreateOrUpdate(ctx, kubeClient, saTobeCreatedOrUpdated, func() error {
		saTobeCreatedOrUpdated.Labels = childResource.EPPServiceAccount.Labels
		saTobeCreatedOrUpdated.OwnerReferences = childResource.EPPServiceAccount.OwnerReferences
		saTobeCreatedOrUpdated.ImagePullSecrets = childResource.EPPServiceAccount.ImagePullSecrets
		return nil
	})

	if err != nil {
		if op != controllerutil.OperationResultNone {
			log.FromContext(ctx).V(1).Error(err, "unable to create epp service account")
			return err
		}
	}
	return nil
}

// createEppDeployment spawns epp service from immutable configmap
func (childResource *BaseConfig) createEppRoleBinding(ctx context.Context, kubeClient client.Client) error {

	if childResource == nil || childResource.EPPRoleBinding == nil {
		return nil
	}
	eppRoleBindingInCluster := rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      childResource.EPPRoleBinding.Name,
			Namespace: childResource.EPPRoleBinding.Namespace,
		}}

	op, err := controllerutil.CreateOrUpdate(ctx, kubeClient, &eppRoleBindingInCluster, func() error {
		eppRoleBindingInCluster.Labels = childResource.EPPRoleBinding.Labels
		eppRoleBindingInCluster.OwnerReferences = childResource.EPPRoleBinding.OwnerReferences
		eppRoleBindingInCluster.Subjects = childResource.EPPRoleBinding.Subjects
		eppRoleBindingInCluster.RoleRef = childResource.EPPRoleBinding.RoleRef
		return nil
	})
	if err != nil {
		if op != controllerutil.OperationResultNone {
			log.FromContext(ctx).V(1).Error(err, "unable to create epp rolebinding from immutable base configmap ")
			return err
		}
	}
	return nil
}

// createPDServiceAccount creates or updates service account for p and d deployments
func (childResource *BaseConfig) createPDServiceAccount(ctx context.Context, kubeClient client.Client) error {
	if childResource == nil || childResource.PDServiceAccount == nil {
		return nil
	}

	saToBeCreatedOrUpdated := corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{
		Name:      childResource.PDServiceAccount.Name,
		Namespace: childResource.PDServiceAccount.Namespace,
	}}

	op, err := controllerutil.CreateOrUpdate(ctx, kubeClient, &saToBeCreatedOrUpdated, func() error {
		saToBeCreatedOrUpdated.Labels = childResource.PDServiceAccount.Labels
		saToBeCreatedOrUpdated.OwnerReferences = childResource.PDServiceAccount.OwnerReferences
		saToBeCreatedOrUpdated.ImagePullSecrets = childResource.PDServiceAccount.ImagePullSecrets
		return nil
	})

	if err != nil {
		if op != controllerutil.OperationResultNone {
			log.FromContext(ctx).V(1).Error(err, "unable to create pd service account")
			return err
		}
	}

	return nil
}
