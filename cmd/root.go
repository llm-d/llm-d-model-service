package cmd

import (
	"os"

	"github.com/llm-d/llm-d-model-service/internal/controller"
	"github.com/spf13/cobra"
)

// rbac options
var rbacOptions controller.RBACOptions

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "manager",
	Short: "ModelService controller CLI",
	Long:  `ModelService controller CLI`,
}

// MyExecute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	// secrets & cluster roles
	rootCmd.PersistentFlags().StringVar(&rbacOptions.EPPClusterRole, "epp-cluster-role", "", "Name of the epp cluster role")
	_ = rootCmd.MarkFlagRequired("epp-cluster-role")
	rootCmd.PersistentFlags().StringSliceVar(&rbacOptions.EPPPullSecrets, "epp-pull-secrets", []string{}, "List of pull secrets for configuring the epp deployment")
	rootCmd.PersistentFlags().StringSliceVar(&rbacOptions.PDPullSecrets, "pd-pull-secrets", []string{}, "List of pull secrets for configuring the prefill and decode deployments")
}
