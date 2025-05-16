package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	msv1alpha1 "github.com/llm-d/llm-d-model-service/api/v1alpha1"
	"github.com/llm-d/llm-d-model-service/test/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	giev1alpha2 "sigs.k8s.io/gateway-api-inference-extension/api/v1alpha2"
	"sigs.k8s.io/yaml"
)

// namespace where the project is deployed in
const namespace = "modelservice-system"

// serviceAccountName created for the project
const serviceAccountName = "modelservice-controller-manager"

// metricsServiceName is the name of the metrics service of the project
const metricsServiceName = "modelservice-controller-manager-metrics-service"

// metricsRoleBindingName is the name of the RBAC that will be created to allow get the metrics data
const metricsRoleBindingName = "modelservice-metrics-binding"

const (
	modelServiceName    = "llama-3-1-8b-instruct"
	decodeWorkloadName  = "llama-3-1-8b-instruct-decode"
	prefillWorkloadName = "llama-3-1-8b-instruct-prefill"
)

const configMapYAML = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: basic-base-conf
data:
  configMaps: |
    - metadata:
        name: cm1
      data: 
        key1: value1
        key2: value2
    - metadata:
        name: cm2
      data:
        key3: value3
        key4: value4
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
            image: busybox
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

var imageName = "ghcr.io/linuxserver/nginx:1.26.3-r0-ls319"
var _ = Describe("Manager", Ordered, func() {
	var controllerPodName string

	// Before running the tests, set up the environment by creating the namespace,
	// enforce the restricted security policy to the namespace, installing CRDs,
	// and deploying the controller.
	BeforeAll(func() {
		By("creating manager namespace")
		cmd := exec.Command("kubectl", "create", "ns", namespace)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")

		By("labeling the namespace to enforce the restricted security policy")
		cmd = exec.Command("kubectl", "label", "--overwrite", "ns", namespace,
			"pod-security.kubernetes.io/enforce=restricted")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to label namespace with restricted policy")

		By("installing CRDs")
		cmd = exec.Command("make", "install")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to install CRDs")

		By("installing external CRDs")

		cmd = exec.Command("kubectl", "apply", "-f",
			"https://github.com/kubernetes-sigs/gateway-api-inference-extension/releases/download/v0.3.0/manifests.yaml")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to install CRDs")

		By("installing rolebinding for Epp")

		cmd = exec.Command("kubectl", "apply", "-f",
			"https://raw.githubusercontent.com/llm-d/gateway-api-inference-extension/refs/heads/dev/config/manifests/inferencepool-resources.yaml")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to install rolebinding for Epp")

		By("deploying the controller-manager")
		cmd = exec.Command("make", "dev-deploy", fmt.Sprintf("IMG=%s", projectImage), fmt.Sprintf("EPP_CLUSTERROLE=%s", "pod-read"), fmt.Sprintf("IMAGE_PULL_POLICY=%s", "Never"))
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to deploy the controller-manager")
	})

	// After all tests have been executed, clean up by undeploying the controller, uninstalling CRDs,
	// and deleting the namespace.
	AfterAll(func() {
		By("cleaning up the curl pod for metrics")
		cmd := exec.Command("kubectl", "delete", "pod", "curl-metrics", "-n", namespace)
		_, _ = utils.Run(cmd)

		By("undeploying the controller-manager")
		cmd = exec.Command("make", "undeploy")
		_, _ = utils.Run(cmd)

		By("uninstalling CRDs")
		cmd = exec.Command("make", "uninstall")
		_, _ = utils.Run(cmd)

		By("removing manager namespace")
		cmd = exec.Command("kubectl", "delete", "ns", namespace)
		_, _ = utils.Run(cmd)
	})

	// After each test, check for failures and collect logs, events,
	// and pod descriptions for debugging.
	AfterEach(func() {
		specReport := CurrentSpecReport()
		if specReport.Failed() {
			By("Fetching controller manager pod logs")
			cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
			controllerLogs, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Controller logs:\n %s", controllerLogs)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Controller logs: %s", err)
			}

			By("Fetching Kubernetes events")
			cmd = exec.Command("kubectl", "get", "events", "-n", namespace, "--sort-by=.lastTimestamp")
			eventsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Kubernetes events:\n%s", eventsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Kubernetes events: %s", err)
			}

			By("Fetching curl-metrics logs")
			cmd = exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
			metricsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Metrics logs:\n %s", metricsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get curl-metrics logs: %s", err)
			}

			By("Fetching controller manager pod description")
			cmd = exec.Command("kubectl", "describe", "pod", controllerPodName, "-n", namespace)
			podDescription, err := utils.Run(cmd)
			if err == nil {
				fmt.Println("Pod description:\n", podDescription)
			} else {
				fmt.Println("Failed to describe controller pod")
			}
		}
	})

	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	Context("Manager", func() {
		It("should run successfully", func() {
			By("validating that the controller-manager pod is running as expected")
			verifyControllerUp := func(g Gomega) {
				// Get the name of the controller-manager pod
				cmd := exec.Command("kubectl", "get",
					"pods", "-l", "control-plane=controller-manager",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", namespace,
				)

				podOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve controller-manager pod information")
				podNames := utils.GetNonEmptyLines(podOutput)
				g.Expect(podNames).To(HaveLen(1), "expected 1 controller pod running")
				controllerPodName = podNames[0]
				g.Expect(controllerPodName).To(ContainSubstring("controller-manager"))

				// Validate the pod's status
				cmd = exec.Command("kubectl", "get",
					"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
					"-n", namespace,
				)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Running"), "Incorrect controller-manager pod status")
			}
			Eventually(verifyControllerUp).Should(Succeed())
		})

		It("should ensure the metrics endpoint is serving metrics", func() {
			By("creating a ClusterRoleBinding for the service account to allow access to metrics")
			cmd := exec.Command("kubectl", "create", "clusterrolebinding", metricsRoleBindingName,
				"--clusterrole=modelservice-metrics-reader",
				fmt.Sprintf("--serviceaccount=%s:%s", namespace, serviceAccountName),
			)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create ClusterRoleBinding")

			By("validating that the metrics service is available")
			cmd = exec.Command("kubectl", "get", "service", metricsServiceName, "-n", namespace)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Metrics service should exist")

			By("getting the service account token")
			token, err := serviceAccountToken()
			Expect(err).NotTo(HaveOccurred())
			Expect(token).NotTo(BeEmpty())

			By("waiting for the metrics endpoint to be ready")
			verifyMetricsEndpointReady := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "endpoints", metricsServiceName, "-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("8443"), "Metrics endpoint is not ready")
			}
			Eventually(verifyMetricsEndpointReady).Should(Succeed())

			By("verifying that the controller manager is serving the metrics server")
			verifyMetricsServerStarted := func(g Gomega) {
				cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("Serving metrics server"),
					"Metrics server not yet started")
			}
			Eventually(verifyMetricsServerStarted).Should(Succeed())

			By("creating the curl-metrics pod to access the metrics endpoint")
			cmd = exec.Command("kubectl", "run", "curl-metrics", "--restart=Never",
				"--namespace", namespace,
				"--image=curlimages/curl:latest",
				"--overrides",
				fmt.Sprintf(`{
					"spec": {
						"containers": [{
							"name": "curl",
							"image": "curlimages/curl:latest",
							"command": ["/bin/sh", "-c"],
							"args": ["curl -v -k -H 'Authorization: Bearer %s' https://%s.%s.svc.cluster.local:8443/metrics"],
							"securityContext": {
								"allowPrivilegeEscalation": false,
								"capabilities": {
									"drop": ["ALL"]
								},
								"runAsNonRoot": true,
								"runAsUser": 1000,
								"seccompProfile": {
									"type": "RuntimeDefault"
								}
							}
						}],
						"serviceAccount": "%s"
					}
				}`, token, metricsServiceName, namespace, serviceAccountName))
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create curl-metrics pod")

			By("waiting for the curl-metrics pod to complete.")
			verifyCurlUp := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pods", "curl-metrics",
					"-o", "jsonpath={.status.phase}",
					"-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Succeeded"), "curl pod in wrong status")
			}
			Eventually(verifyCurlUp, 5*time.Minute).Should(Succeed())

			By("getting the metrics by checking curl-metrics logs")
			metricsOutput := getMetricsOutput()
			Expect(metricsOutput).To(ContainSubstring(
				"controller_runtime_reconcile_total",
			))
		})

		// +kubebuilder:scaffold:e2e-webhooks-checks

		It("creates multiple ModelService resources using kubectl", func() {
			By("Creating the basic base configmap")
			var cm corev1.ConfigMap
			err := yaml.Unmarshal([]byte(configMapYAML), &cm)
			Expect(err).NotTo(HaveOccurred())

			cm.Namespace = "default"

			err = k8sClient.Create(ctx, &cm)
			Expect(err).NotTo(HaveOccurred())

			By(fmt.Sprintf("Creating ModelService %s", modelServiceName))

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
						ModelServicePodSpec: msv1alpha1.ModelServicePodSpec{
							Containers: []msv1alpha1.ContainerSpec{
								{
									Name:  "llm",
									Image: &imageName,
								},
							},
						},
					},
					Prefill: &msv1alpha1.PDSpec{
						ModelServicePodSpec: msv1alpha1.ModelServicePodSpec{
							Containers: []msv1alpha1.ContainerSpec{
								{
									Name:  "llm",
									Image: &imageName,
								},
							},
						},
					},
					BaseConfigMapRef: &corev1.ObjectReference{
						Name:      "basic-base-conf",
						Namespace: "default",
					},
				},
			}

			By(fmt.Sprintf("Creating ModelService %s", modelServiceName))
			err = k8sClient.Create(context.TODO(), modelService)
			Expect(err).ToNot(HaveOccurred())

			By("Checking if Decode deployment was created")
			var decode appsv1.Deployment
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: decodeWorkloadName, Namespace: namespace}, &decode)
				return err == nil
			}, time.Second*5, time.Millisecond*500).Should(BeTrue())

			By("Checking if prefill deployment was created")
			var prefill appsv1.Deployment
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: prefillWorkloadName, Namespace: namespace}, &prefill)
				return err == nil
			}, time.Second*5, time.Millisecond*500).Should(BeTrue())

			By("Checking if a PD SA was created")
			sa := corev1.ServiceAccount{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: modelServiceName + "-sa", Namespace: namespace}, &sa)
				return err == nil
			}, time.Second*5, time.Millisecond*500).Should(BeTrue())

			By("Validating that the EPP ServiceAccount was created")
			eppSA := corev1.ServiceAccount{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: modelServiceName + "-epp-sa", Namespace: namespace}, &eppSA)
				return err == nil
			}, time.Second*5, time.Millisecond*500).Should(BeTrue())

			By("Checking if a epp RoleBinding was created")
			rolebinding := rbacv1.RoleBinding{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: modelServiceName + "-epp-rolebinding", Namespace: namespace}, &rolebinding)
				return err == nil
			}, time.Second*5, time.Millisecond*500).Should(BeTrue())

			By("Validating that the inferencepool was created")
			infPool := giev1alpha2.InferencePool{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: modelServiceName + "-inference-pool", Namespace: namespace}, &infPool)
				return err == nil
			}, time.Second*5, time.Millisecond*500).Should(BeTrue())

			By("Validating that the inferencemodel was created")
			infModel := giev1alpha2.InferenceModel{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: modelServiceName, Namespace: namespace}, &infModel)
				return err == nil
			}, time.Second*5, time.Millisecond*500).Should(BeTrue())
		})
	})

})

