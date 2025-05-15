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

	msv1alpha1 "github.com/llm-d/llm-d-model-service/api/v1alpha1"
	"github.com/llm-d/llm-d-model-service/internal/controller"
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

	yamlStr := ""
	yamlBytes, err := yaml.Marshal(&cR)
	if err != nil {
		logger.Error(err, "unable to marshal object to YAML")
		return nil, err
	}

	yamlStr = string(yamlBytes)
	return &yamlStr, nil
}

var modelServiceManifest string
var baseConfigurationManifest string

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate manifest",
	Long:  `Generate manifest for objects created by ModelService controller`,
	RunE: func(cmd *cobra.Command, args []string) error {
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

		result, err := generateManifests(ctx, modelServiceManifest, baseConfigurationManifest)
		if err != nil {
			return err
		}

		fmt.Println(*result)
		return nil
	},
}

func init() {
	generateCmd.Flags().StringVarP(&modelServiceManifest, "modelservice", "m", "", "File containing the ModelService definition.")
	_ = generateCmd.MarkFlagRequired("modelservice")
	generateCmd.Flags().StringVarP(&baseConfigurationManifest, "baseconfig", "b", "", "File containing the base platform configuration.")
	rootCmd.AddCommand(generateCmd)
}
