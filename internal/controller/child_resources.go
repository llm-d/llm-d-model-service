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

	"dario.cat/mergo"
	msv1alpha1 "github.com/neuralmagic/llm-d-model-service/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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

	return BaseConfigFromCM(&cm)
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

func (childResource *BaseConfig) UpdateChildResources(ctx context.Context, msvc *msv1alpha1.ModelService, scheme *runtime.Scheme) *BaseConfig {
	childResource = childResource.updateConfigMaps(ctx, msvc, scheme)
	if msvc.Spec.Prefill != nil {
		log.FromContext(ctx).Info("update Prefill Deployment and Service")
		childResource = childResource.updatePDDeployment(ctx, msvc, PREFILL_ROLE, scheme).
			updatePDService(ctx, msvc, PREFILL_ROLE, scheme)
	}
	if msvc.Spec.Decode != nil {
		log.FromContext(ctx).Info("update Decode Deployment and Service")
		childResource = childResource.updatePDDeployment(ctx, msvc, DECODE_ROLE, scheme).
			updatePDService(ctx, msvc, DECODE_ROLE, scheme)
	}
	if childResource.EPPDeployment != nil {
		log.FromContext(ctx).Info("update EPP Deployment and Service")
		childResource = childResource.updateEppDeployment(ctx, msvc, scheme)
	}
	if childResource.EPPService != nil {
		log.FromContext(ctx).Info("update EPP Deployment and Service")
		childResource = childResource.updateEppService(ctx, msvc, scheme)
	}
	if childResource.InferencePool != nil {
		log.FromContext(ctx).Info("update InferencePool")
		childResource = childResource.updateInferencePool(ctx, msvc, scheme)
	}
	if childResource.InferenceModel != nil {
		log.FromContext(ctx).Info("update EPP InferenceModel")
		childResource = childResource.updateInferenceModel(ctx, msvc, scheme)
	}

	return childResource
}

// updateConfigMaps updates childResource configmaps
func (childResource *BaseConfig) updateConfigMaps(ctx context.Context, msvc *msv1alpha1.ModelService, scheme *runtime.Scheme) *BaseConfig {
	for i := range childResource.ConfigMaps {
		// if there's no namespace, set it to msvc's
		if strings.TrimSpace(childResource.ConfigMaps[i].Namespace) == "" {
			childResource.ConfigMaps[i].Namespace = msvc.Namespace
		}
		// Note: there seems to be a controllerutil bug here ...
		// Setting owner ref before setting namespace seems problematic
		err := controllerutil.SetOwnerReference(msvc, &childResource.ConfigMaps[i], scheme)
		if err != nil {
			log.FromContext(ctx).Error(err, "unable to set owner reference")
		}
	}
	return childResource
}

