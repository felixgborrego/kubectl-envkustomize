package cmd

import "github.com/spf13/cobra"
import "kubectl-envkustomize/pkg"

var renderCmd = &cobra.Command{
	Use:   "render",
	Short: "Render kustomize manifests",
	Long:  `Render kustomize manifests from the environment configurations. To run this command you meed an .env and kustomization.yaml file on the current directory.`,
	Run: func(cmd *cobra.Command, args []string) {
		envFile, _ := cmd.Flags().GetString("env-file")
		pkg.RenderCmd(envFile)
	},
}

func init() {
	renderCmd.Flags().StringP("env-file", "e", ".env", "Path to the environment file")
}
