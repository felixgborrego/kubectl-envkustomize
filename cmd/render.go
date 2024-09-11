package cmd

import (
	"kubectl-envkustomize/pkg"

	"github.com/spf13/cobra"
)

var renderCmd = &cobra.Command{
	Use:   "render",
	Short: "Render kustomize manifests with envs",
	Long:  `Render kustomize manifests using the environment configurations. To run this command you meed an .env and kustomization.yaml file on the current directory.`,
	Run: func(cmd *cobra.Command, args []string) {
		envFile, _ := cmd.Flags().GetString("env-file")
		pkg.RenderCmd(envFile)
	},
}

func init() {
	renderCmd.Flags().StringP("env-file", "e", ".env", "Path to the environment file")
}
