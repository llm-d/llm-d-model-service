package controller

// import (
// 	. "github.com/onsi/ginkgo/v2"
// 	. "github.com/onsi/gomega"
// )

/*

// tests to check if base config reading works ok
var _ = Describe("BaseConfig reader", func() {
	var (
		ctx        context.Context
		reconciler *ModelServiceReconciler
		msvc       *msv1alpha1.ModelService
		cm         *corev1.ConfigMap
	)

	BeforeAll(func() {
		ctx = context.Background()

		// Create test deployment YAML
		deployment := appsv1.Deployment{
			Spec: appsv1.DeploymentSpec{
				Replicas: pointerToInt32(1),
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

	It("should return nil if configmap ref is missing", func() {
		msvc.Spec.BaseConfigMapRef = nil
		bc, err := reconciler.getChildResourcesFromConfigMap(ctx, msvc)
		Expect(err).To(BeNil())
		Expect(bc).To(BeNil())
	})

	It("should error if the ConfigMap is missing", func() {
		msvc.Spec.BaseConfigMapRef.Name = "doesnotexist"
		bc, err := reconciler.getChildResourcesFromConfigMap(ctx, msvc)
		Expect(err).To(HaveOccurred())
		Expect(bc).To(BeNil())
	})

})

func pointerToInt32(i int32) *int32 {
	return &i
}
*/
