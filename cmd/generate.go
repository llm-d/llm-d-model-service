package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/yaml"

	msv1alpha1 "github.com/neuralmagic/llm-d-model-service/api/v1alpha1"
	"github.com/neuralmagic/llm-d-model-service/internal/controller"
	giev1alpha2 "sigs.k8s.io/gateway-api-inference-extension/api/v1alpha2"
)

func readModelService(filename string, logger logr.Logger) (*msv1alpha1.ModelService, error) {
	var modelService msv1alpha1.ModelService
	data, err := os.ReadFile(filename)
	if err != nil {
		logger.Error(err, "unable to read ModelService from "+filename)
		return nil, err
	}

	err = yaml.Unmarshal(data, &modelService)
	if err != nil {
		logger.Error(err, "unable to unmarshal data")
		return nil, err
	}

	return &modelService, nil
}

func readBaseChildResources(filename string, logger logr.Logger) (*controller.BaseConfig, error) {
	var baseChildResourcesConfigMap *corev1.ConfigMap
	var baseChildResources *controller.BaseConfig

	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Error(err, "unable to read base child resources from "+filename)
			return nil, err
		}
		data = []byte{}
	}

	err = yaml.Unmarshal(data, &baseChildResourcesConfigMap)
	if err != nil {
		logger.Error(err, "unable to unmarshal base child resources")
		return nil, err
	}

	baseChildResources, err = controller.BaseConfigFromCM(baseChildResourcesConfigMap)
	if err != nil {
		logger.Error(err, "unable to create base child resources from config map")
		return nil, err
	}

	return baseChildResources, nil
}

func toYaml(obj interface{}) string {
	var yamlStr string

	yamlBytes, err := yaml.Marshal(&obj)
	if err != nil {
		yamlStr = "message: not able to marshal object\nerror: " + err.Error()
	} else {
		yamlStr = string(yamlBytes)
	}
	return yamlStr
}

func generateManifests(ctx context.Context, manifestFile string, configFile string) (*string, error) {
	logger := log.FromContext(ctx)

	// get msvc from file
	msvc, err := readModelService(manifestFile, logger)
	if err != nil {
		logger.Error(err, "unable to read ModelService", "location", manifestFile)
		return nil, err
	}
	logger.Info("generateManifest", "modelService", msvc)

	// get base child resources from file
	config, err := readBaseChildResources(configFile, logger)
	if err != nil {
		logger.Error(err, "unable to read basic configuration", "location", configFile)
		return nil, err
	}
	logger.Info("generateManifest", "baseResources", config)

	// create scheme
	err = msv1alpha1.AddToScheme(scheme.Scheme)
	if err != nil {
		logger.Info("unable to add model service to scheme")
		return nil, err
	}
	err = giev1alpha2.Install(scheme.Scheme)
	if err != nil {
		logger.Info("unable to add gateway api extension to scheme")
		return nil, err
	}

	// update child resources
	cR := config.UpdateChildResources(ctx, msvc, scheme.Scheme)
	logger.Info("generateManifest", "baseResources", cR)

	// generate yaml for chile resources
	allYaml := ""
	for _, cm := range cR.ConfigMaps {
		allYaml = fmt.Sprintf("%s\n---\n%s", allYaml, toYaml(cm))
	}
	if cR.PrefillDeployment != nil {
		allYaml = fmt.Sprintf("%s\n---\n%s", allYaml, toYaml(cR.PrefillDeployment))
	}
	if cR.DecodeDeployment != nil {
		allYaml = fmt.Sprintf("%s\n---\n%s", allYaml, toYaml(cR.DecodeDeployment))
	}
	if cR.PrefillService != nil {
		allYaml = fmt.Sprintf("%s\n---\n%s", allYaml, toYaml(cR.PrefillService))
	}
	if cR.DecodeService != nil {
		allYaml = fmt.Sprintf("%s\n---\n%s", allYaml, toYaml(cR.DecodeService))
	}
	if cR.InferencePool != nil {
		allYaml = fmt.Sprintf("%s\n---\n%s", allYaml, toYaml(cR.InferencePool))
	}
	if cR.InferenceModel != nil {
		allYaml = fmt.Sprintf("%s\n---\n%s", allYaml, toYaml(cR.InferenceModel))
	}
	if cR.EPPDeployment != nil {
		allYaml = fmt.Sprintf("%s\n---\n%s", allYaml, toYaml(cR.EPPDeployment))
	}
	if cR.EPPService != nil {
		allYaml = fmt.Sprintf("%s\n---\n%s", allYaml, toYaml(cR.EPPService))
	}

	logger.Info("generateManifest", "yaml", allYaml)
	return &allYaml, nil
}

var modelServiceManifest string
var baseConfigurationManifest string

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate manifest",
	Long:  `Generate manifest for objects created by ModelService controller`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()

		logger := zap.New(zap.UseFlagOptions(&opts))
		log.SetLogger(logger)
		log.IntoContext(ctx, logger)

		result, _ := generateManifests(ctx, modelServiceManifest, baseConfigurationManifest)
		fmt.Println(*result)
	},
}

func init() {
	generateCmd.Flags().StringVar(&modelServiceManifest, "modelservice", "", "File containing the ModelService definition.")
	_ = rootCmd.MarkFlagRequired("modelservice")
	generateCmd.Flags().StringVar(&baseConfigurationManifest, "baseConfiguration", "", "File containing the base platform configuration.")
	rootCmd.AddCommand(generateCmd)
}
