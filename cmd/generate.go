package cmd

import (
	"os"

	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/yaml"

	msv1alpha1 "github.com/neuralmagic/modelservice/api/v1alpha1"
)

func readModelService(manifest string, logger logr.Logger) (*msv1alpha1.ModelService, error) {
	var modelService msv1alpha1.ModelService
	data, err := os.ReadFile(manifest)
	if err != nil {
		logger.Error(err, "unable to read manifest")
		return nil, err
	}

	err = yaml.Unmarshal(data, &modelService)
	if err != nil {
		logger.Error(err, "unable to unmarshal data")
		return nil, err
	}

	return &modelService, nil
}

func generateManifest() {
	logger := zap.New(zap.UseFlagOptions(&opts))
	log.SetLogger(logger)

	logger.Info("generateManifest called")
	ms, err := readModelService(modelServiceManifest, logger)
	if err != nil {
		logger.Error(err, "unable to read ModelService", "location", modelServiceManifest)
	}
	logger.Info("generateManifest", "modelService", ms)

	// decodeDeployment, _ := controller.GetDeploymentForPDSpec(
	// 	context.Background(),
	// 	ms,
	// 	controller.DECODE_ROLE,
	// 	nil,
	// )

	// logger.Info("generateManifest", "decode deployment", decodeDeployment)

	// var manifestYamlStr string
	// manifestYamlBytes, err := yaml.Marshal(&decodeDeployment)
	// if err != nil {
	// 	logger.Error(err, "unable to marshal decodeDeployment")
	// 	manifestYamlStr = "can not unmarshal"
	// } else {
	// 	manifestYamlStr = string(manifestYamlBytes)
	// }
	// logger.Info("generateManifest", "decode deployment", manifestYamlStr)
}

var modelServiceManifest string
var baseConfigurationManifest string

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate manifest",
	Long:  `Generate manifest for objects created by ModelService controller`,
	Run: func(cmd *cobra.Command, args []string) {
		generateManifest()
	},
}

func init() {
	generateCmd.Flags().StringVar(&modelServiceManifest, "modelservice", "", "File containing the ModelService definition.")
	_ = rootCmd.MarkFlagRequired("modelservice")
	generateCmd.Flags().StringVar(&baseConfigurationManifest, "baseConfiguration", "", "File containing the base platform configuration.")
	rootCmd.AddCommand(generateCmd)
}
