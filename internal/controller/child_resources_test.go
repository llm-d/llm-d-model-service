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

	"k8s.io/apimachinery/pkg/api/errors"

	msv1alpha1 "github.com/neuralmagic/llm-d-model-service/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// tests to check if base config reading works ok
var _ = Describe("BaseConfig reader", func() {
	var (
		ctx        context.Context
		reconciler *ModelServiceReconciler
		msvc       *msv1alpha1.ModelService
		cm         *corev1.ConfigMap
		replicas   = int32(1)
	)

	BeforeEach(func() {
		ctx = context.Background()

		// Create test deployment YAML
		deployment := appsv1.Deployment{
			Spec: appsv1.DeploymentSpec{
				Replicas: &replicas,
			},
		}
		deployYaml, err := yaml.Marshal(deployment)
		Expect(err).To(BeNil())

		// Create ConfigMap with a deployment inside
		cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-base-config",
				Namespace: "default",
			},
			Data: map[string]string{
				"eppDeployment": string(deployYaml),
			},
		}

		// Create ModelService referencing the ConfigMap
		msvc = &msv1alpha1.ModelService{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-modelservice",
				Namespace: "default",
			},
			Spec: msv1alpha1.ModelServiceSpec{
				BaseConfigMapRef: &corev1.ObjectReference{
					Name: "test-base-config",
				},
				ModelArtifacts: msv1alpha1.ModelArtifacts{
					URI: "hf://facebook/opt-125m",
				},
			},
		}

		By("Creating the base config cm")
		Expect(k8sClient.Create(ctx, cm)).To(Succeed())

		By("Creating the msvc")
		Expect(k8sClient.Create(ctx, msvc)).To(Succeed())

		reconciler = &ModelServiceReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}
	})

	It("should correctly deserialize the eppDeployment from ConfigMap", func() {
		bc, err := reconciler.getChildResourcesFromConfigMap(ctx, msvc)
		Expect(err).To(BeNil())
		Expect(bc).ToNot(BeNil())
		Expect(bc.EPPDeployment).ToNot(BeNil())
		Expect(bc.EPPDeployment.Spec.Replicas).ToNot(BeNil())
		Expect(*bc.EPPDeployment.Spec.Replicas).To(Equal(int32(1)))
	})

	It("should continue to correctly deserialize the eppDeployment from ConfigMap with pvc prefix", func() {
		msvc.Spec.ModelArtifacts.URI = "pvc://my-pvc/path/to/opt-125m"
		bc, err := reconciler.getChildResourcesFromConfigMap(ctx, msvc)
		Expect(err).To(BeNil())
		Expect(bc).ToNot(BeNil())
		Expect(bc.EPPDeployment).ToNot(BeNil())
		Expect(bc.EPPDeployment.Spec.Replicas).ToNot(BeNil())
		Expect(*bc.EPPDeployment.Spec.Replicas).To(Equal(int32(1)))
	})

	It("should return nil if configmap ref is missing", func() {
		msvc.Spec.BaseConfigMapRef = nil
		bc, err := reconciler.getChildResourcesFromConfigMap(ctx, msvc)
		Expect(err).To(BeNil())
		Expect(bc.PrefillDeployment).To(BeNil())
		Expect(bc.DecodeDeployment).To(BeNil())
		Expect(bc.PrefillService).To(BeNil())
		Expect(bc.DecodeService).To(BeNil())
		Expect(bc.InferencePool).To(BeNil())
		Expect(bc.InferenceModel).To(BeNil())
		Expect(bc.EPPDeployment).To(BeNil())
		Expect(bc.EPPService).To(BeNil())
	})

	It("should error if the ConfigMap is missing", func() {
		msvc.Spec.BaseConfigMapRef.Name = "doesnotexist"
		bc, err := reconciler.getChildResourcesFromConfigMap(ctx, msvc)
		Expect(err).To(HaveOccurred())
		Expect(bc).To(BeNil())
	})

	AfterEach(func() {
		// Clean up resources after each test
		err := k8sClient.Delete(ctx, msvc)
		if err != nil && !errors.IsNotFound(err) {
			Fail(fmt.Sprintf("Failed to delete ModelService: %v", err))
		}

		err = k8sClient.Delete(ctx, cm)
		if err != nil && !errors.IsNotFound(err) {
			Fail(fmt.Sprintf("Failed to delete ConfigMap: %v", err))
		}
	})
})

