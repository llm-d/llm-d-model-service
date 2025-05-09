package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	zaplog "go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/yaml"

	msv1alpha1 "github.com/neuralmagic/llm-d-model-service/api/v1alpha1"
	"github.com/neuralmagic/llm-d-model-service/internal/controller"
	giev1alpha2 "sigs.k8s.io/gateway-api-inference-extension/api/v1alpha2"
)

func readModelService(ctx context.Context, filename string, logger logr.Logger) (*msv1alpha1.ModelService, error) {
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

	// interpolate MSVC
	return controller.InterpolateModelService(ctx, &modelService)
}

func getBaseChildResources(filename string, msvc *msv1alpha1.ModelService, logger logr.Logger) (*controller.BaseConfig, error) {
	var baseChildResourcesConfigMap *corev1.ConfigMap
	var baseChildResources *controller.BaseConfig

	if filename != "" {
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
	} else {
		baseChildResourcesConfigMap = &corev1.ConfigMap{}
	}

	interpolated, err := controller.InterpolateBaseConfigMap(context.TODO(), baseChildResourcesConfigMap, msvc)
	if err != nil {
		logger.Error(err, "cannot interpolate base configmap")
		return nil, err
	}

	baseChildResources, err = controller.BaseConfigFromCM(interpolated)
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

	// get msvc from file and interpolate it
	msvc, err := readModelService(ctx, manifestFile, logger)
	if err != nil {
		logger.Error(err, "unable to read ModelService", "location", manifestFile)
		return nil, err
	}
	logger.V(1).Info("generateManifest", "modelService", msvc)

	// get base child resources from file
	config, err := getBaseChildResources(configFile, msvc, logger)
	if err != nil {
		logger.Error(err, "unable to read basic configuration", "location", configFile)
		return nil, err
	}
	logger.V(1).Info("generateManifest", "baseResources", config)

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
	cR := config.MergeChildResources(ctx, msvc, scheme.Scheme, &rbacOptions)
	logger.V(1).Info("generateManifest", "baseResources", cR)

	// add apiVersion and kind
	for i, _ := range cR.ConfigMaps {
		cR.ConfigMaps[i].APIVersion = "v1"
		cR.ConfigMaps[i].Kind = "ConfigMap"
	}

	if cR.PrefillDeployment != nil {
		cR.PrefillDeployment.APIVersion = "apps/v1"
		cR.PrefillDeployment.Kind = "Deployment"
	}
	if cR.PrefillService != nil {
		cR.PrefillService.APIVersion = "v1"
		cR.PrefillService.Kind = "Service"
	}
	if cR.DecodeDeployment != nil {
		cR.DecodeDeployment.APIVersion = "apps/v1"
		cR.DecodeDeployment.Kind = "Deployment"
	}
	if cR.DecodeService != nil {
		cR.DecodeService.APIVersion = "v1"
		cR.DecodeService.Kind = "Service"
	}
	if cR.PDServiceAccount != nil {
		cR.PDServiceAccount.APIVersion = "v1"
		cR.PDServiceAccount.Kind = "ServiceAccount"
	}
	if cR.PDRoleBinding != nil {
		cR.PDRoleBinding.APIVersion = "rbac.authorization.k8s.io/v1"
		cR.PDRoleBinding.Kind = "RoleBinding"
	}

	if cR.EPPDeployment != nil {
		cR.EPPDeployment.APIVersion = "apps/v1"
		cR.EPPDeployment.Kind = "Deployment"
	}
	if cR.EPPService != nil {
		cR.EPPService.APIVersion = "v1"
		cR.EPPService.Kind = "Service"
	}
	if cR.EPPServiceAccount != nil {
		cR.EPPServiceAccount.APIVersion = "v1"
		cR.EPPServiceAccount.Kind = "ServiceAccount"
	}
	if cR.EPPRoleBinding != nil {
		cR.EPPRoleBinding.APIVersion = "rbac.authorization.k8s.io/v1"
		cR.EPPRoleBinding.Kind = "RoleBinding"
	}

	if cR.InferencePool != nil {
		cR.InferencePool.APIVersion = "inference.networking.x-k8s.io/v1alpha2"
		cR.InferencePool.Kind = "InferencePool"
	}
	if cR.InferenceModel != nil {
		cR.InferenceModel.APIVersion = "inference.networking.x-k8s.io/v1alpha2"
		cR.InferenceModel.Kind = "InferencePool"
	}

	yamlStr := ""
	yamlBytes, err := yaml.Marshal(&cR)
	if err != nil {
		yamlStr = "message: not able to marshal object\nerror: " + err.Error()
	} else {
		yamlStr = string(yamlBytes)
	}

	// allYaml := toYaml(cR)
	return &yamlStr, nil
}

var modelServiceManifest string
var baseConfigurationManifest string

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate manifest",
	Long:  `Generate manifest for objects created by ModelService controller`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()
		var opts = zap.Options{
			Development: false,
			TimeEncoder: zapcore.RFC3339NanoTimeEncoder,
			ZapOpts:     []zaplog.Option{zaplog.AddCaller()},
			Level:       parseZapLogLevel(logLevel),
		}
		logger := zap.New(zap.UseFlagOptions(&opts))
		log.SetLogger(logger)
		log.IntoContext(ctx, logger)

		result, _ := generateManifests(ctx, modelServiceManifest, baseConfigurationManifest)
		fmt.Println(*result)
	},
}

func init() {
	generateCmd.Flags().StringVarP(&modelServiceManifest, "modelservice", "m", "", "File containing the ModelService definition.")
	_ = generateCmd.MarkFlagRequired("modelservice")
	generateCmd.Flags().StringVarP(&baseConfigurationManifest, "baseconfig", "b", "", "File containing the base platform configuration.")
	rootCmd.AddCommand(generateCmd)
}