// serviceAccountToken returns a token for the specified service account in the given namespace.
// It uses the Kubernetes TokenRequest API to generate a token by directly sending a request
// and parsing the resulting token from the API response.
func serviceAccountToken() (string, error) {
	const tokenRequestRawString = `{
		"apiVersion": "authentication.k8s.io/v1",
		"kind": "TokenRequest"
	}`

	// Temporary file to store the token request
	secretName := fmt.Sprintf("%s-token-request", serviceAccountName)
	tokenRequestFile := filepath.Join("/tmp", secretName)
	err := os.WriteFile(tokenRequestFile, []byte(tokenRequestRawString), os.FileMode(0o644))
	if err != nil {
		return "", err
	}

	var out string
	verifyTokenCreation := func(g Gomega) {
		// Execute kubectl command to create the token
		cmd := exec.Command("kubectl", "create", "--raw", fmt.Sprintf(
			"/api/v1/namespaces/%s/serviceaccounts/%s/token",
			namespace,
			serviceAccountName,
		), "-f", tokenRequestFile)

		output, err := cmd.CombinedOutput()
		g.Expect(err).NotTo(HaveOccurred())

		// Parse the JSON output to extract the token
		var token tokenRequest
		err = json.Unmarshal(output, &token)
		g.Expect(err).NotTo(HaveOccurred())

		out = token.Status.Token
	}
	Eventually(verifyTokenCreation).Should(Succeed())

	return out, err
}

// getMetricsOutput retrieves and returns the logs from the curl pod used to access the metrics endpoint.
func getMetricsOutput() string {
	By("getting the curl-metrics logs")
	cmd := exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
	metricsOutput, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to retrieve logs from curl pod")
	Expect(metricsOutput).To(ContainSubstring("< HTTP/1.1 200 OK"))
	return metricsOutput
}

// tokenRequest is a simplified representation of the Kubernetes TokenRequest API response,
// containing only the token field that we need to extract.
type tokenRequest struct {
	Status struct {
		Token string `json:"token"`
	} `json:"status"`
}

func stringToReader(s string) *stringReader {
	return &stringReader{s: s}
}

type stringReader struct {
	s string
	i int64
}

func (r *stringReader) Read(p []byte) (int, error) {
	if r.i >= int64(len(r.s)) {
		return 0, fmt.Errorf("EOF")
	}
	n := copy(p, r.s[r.i:])
	r.i += int64(n)
	return n, nil
}
