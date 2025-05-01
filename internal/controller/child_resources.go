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
	msv1alpha1 "github.com/neuralmagic/modelservice/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
		childResource = childResource.updatePDDeployment(ctx, msvc, PREFILL_ROLE, scheme)
		// TBD update prefill service
	}
	if msvc.Spec.Decode != nil {
		childResource = childResource.updatePDDeployment(ctx, msvc, DECODE_ROLE, scheme)
		// update decode service
	}
	if childResource.EPPDeployment != nil {
		log.FromContext(ctx).Info("update EPP Deployment and Service")
		// TBD update epp deployment, service
	}
	if childResource.InferencePool != nil {
		log.FromContext(ctx).Info("update InferencePool")
		// TBD update inference pool
	}
	if childResource.InferenceModel != nil {
		log.FromContext(ctx).Info("update EPP InferenceModel")
		// TBD update inference model
	}

	return childResource
}

// updateConfigMaps updates childResource configmaps
func (childResource *BaseConfig) updateConfigMaps(ctx context.Context, msvc *msv1alpha1.ModelService, scheme *runtime.Scheme) *BaseConfig {
	for i := range childResource.ConfigMaps {
		err := controllerutil.SetOwnerReference(msvc, &childResource.ConfigMaps[i], scheme)
		if err != nil {
			log.FromContext(ctx).Error(err, "unable to set owner reference")
		}
	}
	return childResource
}

func getPodLabels(ctx context.Context, msvc *msv1alpha1.ModelService, role string) map[string]string {
	// Step 3: Define object meta
	// Sanitize modelName into a valid label
	// TODO: this is not a good approach. Confirm with routing team on what label they need
	modelLabel, err := SanitizeModelName(msvc.Spec.Routing.ModelName)
	if err != nil {
		log.FromContext(ctx).Error(err, "unable to set owner reference")
	}

	return map[string]string{
		"llm-d.ai/inferenceServing": "true",
		"llm-d.ai/model":            modelLabel,
		"llm-d.ai/role":             role,
	}
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
		_, err := controllerutil.CreateOrUpdate(ctx, r.Client, &childResource.ConfigMaps[i], func() error {
			return nil
		})
		if err != nil {
			log.FromContext(ctx).Error(err, "unable to create configmap")
		}
	}

	log.FromContext(ctx).Info("attempting to createOrUpdate prefill deployment")
	// create prefill deployment
	if childResource.PrefillDeployment != nil {
		op, err := controllerutil.CreateOrUpdate(ctx, r.Client, childResource.PrefillDeployment, func() error {
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
	desired := childResource.DecodeDeployment
	if desired != nil {
		deploy := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:        desired.Name,
				Namespace:   desired.Namespace,
				Labels:      desired.Labels,
				Annotations: desired.Annotations,
			}}
		op, err := controllerutil.CreateOrUpdate(ctx, r.Client, deploy, func() error {
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

	return nil
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
				Name:            service.Name,
				Namespace:       service.Namespace,
				Labels:          service.Labels,
				Annotations:     service.Annotations,
				OwnerReferences: service.OwnerReferences,
			}}

		_, err := controllerutil.CreateOrUpdate(ctx, r.Client, svcInCluster, func() error {
			svcInCluster.Spec = service.Spec
			return nil
		})

		if err != nil {
			log.FromContext(ctx).Error(err, "unable to create service for "+role)
		}
	}
}