// getCommonLabels that are applicable to all resources owned by msvc
func getCommonLabels(ctx context.Context, msvc *msv1alpha1.ModelService) map[string]string {
	// Step 3: Define object meta
	// Sanitize modelName into a valid label
	// TODO: this is not a good approach. Confirm with routing team on what label they need
	modelLabel, err := SanitizeModelName(msvc.Spec.Routing.ModelName)
	if err != nil {
		log.FromContext(ctx).Error(err, "unable to sanitize model name")
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

// updateInferenceModel uses msvc fields to update childResource inference model
func (childResources *BaseConfig) updateInferenceModel(ctx context.Context, msvc *msv1alpha1.ModelService, scheme *runtime.Scheme) *BaseConfig {
	// there's nothing to update
	if childResources.InferenceModel == nil {
		return childResources
	}

	im := childResources.InferenceModel

	im.Name = infModelName(msvc)
	im.Namespace = msvc.Namespace
	im.Labels = getCommonLabels(ctx, msvc)
	im.Spec.ModelName = msvc.Spec.Routing.ModelName
	im.Spec.PoolRef.Name = giev1alpha2.ObjectName(infPoolName(msvc))

	// Set owner reference for the merged service
	if err := controllerutil.SetOwnerReference(msvc, im, scheme); err != nil {
		log.FromContext(ctx).Error(err, "unable to set owner ref for inferencepool")
		return childResources
	}

	return childResources
}

// updatePDService uses msvc fields to update childResource P/D Service
func (childResource *BaseConfig) updatePDService(ctx context.Context, msvc *msv1alpha1.ModelService, role string, scheme *runtime.Scheme) *BaseConfig {

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

	// At this point, we are going to create a service for role
	// srcService contains ownerRef, name for service, and selector labels
	// srcService contains the stuff we want to override destService with
	srcService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      msvc.Name + "-service-" + role,
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
		log.FromContext(ctx).Error(err, "problem with service merge for "+role)
		return childResource
	}

	// Set owner reference for the merged service
	if err := controllerutil.SetOwnerReference(msvc, &destService, scheme); err != nil {
		log.FromContext(ctx).Error(err, "unable to set owner ref for service "+role)
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

// updatePDDeployment uses msvc fields to update childResource prefill deployment
func (childResource *BaseConfig) updatePDDeployment(ctx context.Context, msvc *msv1alpha1.ModelService, role string, scheme *runtime.Scheme) *BaseConfig {
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
	depl := &appsv1.Deployment{}

	// Step 3: Define object meta
	// Sanitize modelName into a valid label
	// TODO: this is not a good approach. Confirm with routing team on what label they need
	modelLabel, err := SanitizeModelName(msvc.Spec.Routing.ModelName)
	if err != nil {
		log.FromContext(ctx).Error(err, "unable to set owner reference")
	}

	labels := map[string]string{
		"llm-d.ai/inferenceServing": "true",
		"llm-d.ai/model":            modelLabel,
		"llm-d.ai/role":             role,
	}

	log.FromContext(ctx).Info("model service", "type meta", msvc.TypeMeta)

	depl.ObjectMeta = metav1.ObjectMeta{
		Name:      deploymentName(msvc, role),
		Namespace: msvc.Namespace,
		Labels:    labels,
	}
	err = controllerutil.SetOwnerReference(msvc, depl, scheme)
	if err != nil {
		log.FromContext(ctx).Error(err, "unable to set owner reference")
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
	depl.Spec.Template.Spec.Containers = msv1alpha1.ConvertToContainerSlice(pdSpec.Containers)
	depl.Spec.Template.Spec.InitContainers = msv1alpha1.ConvertToContainerSlice(pdSpec.InitContainers)

	// Step 5: populate nodeaffinity
	// AcceleratorTypes maybe nil... TODO: check
	na, err := pdSpec.AcceleratorTypes.ToNodeAffinity()
	if err == nil {
		depl.Spec.Template.Spec.Affinity = &corev1.Affinity{
			NodeAffinity: na,
		}
	} else {
		log.FromContext(ctx).Error(err, "unable to get node affinity")
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
			log.FromContext(ctx).Error(err, "unable to get volume")
		}
		if volume != nil {
			depl.Spec.Template.Spec.Volumes = append(depl.Spec.Template.Spec.Volumes, *volume)
		}
	}

	// Step 9: We need to figure out the volume mount story in
	// addition to the volume story above ...

	// Final step:
	// In Mergo merge...
	// We create a destination deployment object from baseconfig
	// We create a source deployment object from model service
	// We merge source into destination
	// We apply the merged destination
	if role == PREFILL_ROLE {
		log.FromContext(ctx).Info("mergo info", "role", role, "dst", childResource.PrefillDeployment, "src", depl)
		err = mergo.Merge(childResource.PrefillDeployment, depl, mergo.WithOverride, mergo.WithAppendSlice, mergo.WithTransformers(containerSliceTransformer{}))
		if err != nil {
			log.FromContext(ctx).Error(err, "mergo error")
		}
	}
	if role == DECODE_ROLE {
		log.FromContext(ctx).Info("mergo info", "role", role, "dst", childResource.DecodeDeployment, "src", depl)
		log.FromContext(ctx).Info("before mergo info -- details", "role", role, "dst.spec", childResource.DecodeDeployment.Spec, "src.spec", depl.Spec)
		err = mergo.Merge(childResource.DecodeDeployment, depl, mergo.WithOverride, mergo.WithAppendSlice, mergo.WithTransformers(containerSliceTransformer{}))
		log.FromContext(ctx).Info("after mergo info -- details", "role", role, "dst.spec", childResource.DecodeDeployment.Spec)
		if err != nil {
			log.FromContext(ctx).Error(err, "mergo error")
		}
	}

	return childResource
}

// createOrUpdate all the child resources
func (childResource *BaseConfig) createOrUpdate(ctx context.Context, r *ModelServiceReconciler) error {
	// create or update configmaps
	log.FromContext(ctx).Info("attempting to createOrUpdate configmaps")
	for i := range childResource.ConfigMaps {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      childResource.ConfigMaps[i].Name,
				Namespace: childResource.ConfigMaps[i].Namespace,
			}}

		_, err := controllerutil.CreateOrUpdate(ctx, r.Client, cm, func() error {
			cm.OwnerReferences = childResource.ConfigMaps[i].OwnerReferences
			cm.Labels = childResource.ConfigMaps[i].Labels
			cm.Data = childResource.ConfigMaps[i].Data
			return nil
		})
		if err != nil {
			log.FromContext(ctx).Error(err, "unable to create configmap")
		}
	}

	log.FromContext(ctx).Info("attempting to createOrUpdate prefill deployment")
	// create decode deployment
	desired := childResource.PrefillDeployment
	if desired != nil {
		deploy := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      desired.Name,
				Namespace: desired.Namespace,
			}}
		op, err := controllerutil.CreateOrUpdate(ctx, r.Client, deploy, func() error {
			deploy.OwnerReferences = desired.OwnerReferences
			deploy.Labels = desired.Labels
			deploy.Spec = desired.Spec
			return nil
		})
		log.FromContext(ctx).Info("from CreateOrUpdate", "op", op)
		if err != nil {
			log.FromContext(ctx).Error(err, "unable to create configmap")
		}
	}

	log.FromContext(ctx).Info("attempting to createOrUpdate decode deployment")
	log.FromContext(ctx).Info("printing decode info", "deployment", childResource.DecodeDeployment)

	// create decode deployment
	desired = childResource.DecodeDeployment
	if desired != nil {
		deploy := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      desired.Name,
				Namespace: desired.Namespace,
			}}
		op, err := controllerutil.CreateOrUpdate(ctx, r.Client, deploy, func() error {
			deploy.OwnerReferences = desired.OwnerReferences
			deploy.Labels = desired.Labels
			deploy.Spec = desired.Spec
			return nil
		})
		log.FromContext(ctx).Info("from CreateOrUpdate", "op", op)
		if err != nil {
			log.FromContext(ctx).Error(err, "unable to create configmap")
		}
	}

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
			log.FromContext(ctx).Error(err, "unable to create epp deployment")
		}
	}

	if childResource.EPPService != nil {
		err := childResource.createEppService(ctx, r.Client, *childResource.EPPService)
		if err != nil {
			log.FromContext(ctx).Error(err, "unable to create epp service")
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

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, infModelInCluster, func() error {
		infModelInCluster.Labels = childResource.InferenceModel.Labels
		infModelInCluster.OwnerReferences = childResource.InferenceModel.OwnerReferences
		infModelInCluster.Spec = childResource.InferenceModel.Spec
		return nil
	})

	if err != nil {
		log.FromContext(ctx).Error(err, "unable to create inference model")
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
		log.FromContext(ctx).Info("service for "+role, "service", service)

		svcInCluster := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      service.Name,
				Namespace: service.Namespace,
			}}

		_, err := controllerutil.CreateOrUpdate(ctx, r.Client, svcInCluster, func() error {
			svcInCluster.Labels = service.Labels
			svcInCluster.OwnerReferences = service.OwnerReferences
			svcInCluster.Spec = service.Spec
			return nil
		})

		if err != nil {
			log.FromContext(ctx).Error(err, "unable to create service for "+role)
		}
	}
}

