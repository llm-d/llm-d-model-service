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
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	. "github.com/onsi/ginkgo/v2"

	msv1alpha1 "github.com/neuralmagic/llm-d-model-service/api/v1alpha1"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	giev1alpha2 "sigs.k8s.io/gateway-api-inference-extension/api/v1alpha2"
)

const namespace = "default"
const modelServiceName = "llama-3-1-8b-instruct"
const decodeWorkloadName = "llama-3-1-8b-instruct-decode"
const prefillWorkloadName = "llama-3-1-8b-instruct-prefill"

const configMapYAML = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: basic-base-conf
data:
  decodeDeployment: |
    spec:
      replicas: 2
      template:
        spec:
          containers:
          - name: llm
            command:
            - sleep
  decodeService: |
    spec:
      selector:
        app.kubernetes.io/name: decodeServiceLabelInBaseConfig
      ports:
        - protocol: TCP
          port: 80
          targetPort: 9376
  prefillDeployment: |
    spec:
      replicas: 2
      template:
        spec:
          containers:
          - name: llm
            command:
            - sleep
  prefillService: |
    spec:
      selector:
        app.kubernetes.io/name: prefillServiceLabelInBaseConfig
      ports:
        - protocol: TCP
          port: 80
          targetPort: 9376
  inferenceModel: |
    spec:
      criticality: Standard    
  inferencePool: |
    spec:
      targetPortNumber: 9376
  eppDeployment: |
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: epp
      namespace: default
    spec:
      replicas: 1
      template:
        spec:
          terminationGracePeriodSeconds: 130
          containers:
          - name: epp
            image: us-central1-docker.pkg.dev/k8s-staging-images/gateway-api-inference-extension/epp:main
            imagePullPolicy: Always
            args:
            - -poolName
            - my-pool-name
            - -poolNamespace
            - my-pool-namespace
            - -v
            - "4"
            - --zap-encoder
            - "json"
            - -grpcPort
            - "9002"
            - -grpcHealthPort
            - "9003"
            env:
            - name: USE_STREAMING
              value: "true"
            ports:
            - containerPort: 9002
            - containerPort: 9003
            - name: metrics
              containerPort: 9090
            livenessProbe:
              grpc:
                port: 9003
                service: inference-extension
              initialDelaySeconds: 5
              periodSeconds: 10
            readinessProbe:
              grpc:
                port: 9003
                service: inference-extension
              initialDelaySeconds: 5
              periodSeconds: 10 
  eppService: |
    apiVersion: v1
    kind: Service
    metadata:
      name: llm-llama3-8b-instruct-epp
      namespace: default
    spec:
      selector:
        app: llm-llama3-8b-instruct-epp
      ports:
      - protocol: TCP
        port: 9002
        targetPort: 9002
        appProtocol: http2
    type: ClusterIP
