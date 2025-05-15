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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	msv1alpha1 "github.com/llm-d/llm-d-model-service/api/v1alpha1"
	giev1alpha2 "sigs.k8s.io/gateway-api-inference-extension/api/v1alpha2"
)

const HF_PREFIX string = "hf://"
const PVC_PREFIX string = "pvc://"

// TODO: Decide where to requeue and where to requeueAfter

// RBACOptions provides the options need to create service accounts and
// role binding during reconcile
type RBACOptions struct {
	// EPPPullSecrets contains names of epp pull secrets
	// the secrets objects are assumed to be in the controller namespace
	// these are pull secrets used by epp deployment created by the controller
	EPPPullSecrets []string
	// PDPullSecrets contains names of pd pull secrets;
	// the secrets objects are assumed to be in the controller namespace
	// these are pull secrets used by pd deployment created by the controller
	PDPullSecrets []string
	// EPPClusterRole name of the epp cluster role
	// this is a cluster role used in the rolebinding for the epp deployment
	EPPClusterRole string
}

// ModelServiceReconciler reconciles a ModelService object
type ModelServiceReconciler struct {
	RBACOptions RBACOptions
	client.Client
	Scheme *runtime.Scheme
}

// Context is intended to be use for interpolating template variables
// in BaseConfig
type TemplateVars struct {
	ModelServiceName      string `json:"modelServiceName,omitempty"`
	ModelServiceNamespace string `json:"modelServiceNamespace,omitempty"`
	ModelName             string `json:"modelName,omitempty"`
	HFModelName           string `json:"hfModelName,omitempty"`
	SanitizedModelName    string `json:"sanitizedModelName,omitempty"`
	ModelPath             string `json:"modelPath,omitempty"`
	MountedModelPath      string `json:"mountedModelPath,omitempty"`
	AuthSecretName        string `json:"authSecretName,omitempty"`
	EPPServiceName        string `json:"eppServiceName,omitempty"`
	EPPDeploymentName     string `json:"eppDeploymentName,omitempty"`
	PrefillDeploymentName string `json:"prefillDeploymentName,omitempty"`
	DecodeDeploymentName  string `json:"decodeDeploymentName,omitempty"`
	PrefillServiceName    string `json:"prefillServiceName,omitempty"`
	DecodeServiceName     string `json:"decodeServiceName,omitempty"`
	InferencePoolName     string `json:"inferencePoolName,omitempty"`
	InferenceModelName    string `json:"inferenceModelName,omitempty"`
}

// from populates the field values for TemplateVars from the model service
func (t *TemplateVars) from(ctx context.Context, msvc *msv1alpha1.ModelService) error {
	if t == nil {
		log.FromContext(ctx).V(1).Info("nil templatevars")
		return fmt.Errorf("nil templatevars")
	}

	// non empty template vars; attempt to populate
	if msvc == nil {
		log.FromContext(ctx).V(1).Info("empty modelservice; nothing to do")
		return nil
	}

	t.ModelServiceName = msvc.Name
	t.ModelServiceNamespace = msvc.Namespace
	t.EPPServiceName = eppServiceName(msvc)
	t.EPPDeploymentName = eppDeploymentName(msvc)
	t.PrefillDeploymentName = deploymentName(msvc, PREFILL_ROLE)
	t.DecodeDeploymentName = deploymentName(msvc, DECODE_ROLE)
	t.PrefillServiceName = sanitizeSvcName(msvc, PREFILL_ROLE)
	t.DecodeServiceName = sanitizeSvcName(msvc, DECODE_ROLE)
	t.InferencePoolName = infPoolName(msvc)
	t.InferenceModelName = infModelName(msvc)
	t.ModelName = msvc.Spec.Routing.ModelName
	t.SanitizedModelName = sanitizeModelName(msvc)

	if msvc.Spec.ModelArtifacts.AuthSecretName != nil {
		t.AuthSecretName = *msvc.Spec.ModelArtifacts.AuthSecretName
	}

	uri := msvc.Spec.ModelArtifacts.URI
	if strings.HasPrefix(uri, HF_PREFIX) {
		t.HFModelName = strings.TrimPrefix(uri, HF_PREFIX)
		t.ModelPath = t.HFModelName
	} else if strings.HasPrefix(uri, PVC_PREFIX) {
		tail := strings.TrimPrefix(uri, PVC_PREFIX)
		segments := strings.Split(tail, pathSep)
		t.ModelPath = strings.Join(segments[1:], pathSep)
	} else {
		err := fmt.Errorf("unsupported prefix")
		log.FromContext(ctx).V(1).Error(err, "cannot get template vars", "uri", uri)
		return err
	}

	mountedModelPath, err := mountedModelPath(msvc)
	if err != nil {
		return err
	}
	t.MountedModelPath = mountedModelPath

	return nil

}

