package cmd

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:   "kubectl-envkustomize",
	Short: "Kubectl env kustomize plugin",
	Long:  `Kustomize with env support`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(renderCmd)
}
