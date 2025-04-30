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

/*
import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)
*/

// const namespace = "default"
// const modelServiceName = "llama-3-1-8b-instruct"
// const decodeWorkloadName = "llama-3-1-8b-instruct-decode"
// const prefillWorkloadName = "llama-3-1-8b-instruct-prefill"

// // TODO: change image
// const imageName = "quay.io/redhattraining/hello-world-nginx"

// this test needs to change... commenting for now ...
/*
var _ = Describe("ModelService Controller", func() {
	Context("When reconciling a resource", func() {
		//ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      modelServiceName,
			Namespace: namespace,
		}

		It("should spawn decode and prefill deployments with status", func() {
			ctx := context.Background()
			modelService := &msv1alpha1.ModelService{
				TypeMeta: metav1.TypeMeta{
					APIVersion: msv1alpha1.GroupVersion.String(),
					Kind:       "ModelService",
				}, ObjectMeta: metav1.ObjectMeta{
					Name:      modelServiceName,
					Namespace: namespace,
				},
				Spec: msv1alpha1.ModelServiceSpec{
					ModelArtifacts: msv1alpha1.ModelArtifacts{
						URI: "pvc://llama-pvc/path/to/model",
					},
					Routing: msv1alpha1.Routing{
						InferencePoolRef: modelServiceName,
					},
					DecoupleScaling: false,
					Decode: msv1alpha1.PDSpec{
						VLLMContainer: &corev1.Container{
							Name:  "vllm",
							Image: imageName,
						},
						VLLMProxyContainer: &corev1.Container{
							Name:  "vllm-proxy",
							Image: imageName,
						},
					},
					Prefill: &msv1alpha1.PDSpec{
						VLLMContainer: &corev1.Container{
							Name:  "vllm",
							Image: imageName,
						},
						VLLMProxyContainer: &corev1.Container{
							Name:  "vllm-proxy",
							Image: imageName,
						},
					},
				},
			}

			By("Creating the ModelService CR")
			Expect(k8sClient.Create(ctx, modelService)).To(Succeed())

			By("Reconciling the ModelService")
			reconciler := &ModelServiceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      modelService.Name,
					Namespace: modelService.Namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking if Decode deployment was created")
			var decode appsv1.Deployment
			var prefill appsv1.Deployment
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: decodeWorkloadName, Namespace: namespace}, &decode)
				return err == nil
			}, time.Second*5, time.Millisecond*500).Should(BeTrue())

			By("Checking if Decode deployment has correct owner reference")
			Expect(decode.OwnerReferences).ToNot(BeEmpty())
			ownerRef := decode.OwnerReferences[0]
			Expect(ownerRef.Kind).To(Equal("ModelService"))
			Expect(ownerRef.Name).To(Equal(modelService.Name))
			Expect(ownerRef.APIVersion).To(Equal("llm-d.ai/v1alpha1"))
			updated := &msv1alpha1.ModelService{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, updated)).To(Succeed())
			updated.Status.DecodeDeploymentRef = ptr.To(decode.Name)
			err = k8sClient.Status().Update(ctx, updated)
			Expect(err).NotTo(HaveOccurred())

			By("Checking if prefill deployment was created")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: prefillWorkloadName, Namespace: namespace}, &prefill)
				return err == nil
			}, time.Second*5, time.Millisecond*500).Should(BeTrue())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: prefillWorkloadName, Namespace: namespace}, &prefill)
				return err == nil
			}, time.Second*5, time.Second*5).Should(BeTrue())

			By("Checking if Prefill deployment has correct owner reference")
			Expect(prefill.OwnerReferences).ToNot(BeEmpty())
			Expect(ownerRef.Kind).To(Equal("ModelService"))
			Expect(ownerRef.Name).To(Equal(modelService.Name))
			Expect(ownerRef.APIVersion).To(Equal("llm-d.ai/v1alpha1"))
			updated = &msv1alpha1.ModelService{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, updated)).To(Succeed())
			updated.Status.PrefillDeploymentRef = ptr.To(prefill.Name)
			err = k8sClient.Status().Update(ctx, updated)
			Expect(err).NotTo(HaveOccurred())

			By("Checking if ModelService status has been updated")
			Eventually(func() string {
				updatedModelService := &msv1alpha1.ModelService{}
				err := k8sClient.Get(ctx, typeNamespacedName, updatedModelService)
				if err != nil || updatedModelService.Status.DecodeDeploymentRef == nil {
					fmt.Printf("%v", updatedModelService)
					return ""
				}
				return *updatedModelService.Status.DecodeDeploymentRef
			}, time.Second*5, time.Millisecond*500).Should(Equal(decodeWorkloadName))

			Eventually(func() string {
				updatedModelService := &msv1alpha1.ModelService{}
				err := k8sClient.Get(ctx, typeNamespacedName, updatedModelService)
				if err != nil || updatedModelService.Status.PrefillDeploymentRef == nil {
					fmt.Printf("%v", updatedModelService)
					return ""
				}
				return *updatedModelService.Status.PrefillDeploymentRef
			}, time.Second*5, time.Millisecond*500).Should(Equal(prefillWorkloadName))
		})

	})
})
*/