type TemplateFuncs struct {
	funcMap template.FuncMap
}

// from populates the field values for TemplateVars from the model service
func (t *TemplateFuncs) from(ctx context.Context, msvc *msv1alpha1.ModelService) {

	fn := func(name string) int32 {
		for _, p := range msvc.Spec.Routing.Ports {
			if p.Name == name {
				return p.Port
			}
		}
		log.FromContext(ctx).V(1).Info("unknown port", "name", name, "ports", msvc.Spec.Routing.Ports)
		return -1
	}

	t.funcMap["getPort"] = fn
}

// +kubebuilder:rbac:groups=llm-d.ai,resources=modelservices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=llm-d.ai,resources=modelservices/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=llm-d.ai,resources=modelservices/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/scale,verbs=update;patch
// +kubebuilder:rbac:groups=inference.networking.x-k8s.io,resources=inferencemodels,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=inference.networking.x-k8s.io,resources=inferencepools,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=rolebindings,verbs=get;list;watch;create;update;patch;delete

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.4/pkg/reconcile
func (r *ModelServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log.FromContext(ctx).V(1).Info("ModelService Reconciler started")

	// Step 1: Check that the model service is valid:
	// Get the current model service from API server
	// if it doesn't exist, return
	// if it is marked for deletion, return
	modelService := &msv1alpha1.ModelService{}
	if err := r.Get(ctx, req.NamespacedName, modelService); err != nil {
		if errors.IsNotFound(err) {
			log.FromContext(ctx).V(1).Info("ModelService not found.")
			return ctrl.Result{}, nil
		}
		log.FromContext(ctx).V(1).Error(err, "Unable to get ModelService")
		// should we requeue?  Neurops controller does.
		// Others do not always. See, for example https://github.com/kubernetes-sigs/kueue/blob/e9b35497ccf5b0534cce64a2a5f71c81b0926d6d/pkg/controller/core/workload_controller.go#L145
		// and https://github.com/kubernetes-sigs/gateway-api-inference-extension/blob/bd9ee36450d68fb4d0d8ac4f9be4db7d1ec4fee3/pkg/epp/controller/inferencepool_reconciler.go#L53
		// if we don't requeue there is a utility method we could use: client.IgnoreNotFound(err)
		return ctrl.Result{Requeue: true}, err
	} else if !modelService.DeletionTimestamp.IsZero() {
		log.FromContext(ctx).V(1).Info("ModelService is marked for deletion")
		return ctrl.Result{}, nil
	}

	// Step 1.1: interpolate the modelService since it can include template vars
	interpolatedModelService, err := InterpolateModelService(ctx, modelService)
	if err != nil {
		return ctrl.Result{}, err
	}

	log.FromContext(ctx).V(1).Info("attempting to get baseconfig object")
	// Step 2: Get the interpolated baseconfig object if it exists
	interpolatedBaseConfig, err := r.getChildResourcesFromConfigMap(ctx, interpolatedModelService)
	if err != nil {
		return ctrl.Result{}, err
	}

	interpolatedBaseConfig = interpolatedBaseConfig.MergeChildResources(ctx, interpolatedModelService, r.Scheme, &r.RBACOptions)

	// TODO: Post-process for decoupled Scaling
	log.FromContext(ctx).V(1).Info("creating or updating child resources now")

	errs := interpolatedBaseConfig.invokeCreateOrUpdate(ctx, r, interpolatedModelService)

	if len(errs) > 0 {
		log.FromContext(ctx).Error(fmt.Errorf("problem creating %d child resources", len(errs)), "createOrUpdate failed")

		// TODO: requeue here?
		// Return the last error
		return ctrl.Result{}, errs[len(errs)-1]
	}

	//update status
	err = r.populateStatus(ctx, interpolatedModelService)
	if err != nil {
		// modelservice could be deleted before populating status
		// next reconcile cycle should ignore this request
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ModelServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&msv1alpha1.ModelService{}).
		Named("modelservice").
		Owns(&msv1alpha1.ModelService{}).
		Watches(&appsv1.Deployment{}, handler.EnqueueRequestsFromMapFunc(r.deploymentMapFunc)).
		Watches(&corev1.Service{}, handler.EnqueueRequestsFromMapFunc(r.serviceMapFunc)).
		Watches(&rbacv1.RoleBinding{}, handler.EnqueueRequestsFromMapFunc(r.roleBindingMapFunc)).
		Watches(&corev1.ConfigMap{}, handler.EnqueueRequestsFromMapFunc(r.configMapMapFunc)).
		Watches(&giev1alpha2.InferenceModel{}, handler.EnqueueRequestsFromMapFunc(r.inferenceModelMapFunc)).
		Watches(&giev1alpha2.InferencePool{}, handler.EnqueueRequestsFromMapFunc(r.inferencePoolMapFunc)).
		Watches(&corev1.ServiceAccount{}, handler.EnqueueRequestsFromMapFunc(r.serviceAccountMapFunc)).
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
				log.FromContext(ctx).V(1).Info("Found deployment owner", "deployment owner", owner.Name)
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

func (r *ModelServiceReconciler) populateStatus(ctx context.Context, msvc *msv1alpha1.ModelService) error {
	var conditions []metav1.Condition
	totalReady, expected := int32(0), int32(0)
	original := msvc.DeepCopy()
	baseConfig, err := r.getChildResourcesFromConfigMap(ctx, msvc)
	if err != nil {
		return err
	}
	infModelName := infModelName(msvc)
	msvc.Status.InferenceModelRef = &infModelName

	infPoolName := infPoolName(msvc)
	msvc.Status.InferencePoolRef = &infPoolName

	pdSA := pdServiceAccountName(msvc)
	msvc.Status.PDServiceAccountRef = &pdSA

	eppSA := eppServiceAccountName(msvc)
	msvc.Status.PDServiceAccountRef = &eppSA

	eppRoleBinding := eppRolebindingName(msvc)
	msvc.Status.EppRoleBinding = &eppRoleBinding

	var configMapNames []string
	for _, v := range baseConfig.ConfigMaps {
		configMapNames = append(configMapNames, v.Name)
	}
	msvc.Status.ConfigMapNames = configMapNames

	if msvc.Spec.Prefill != nil {
		prefillDeploymentName := deploymentName(msvc, PREFILL_ROLE)
		msvc.Status.PrefillDeploymentRef = &prefillDeploymentName
		prefillDeploymentFromCluster := &appsv1.Deployment{}
		// Mirror conditions with "Prefill" prefix
		err := r.Client.Get(ctx, client.ObjectKey{Name: prefillDeploymentName, Namespace: msvc.Namespace}, prefillDeploymentFromCluster)
		if err != nil {
			log.FromContext(ctx).Error(err, "unable to get prefill deployment")
			conditions = append(conditions, metav1.Condition{
				Type:               "PrefillDeploymentAvailable",
				Status:             metav1.ConditionFalse,
				Reason:             "GetFailed",
				Message:            fmt.Sprintf("Failed to fetch Prefill Deployment: %v", err),
				LastTransitionTime: metav1.Now(),
			})
		} else {
			totalReady = prefillDeploymentFromCluster.Status.ReadyReplicas
			totalAvailable := prefillDeploymentFromCluster.Status.AvailableReplicas
			expected = *prefillDeploymentFromCluster.Spec.Replicas
			msvc.Status.PrefillReady = fmt.Sprintf("%d/%d", totalReady, expected)
			msvc.Status.PrefillAvailable = totalAvailable

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
	}

	if msvc.Spec.Decode != nil {
		decodeDeploymentName := deploymentName(msvc, DECODE_ROLE)
		msvc.Status.DecodeDeploymentRef = &decodeDeploymentName
		decodeDeploymentFromCluster := &appsv1.Deployment{}
		err := r.Client.Get(ctx, client.ObjectKey{Name: decodeDeploymentName, Namespace: msvc.Namespace}, decodeDeploymentFromCluster)
		if err != nil {
			log.FromContext(ctx).Error(err, "unable to get prefill deployment")
			conditions = append(conditions, metav1.Condition{
				Type:               "DecodeDeploymentAvailable",
				Status:             metav1.ConditionFalse,
				Reason:             "GetFailed",
				Message:            fmt.Sprintf("Failed to fetch Decode Deployment: %v", err),
				LastTransitionTime: metav1.Now(),
			})
		} else {
			totalReady := decodeDeploymentFromCluster.Status.ReadyReplicas
			totalAvailable := decodeDeploymentFromCluster.Status.AvailableReplicas
			expected := *decodeDeploymentFromCluster.Spec.Replicas
			msvc.Status.DecodeReady = fmt.Sprintf("%d/%d", totalReady, expected)
			msvc.Status.DecodeAvailable = totalAvailable

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
	}

	if baseConfig.EPPDeployment != nil {
		eppName := eppDeploymentName(msvc)
		msvc.Status.EppDeploymentRef = &eppName
		eppDeploymentFromCluster := &appsv1.Deployment{}
		err := r.Client.Get(ctx, client.ObjectKey{Name: eppName, Namespace: msvc.Namespace}, eppDeploymentFromCluster)
		if err != nil {
			log.FromContext(ctx).Error(err, "unable to get Epp deployment")
			conditions = append(conditions, metav1.Condition{
				Type:               "EppDeploymentAvailable",
				Status:             metav1.ConditionFalse,
				Reason:             "GetFailed",
				Message:            fmt.Sprintf("Failed to fetch Epp Deployment: %v", err),
				LastTransitionTime: metav1.Now(),
			})
		} else {
			totalReady := eppDeploymentFromCluster.Status.ReadyReplicas
			totalAvailable := eppDeploymentFromCluster.Status.AvailableReplicas
			expected := *eppDeploymentFromCluster.Spec.Replicas
			msvc.Status.EppReady = fmt.Sprintf("%d/%d", totalReady, expected)
			msvc.Status.EppAvailable = totalAvailable

			// Mirror conditions with "Epp" prefix
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
	}

	msvc.Status.Conditions = conditions

	latest := &msv1alpha1.ModelService{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: msvc.Name, Namespace: msvc.Namespace}, latest); err != nil {
		if errors.IsNotFound(err) {
			log.FromContext(ctx).Info("ModelService no longer exists, skipping status update")
			return nil
		}
		return err
	}
	latest.Status = msvc.Status
	if !equality.Semantic.DeepEqual(&original.Status, &latest.Status) {
		if err := r.Status().Update(ctx, latest); err != nil {
			if errors.IsNotFound(err) {
				log.FromContext(ctx).Info("ModelService no longer exists, skipping status update")
				return nil
			}
			return err
		}
	}

	return nil
}

func (r *ModelServiceReconciler) serviceMapFunc(ctx context.Context, obj client.Object) []reconcile.Request {
	svc, ok := obj.(*corev1.Service)
	if !ok {
		return nil
	}
	shouldReturn, result := requeueMsvcReq(ctx, svc)
	if shouldReturn {
		return result
	}
	return nil
}

func (r *ModelServiceReconciler) serviceAccountMapFunc(ctx context.Context, obj client.Object) []reconcile.Request {
	sa, ok := obj.(*corev1.ServiceAccount)
	if !ok {
		return nil
	}
	shouldReturn, result := requeueMsvcReq(ctx, sa)
	if shouldReturn {
		return result
	}
	return nil
}

func (r *ModelServiceReconciler) roleBindingMapFunc(ctx context.Context, obj client.Object) []reconcile.Request {
	rb, ok := obj.(*rbacv1.RoleBinding)
	if !ok {
		return nil
	}
	shouldReturn, result := requeueMsvcReq(ctx, rb)
	if shouldReturn {
		return result
	}
	return nil
}

func (r *ModelServiceReconciler) configMapMapFunc(ctx context.Context, obj client.Object) []reconcile.Request {
	cm, ok := obj.(*corev1.ConfigMap)
	if !ok {
		return nil
	}
	shouldReturn, result := requeueMsvcReq(ctx, cm)
	if shouldReturn {
		return result
	}
	return nil
}

func (r *ModelServiceReconciler) inferencePoolMapFunc(ctx context.Context, obj client.Object) []reconcile.Request {
	ip, ok := obj.(*giev1alpha2.InferencePool)
	if !ok {
		return nil
	}
	shouldReturn, result := requeueMsvcReq(ctx, ip)
	if shouldReturn {
		return result
	}
	return nil
}

func (r *ModelServiceReconciler) inferenceModelMapFunc(ctx context.Context, obj client.Object) []reconcile.Request {
	im, ok := obj.(*giev1alpha2.InferenceModel)
	if !ok {
		return nil
	}
	shouldReturn, result := requeueMsvcReq(ctx, im)
	if shouldReturn {
		return result
	}
	return nil
}

func requeueMsvcReq(ctx context.Context, obj client.Object) (bool, []reconcile.Request) {
	for _, owner := range obj.GetOwnerReferences() {
		if owner.Kind == "ModelService" && owner.APIVersion == "llm-d.ai/v1alpha1" {
			log.FromContext(ctx).V(1).Info("Found owner", "object owner", owner.Name)
			return true, []reconcile.Request{{
				NamespacedName: types.NamespacedName{
					Namespace: obj.GetNamespace(),
					Name:      owner.Name,
				},
			}}
		}
	}
	return false, nil
}