`

// TODO: change image
var imageName = "quay.io/redhattraining/hello-world-nginx"

var _ = Describe("ModelService Controller", func() {
	var rbacOptions *RBACOptions

	Context("When reconciling a resource", func() {
		//ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      modelServiceName,
			Namespace: namespace,
		}

		It("should spawn decode and prefill deployments with status", func() {
			ctx := context.Background()

			// Set RBAC options with EPPPullSecrets and PDPullSecrets
			rbacOptions = &RBACOptions{
				EPPPullSecrets: []string{"epp-pull-secret"},
				PDPullSecrets:  []string{"secret1", "secret2"},
				EPPClusterRole: "epp-cluster-role",
			}

			// Create dummy secrets required for the test
			for _, secretName := range append(rbacOptions.EPPPullSecrets, rbacOptions.PDPullSecrets...) {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretName,
						Namespace: namespace,
					},
				}
				Expect(k8sClient.Create(ctx, secret)).To(Succeed())
			}
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
						ModelName: modelServiceName,
					},
					DecoupleScaling: false,
					Decode: &msv1alpha1.PDSpec{
						Containers: []msv1alpha1.ContainerSpec{
							{
								Name:  "llm",
								Image: &imageName,
							},
						},
					},
					Prefill: &msv1alpha1.PDSpec{
						Containers: []msv1alpha1.ContainerSpec{
							{
								Name:  "llm",
								Image: &imageName,
							},
						},
					},
					BaseConfigMapRef: &corev1.ObjectReference{
						Name:      "basic-base-conf",
						Namespace: "default",
					},
				},
			}
			By("Creating the basic base configmap")
			var cm corev1.ConfigMap
			err := yaml.Unmarshal([]byte(configMapYAML), &cm)
			Expect(err).NotTo(HaveOccurred())

			cm.Namespace = "default"

			err = k8sClient.Create(ctx, &cm)
			Expect(err).NotTo(HaveOccurred())

			var fetched corev1.ConfigMap
			key := client.ObjectKey{Name: cm.Name, Namespace: cm.Namespace}
			err = k8sClient.Get(ctx, key, &fetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(fetched.Data).To(HaveKey("decodeDeployment"))

			By("Creating the ModelService CR")
			Expect(k8sClient.Create(ctx, modelService)).To(Succeed())

			By("Reconciling the ModelService")
			reconciler := &ModelServiceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				RBACOptions: RBACOptions{
					EPPPullSecrets: []string{"epp-pull-secret"},
					PDPullSecrets:  []string{"secret1", "secret2"},
					EPPClusterRole: "epp-cluster-role",
				},
			}
			_, err = reconciler.Reconcile(ctx, reconcile.Request{
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

			By("Eventually expecting the status to be populated with decode deployment name")
			Eventually(func() (string, error) {
				ms := &msv1alpha1.ModelService{}
				err := k8sClient.Get(ctx, client.ObjectKey{Name: modelServiceName, Namespace: namespace}, ms)
				if err != nil {
					return "", err
				}
				if ms.Status.DecodeDeploymentRef == nil {
					return "", nil
				}
				return *ms.Status.DecodeDeploymentRef, nil
			}, time.Second*5, time.Millisecond*500).Should(Equal(deploymentName(modelService, "decode")))

			By("Deleting the decode deployment")
			Expect(k8sClient.Delete(ctx, &decode)).To(Succeed())

			By("Ensuring the decode deployment is recreated")
			_, err = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: client.ObjectKey{
					Name:      modelServiceName,
					Namespace: namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: deploymentName(modelService, "decode"), Namespace: namespace}, &decode)
				return err == nil
			}, 5*time.Second, 500*time.Millisecond).Should(BeTrue())

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

			By("Deleting the prefill deployment")
			Expect(k8sClient.Delete(ctx, &prefill)).To(Succeed())

			By("Ensuring the prefill deployment is recreated")
			_, err = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: client.ObjectKey{
					Name:      modelServiceName,
					Namespace: namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: deploymentName(modelService, "prefill"), Namespace: namespace}, &prefill)
				return err == nil
			}, 5*time.Second, 500*time.Millisecond).Should(BeTrue())

			By("Eventually expecting the status to be populated with prefill deployment name")
			Eventually(func() (string, error) {
				ms := &msv1alpha1.ModelService{}
				err := k8sClient.Get(ctx, client.ObjectKey{Name: modelServiceName, Namespace: namespace}, ms)
				if err != nil {
					return "", err
				}
				fmt.Printf("the ms object status is %v", ms.Status)
				if ms.Status.PrefillDeploymentRef == nil {
					return "", nil
				}
				return *ms.Status.PrefillDeploymentRef, nil
			}, time.Second*5, time.Millisecond*500).Should(Equal(deploymentName(modelService, "prefill")))

			By("Checking if a PD SA was created")
			sa := corev1.ServiceAccount{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: pdServiceAccountName(modelService), Namespace: namespace}, &sa)
				return err == nil
			}, time.Second*5, time.Millisecond*500).Should(BeTrue())

			By("Checking if PD SA has the corret owner reference")
			Expect(sa.Name).To(Equal(pdServiceAccountName(modelService)))
			Expect(sa.OwnerReferences).ToNot(BeEmpty())

			By("Checking that prefill is using the correct SA")
			Expect(prefill.Spec.Template.Spec.ServiceAccountName).To(Equal(pdServiceAccountName(modelService)))

			actualSecrets := make([]string, len(sa.ImagePullSecrets))
			for i, s := range sa.ImagePullSecrets {
				actualSecrets[i] = s.Name
			}

			expectedSecrets := rbacOptions.PDPullSecrets
			Expect(actualSecrets).To(Equal(expectedSecrets))

			By("Deleting the pd service account")
			Expect(k8sClient.Delete(ctx, &sa)).To(Succeed())

			By("Ensuring the pd service is recreated")
			_, err = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: client.ObjectKey{
					Name:      modelServiceName,
					Namespace: namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: pdServiceAccountName(modelService), Namespace: namespace}, &sa)
				return err == nil
			}, 5*time.Second, 500*time.Millisecond).Should(BeTrue())

			By("Validating that the EPP ServiceAccount was created")
			eppSA := corev1.ServiceAccount{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: eppServiceAccountName(modelService), Namespace: namespace}, &eppSA)
				return err == nil
			}, time.Second*5, time.Millisecond*500).Should(BeTrue())

			actualSecrets = make([]string, len(eppSA.ImagePullSecrets))
			for i, s := range eppSA.ImagePullSecrets {
				actualSecrets[i] = s.Name
			}

			expectedSecrets = rbacOptions.EPPPullSecrets
			Expect(actualSecrets).To(Equal(expectedSecrets))

			By("Checking if epp SA has the correct owner reference")
			Expect(eppSA.Name).To(Equal(eppServiceAccountName(modelService)))
			Expect(eppSA.OwnerReferences).ToNot(BeEmpty())

			By("Deleting the epp service account")
			Expect(k8sClient.Delete(ctx, &eppSA)).To(Succeed())

			By("Ensuring the epp service account is recreated")
			_, err = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: client.ObjectKey{
					Name:      modelServiceName,
					Namespace: namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: eppServiceAccountName(modelService), Namespace: namespace}, &eppSA)
				return err == nil
			}, 5*time.Second, 500*time.Millisecond).Should(BeTrue())

			By("Checking if a epp RoleBinding was created")
			rolebinding := rbacv1.RoleBinding{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: eppRolebindingName(modelService), Namespace: namespace}, &rolebinding)
				return err == nil
			}, time.Second*5, time.Millisecond*500).Should(BeTrue())

			By("Deleting the epp RoleBinding")
			Expect(k8sClient.Delete(ctx, &rolebinding)).To(Succeed())

			By("Ensuring the epp RoleBinding is recreated")
			_, err = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: client.ObjectKey{
					Name:      modelServiceName,
					Namespace: namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: eppRolebindingName(modelService), Namespace: namespace}, &rolebinding)
				return err == nil
			}, 5*time.Second, 500*time.Millisecond).Should(BeTrue())

			By("Checking if epp RoleBinding has correct owner reference")
			Expect(rolebinding.Name).To(Equal(eppRolebindingName(modelService)))
			Expect(rolebinding.OwnerReferences).ToNot(BeEmpty())

			By("Checking if ModelService status has been updated")
			Eventually(func() string {
				updatedModelService := &msv1alpha1.ModelService{}
				err := k8sClient.Get(ctx, typeNamespacedName, updatedModelService)
				if err != nil || updatedModelService.Status.DecodeDeploymentRef == nil {
					return ""
				}
				return *updatedModelService.Status.DecodeDeploymentRef
			}, time.Second*5, time.Millisecond*500).Should(Equal(decodeWorkloadName))

			Eventually(func() string {
				updatedModelService := &msv1alpha1.ModelService{}
				err := k8sClient.Get(ctx, typeNamespacedName, updatedModelService)
				if err != nil || updatedModelService.Status.PrefillDeploymentRef == nil {
					return ""
				}
				return *updatedModelService.Status.PrefillDeploymentRef
			}, time.Second*5, time.Millisecond*500).Should(Equal(prefillWorkloadName))

			By("Eventually checking replica count on ModelService")

			Eventually(func() bool {
				updatedMSVC := &msv1alpha1.ModelService{}
				err := k8sClient.Get(ctx, typeNamespacedName, updatedMSVC)
				if err != nil {
					return false
				}

				if updatedMSVC.Status.PrefillReady != "0/1" {
					return false
				}
				if updatedMSVC.Status.DecodeReady != "0/1" {
					return false
				}
				if updatedMSVC.Status.EppReady != "0/1" {
					return false
				}
				if updatedMSVC.Status.PrefillAvailable != 0 {
					return false
				}

				if updatedMSVC.Status.EppAvailable != 0 {
					return false
				}

				return true
			}, time.Second*10, time.Millisecond*500).Should(BeTrue())

			By("Validating that the inferencepool was created")
			infPool := giev1alpha2.InferencePool{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: infPoolName(modelService), Namespace: namespace}, &infPool)
				return err == nil
			}, time.Second*5, time.Millisecond*500).Should(BeTrue())
			Expect(infPool.OwnerReferences).ToNot(BeEmpty())

			By("Deleting the inferencepool")
			Expect(k8sClient.Delete(ctx, &infPool)).To(Succeed())

			By("Ensuring the inferencepool is recreated")
			_, err = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: client.ObjectKey{
					Name:      modelServiceName,
					Namespace: namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: infPoolName(modelService), Namespace: namespace}, &infPool)
				return err == nil
			}, 5*time.Second, 500*time.Millisecond).Should(BeTrue())

			By("Validating that the inferencemodel was created")
			infModel := giev1alpha2.InferenceModel{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: infModelName(modelService), Namespace: namespace}, &infModel)
				return err == nil
			}, time.Second*5, time.Millisecond*500).Should(BeTrue())
			Expect(infModel.OwnerReferences).ToNot(BeEmpty())

			By("Deleting the inferencemodel")
			Expect(k8sClient.Delete(ctx, &infPool)).To(Succeed())

			By("Ensuring the inferencemodel is recreated")
			_, err = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: client.ObjectKey{
					Name:      modelServiceName,
					Namespace: namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: infModelName(modelService), Namespace: namespace}, &infModel)
				return err == nil
			}, 5*time.Second, 500*time.Millisecond).Should(BeTrue())
		})

		It("should reconcile a second ModelService with decoupled scaling", func() {
			By("Deleting the first ModelService")
			firstMS := &msv1alpha1.ModelService{}
			err := k8sClient.Get(ctx, client.ObjectKey{Name: modelServiceName, Namespace: namespace}, firstMS)
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Delete(ctx, firstMS)
			Expect(err).NotTo(HaveOccurred())

			ctx := context.Background()

			secondModelServiceName := "decoupled-modelservice"
			replicas := int32(10)
			secondModelService := &msv1alpha1.ModelService{
				TypeMeta: metav1.TypeMeta{
					APIVersion: msv1alpha1.GroupVersion.String(),
					Kind:       "ModelService",
				}, ObjectMeta: metav1.ObjectMeta{
					Name:      secondModelServiceName,
					Namespace: namespace,
				},
				Spec: msv1alpha1.ModelServiceSpec{
					ModelArtifacts: msv1alpha1.ModelArtifacts{
						URI: "pvc://llama-pvc/path/to/model",
					},
					Routing: msv1alpha1.Routing{
						ModelName: modelServiceName,
					},
					DecoupleScaling: true,
					Decode: &msv1alpha1.PDSpec{
						Replicas: &replicas,
						Containers: []msv1alpha1.ContainerSpec{
							{
								Name:  "llm",
								Image: &imageName,
							},
						},
					},
					Prefill: &msv1alpha1.PDSpec{
						Replicas: &replicas,
						Containers: []msv1alpha1.ContainerSpec{
							{
								Name:  "llm",
								Image: &imageName,
							},
						},
					},
					BaseConfigMapRef: &corev1.ObjectReference{
						Name:      "basic-base-conf",
						Namespace: "default",
					},
				},
			}

			By("Creating the second ModelService with decoupled scaling")
			Expect(k8sClient.Create(ctx, secondModelService)).To(Succeed())

			By("Reconciling the second ModelService")
			reconciler := &ModelServiceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				RBACOptions: RBACOptions{
					EPPPullSecrets: []string{"epp-pull-secret"},
					PDPullSecrets:  []string{"secret1", "secret2"},
					EPPClusterRole: "epp-cluster-role",
				},
			}
			_, err = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      secondModelServiceName,
					Namespace: namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the Decode and Prefill deployments have replicas set to 10")
			var decodeDeployment, prefillDeployment appsv1.Deployment

			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: deploymentName(secondModelService, "decode"), Namespace: namespace}, &decodeDeployment)
				return err == nil
			}, time.Second*5, time.Millisecond*500).Should(BeTrue())
			Expect(decodeDeployment.Spec.Replicas).Should(Equal(&replicas))

			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: deploymentName(secondModelService, "prefill"), Namespace: namespace}, &prefillDeployment)
				return err == nil
			}, time.Second*5, time.Millisecond*500).Should(BeTrue())
			Expect(prefillDeployment.Spec.Replicas).Should(Equal(&replicas))

			By("Updating the second ModelService to use 3 replicas")
			fetched := &msv1alpha1.ModelService{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{
				Name:      secondModelServiceName,
				Namespace: namespace,
			}, fetched)).To(Succeed())

			newReplicas := int32(3)
			fetched.Spec.Decode.Replicas = &newReplicas
			fetched.Spec.Prefill.Replicas = &newReplicas

			Expect(k8sClient.Update(ctx, fetched)).To(Succeed())

			By("Reconciling again after update")
			_, err = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      secondModelServiceName,
					Namespace: namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: deploymentName(secondModelService, "decode"), Namespace: namespace}, &decodeDeployment)
				return err == nil
			}, time.Second*5, time.Millisecond*500).Should(BeTrue())
			Expect(decodeDeployment.Spec.Replicas).Should(Equal(&replicas))

			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: deploymentName(secondModelService, "prefill"), Namespace: namespace}, &prefillDeployment)
				return err == nil
			}, time.Second*5, time.Millisecond*500).Should(BeTrue())
			Expect(prefillDeployment.Spec.Replicas).Should(Equal(&replicas))

		})

	})
})