func getInferencePoolLabels(labels map[string]string) map[giev1alpha2.LabelKey]giev1alpha2.LabelValue {
	m := make(map[giev1alpha2.LabelKey]giev1alpha2.LabelValue, len(labels))
	for k, v := range labels {
		m[giev1alpha2.LabelKey(k)] = giev1alpha2.LabelValue(v)
	}
	m["llm-d.ai/role"] = DECODE_ROLE
	return m
}

func (childResources *BaseConfig) updateEppDeployment(ctx context.Context, msvc *msv1alpha1.ModelService, scheme *runtime.Scheme) *BaseConfig {

	if childResources == nil || childResources.EPPDeployment == nil {
		return childResources
	}
	eppLabels := map[string]string{
		"llm-d.ai/epp": eppDeploymentName(msvc),
	}
	dest := *childResources.EPPDeployment

	src := &appsv1.Deployment{
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

	if err := mergo.Merge(&dest, src, mergo.WithOverride); err != nil {
		log.FromContext(ctx).Error(err, "problem with epp deployment merge")
		return childResources
	}

	// Set owner reference for the merged service
	if err := controllerutil.SetOwnerReference(msvc, &dest, scheme); err != nil {
		log.FromContext(ctx).Error(err, "unable to set owner ref for inferencepool")
		return childResources
	}
	log.FromContext(ctx).Info("deployment", "post-merge-label", dest.Labels, "post-merge-spec", dest.Spec)
	// Set the merged epp deployment in the child resource
	childResources.EPPDeployment = &dest

	return childResources
}

func (childResources *BaseConfig) updateEppService(ctx context.Context, msvc *msv1alpha1.ModelService, scheme *runtime.Scheme) *BaseConfig {
	if childResources == nil || childResources.EPPService == nil {
		return childResources
	}
	eppLabels := map[string]string{
		"llm-d.ai/epp": eppDeploymentName(msvc),
	}
	dest := *childResources.EPPService
	src := corev1.Service{ObjectMeta: metav1.ObjectMeta{
		Name:      eppServiceName(msvc),
		Namespace: msvc.Namespace,
		Labels:    eppLabels,
	}}

	src.Spec.Selector = eppLabels
	if err := mergo.Merge(&dest, src, mergo.WithOverride); err != nil {
		log.FromContext(ctx).Error(err, "problem with epp service merge")
		return childResources
	}
	if err := controllerutil.SetOwnerReference(msvc, &dest, scheme); err != nil {
		log.FromContext(ctx).Error(err, "unable to set owner ref for inferencepool")
		return childResources
	}

	// Set the merged epp service in the child resource
	childResources.EPPService = &dest
	return childResources
}

// updateInferencePool uses msvc fields to update childResource InferencePool resource.
func (childResources *BaseConfig) updateInferencePool(ctx context.Context, msvc *msv1alpha1.ModelService, scheme *runtime.Scheme) *BaseConfig {

	if childResources == nil || childResources.InferencePool == nil {
		return childResources
	}

	// Get dest Service
	dest := *childResources.InferencePool

	// At this point, we are going to create a service for role
	// srcService contains ownerRef, name for service, and selector labels
	// srcService contains the stuff we want to override destService with
	src := &giev1alpha2.InferencePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      infPoolName(msvc),
			Namespace: msvc.Namespace,
		},
		Spec: giev1alpha2.InferencePoolSpec{
			Selector: getInferencePoolLabels(getCommonLabels(ctx, msvc)),
			EndpointPickerConfig: giev1alpha2.EndpointPickerConfig{
				ExtensionRef: &giev1alpha2.Extension{
					ExtensionReference: giev1alpha2.ExtensionReference{
						Name: giev1alpha2.ObjectName(eppDeploymentName(msvc)),
					},
				},
			},
		},
	}

	// Mergo merge src into dst
	if err := mergo.Merge(&dest, src, mergo.WithOverride); err != nil {
		log.FromContext(ctx).Error(err, "problem with inferencepool merge")
		return childResources
	}

	// Set owner reference for the merged service
	if err := controllerutil.SetOwnerReference(msvc, &dest, scheme); err != nil {
		log.FromContext(ctx).Error(err, "unable to set owner ref for inferencepool")
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
	log.FromContext(ctx).Info("merged inf pool", "data", childResource.InferencePool)
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, inferencePoolInCluster, func() error {
		inferencePoolInCluster.Labels = childResource.InferenceModel.Labels
		inferencePoolInCluster.OwnerReferences = childResource.InferenceModel.OwnerReferences
		inferencePoolInCluster.Spec = childResource.InferencePool.Spec
		return nil
	})

	if err != nil {
		log.FromContext(ctx).Error(err, "unable to create inference pool")
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

	_, err := controllerutil.CreateOrUpdate(ctx, kubeClient, deploymentTobeCreatedOrUpdated, func() error {
		deploymentTobeCreatedOrUpdated.Labels = childResource.EPPDeployment.Labels
		deploymentTobeCreatedOrUpdated.OwnerReferences = childResource.EPPDeployment.OwnerReferences
		deploymentTobeCreatedOrUpdated.Spec = childResource.EPPDeployment.Spec
		return nil
	})

	if err != nil {
		log.FromContext(ctx).Error(err, "unable to create epp deployment from immutable base configmap ")
		return err
	}
	return nil
}

// createEppDeployment spawns epp service from immutable configmap
func (childResource *BaseConfig) createEppService(ctx context.Context, kubeClient client.Client, eppService corev1.Service) error {

	if childResource == nil || childResource.EPPService == nil {
		return nil
	}

	serviceTobeCreatedOrUpdated := corev1.Service{ObjectMeta: metav1.ObjectMeta{
		Name:      childResource.EPPService.Name,
		Namespace: childResource.EPPService.Namespace,
	}}

	_, err := controllerutil.CreateOrUpdate(ctx, kubeClient, &eppService, func() error {
		serviceTobeCreatedOrUpdated.Labels = childResource.EPPService.Labels
		serviceTobeCreatedOrUpdated.OwnerReferences = childResource.EPPService.OwnerReferences
		serviceTobeCreatedOrUpdated.Spec = childResource.EPPService.Spec
		return nil
	})

	if err != nil {
		log.FromContext(ctx).Error(err, "unable to create epp service from immutable base configmap ")
		return err
	}
	return nil
}
