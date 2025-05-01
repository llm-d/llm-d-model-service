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

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	msv1alpha1 "github.com/neuralmagic/llm-d-model-service/api/v1alpha1"
)

// TODO: Decide where to requeue and where to requeueAfter

// ModelServiceReconciler reconciles a ModelService object
type ModelServiceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=llm-d.ai,resources=modelservices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=llm-d.ai,resources=modelservices/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=llm-d.ai,resources=modelservices/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/scale,verbs=update;patch
// +kubebuilder:rbac:groups=inference.networking.x-k8s.io,resources=inferencemodel,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=inference.networking.x-k8s.io,resources=inferencepool,verbs=get;list;watch;create;update;patch;delete

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.4/pkg/reconcile
func (r *ModelServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log.FromContext(ctx).Info("ModelService Reconciler started")

	// Step 1: Check that the model service is valid:
	// Get the current model service from API server
	// if it doesn't exist, return
	// if it is marked for deletion, return
	modelService := &msv1alpha1.ModelService{}
	if err := r.Get(ctx, req.NamespacedName, modelService); err != nil {
		if errors.IsNotFound(err) {
			log.FromContext(ctx).Info("ModelService not found.")
			return ctrl.Result{}, nil
		}
		log.FromContext(ctx).Error(err, "Unable to get ModelService")
		// should we requeue?  Neurops controller does.
		// Others do not always. See, for example https://github.com/kubernetes-sigs/kueue/blob/e9b35497ccf5b0534cce64a2a5f71c81b0926d6d/pkg/controller/core/workload_controller.go#L145
		// and https://github.com/kubernetes-sigs/gateway-api-inference-extension/blob/bd9ee36450d68fb4d0d8ac4f9be4db7d1ec4fee3/pkg/epp/controller/inferencepool_reconciler.go#L53
		// if we don't requeue there is a utility method we could use: client.IgnoreNotFound(err)
		return ctrl.Result{Requeue: true}, err
	} else if !modelService.DeletionTimestamp.IsZero() {
		log.FromContext(ctx).Info("ModelService is marked for deletion")
		return ctrl.Result{}, nil
	}

	log.FromContext(ctx).Info("attempting to get baseconfig object")
	// Step 2: Get the baseconfig object if it exists
	childResources, err := r.getChildResourcesFromConfigMap(ctx, modelService)
	if err != nil {
		return ctrl.Result{}, err
	}

	log.FromContext(ctx).Info("attempting to update configmaps")
	// Step: update configmaps
	if childResources.ConfigMaps != nil {
		childResources.updateConfigMaps(ctx, modelService, r.Scheme)
	}

	log.FromContext(ctx).Info("attempting to update prefill deployment")
	// Step 3: update the child resources
	// Idea: updates do the mergo merge
	if modelService.Spec.Prefill != nil || childResources.PrefillDeployment != nil {
		childResources.updatePDDeployment(ctx, modelService, PREFILL_ROLE, r.Scheme)
		childResources.updatePDService(ctx, modelService, PREFILL_ROLE, r.Scheme)
	}
	log.FromContext(ctx).Info("attempting to update decode deployment")
	if modelService.Spec.Decode != nil || childResources.DecodeDeployment != nil {
		childResources.updatePDDeployment(ctx, modelService, DECODE_ROLE, r.Scheme)
		childResources.updatePDService(ctx, modelService, DECODE_ROLE, r.Scheme)
	}

	log.FromContext(ctx).Info("attempting to update inference model")
	childResources.updateInferenceModel(ctx, modelService, r.Scheme)

	// and so on
	// TODO: update other objects here

	// TODO: Post-process for decoupled Scaling
	log.FromContext(ctx).Info("attempting to createOrUpdate child resources")
	err = childResources.createOrUpdate(ctx, r)

	// we will deal with status later
	// err = r.updateStatus(modelService)

	return ctrl.Result{}, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *ModelServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&msv1alpha1.ModelService{}).
		Named("modelservice").
		Owns(&msv1alpha1.ModelService{}).
		Watches(&appsv1.Deployment{}, handler.EnqueueRequestsFromMapFunc(r.deploymentMapFunc)).
		Complete(r)
}

// deploymentMapFunc maps deployments to ModelService owner
func (r *ModelServiceReconciler) deploymentMapFunc(ctx context.Context, obj client.Object) []reconcile.Request {
	deployment, ok := obj.(*appsv1.Deployment)
	if ok {
		ownerRefs := deployment.OwnerReferences

		for _, owner := range ownerRefs {
			ownerKind := owner.Kind
			ownerAPIVersion := owner.APIVersion

			if ownerKind == "ModelService" && ownerAPIVersion == "llm-d.ai/v1alpha1" {
				log.FromContext(ctx).Info("Found deployment owner", "deployment owner", owner.Name)
				return []reconcile.Request{{
					NamespacedName: types.NamespacedName{
						Namespace: deployment.Namespace,
						Name:      owner.Name,
					},
				}}
			}
		}

	}

	return []reconcile.Request{}
}
