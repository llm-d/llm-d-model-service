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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	. "github.com/onsi/ginkgo/v2"

	msv1alpha1 "github.com/neuralmagic/llm-d-model-service/api/v1alpha1"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// this test needs to change... commenting for now ...

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