// tests to check if templating works ok
var _ = Describe("BaseConfig reader", func() {
	var (
		ctx        context.Context
		reconciler *ModelServiceReconciler
		msvc       *msv1alpha1.ModelService
		cm         *corev1.ConfigMap
	)

	BeforeEach(func() {
		ctx = context.Background()

		// Be careful that there are no TAB characters in the string
		deployYamlStr := `metadata:
  name: mvsc-prefill
spec:
  template:
    spec:
      containers:
      - name: vllm
        command:
        - vllm
        - serve
        args:
        - '{{ .HFModelName }}'
        ports:
        - containerPort: {{ "portName" | getPort }}
`

		// Create ConfigMap with a deployment inside
		cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-base-config",
				Namespace: "default",
			},
			Data: map[string]string{
				"prefillDeployment": string(deployYamlStr),
			},
		}

		// Create ModelService referencing the ConfigMap
		msvc = &msv1alpha1.ModelService{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-modelservice",
				Namespace: "default",
			},
			Spec: msv1alpha1.ModelServiceSpec{
				BaseConfigMapRef: &corev1.ObjectReference{
					Name: "test-base-config",
				},
				ModelArtifacts: msv1alpha1.ModelArtifacts{
					URI: "hf://facebook/opt-125m",
				},
				Routing: msv1alpha1.Routing{
					Ports: []msv1alpha1.Port{
						{
							Name: "portName",
							Port: 9999,
						},
					},
				},
			},
		}

		By("Creating the base config cm")
		Expect(k8sClient.Create(ctx, cm)).To(Succeed())

		By("Creating the msvc")
		Expect(k8sClient.Create(ctx, msvc)).To(Succeed())

		reconciler = &ModelServiceReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}
	})

	It("should correctly interpolate container args", func() {
		bc, err := reconciler.getChildResourcesFromConfigMap(ctx, msvc)
		Expect(err).To(BeNil())
		Expect(bc).ToNot(BeNil())
		Expect(bc.PrefillDeployment).ToNot(BeNil())
		Expect(bc.PrefillDeployment.Spec.Template.Spec.Containers).ToNot(BeNil())
		c := bc.PrefillDeployment.Spec.Template.Spec.Containers[0]
		Expect(c.Args).ToNot(BeNil())
		Expect(c.Args[0]).To(Equal("facebook/opt-125m"))
	})

	It("should correctly interpolate containerPort", func() {
		bc, err := reconciler.getChildResourcesFromConfigMap(ctx, msvc)
		Expect(err).To(BeNil())
		Expect(bc).ToNot(BeNil())
		Expect(bc.PrefillDeployment).ToNot(BeNil())
		Expect(bc.PrefillDeployment.Spec.Template.Spec.Containers).ToNot(BeNil())
		c := bc.PrefillDeployment.Spec.Template.Spec.Containers[0]
		Expect(c.Ports).ToNot(BeEmpty())
		Expect(c.Ports[0].ContainerPort).To(Equal(int32(9999)))
	})

	AfterEach(func() {
		// Clean up resources after each test
		err := k8sClient.Delete(ctx, msvc)
		if err != nil && !errors.IsNotFound(err) {
			Fail(fmt.Sprintf("Failed to delete ModelService: %v", err))
		}

		err = k8sClient.Delete(ctx, cm)
		if err != nil && !errors.IsNotFound(err) {
			Fail(fmt.Sprintf("Failed to delete ConfigMap: %v", err))
		}
	})
})
