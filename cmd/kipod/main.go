package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version = "0.1.0"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "kipod",
		Short: "Kubernetes in Podman with CRI-O",
		Long:  `kipod creates Kubernetes clusters using podman containers as nodes with CRI-O as the container runtime.`,
		Version: version,
	}

	rootCmd.AddCommand(createCmd())
	rootCmd.AddCommand(deleteCmd())
	rootCmd.AddCommand(getCmd())
	rootCmd.AddCommand(buildCmd())
	rootCmd.AddCommand(checkCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func createCmd() *cobra.Command {
	var configFile string

	cmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Create a Kubernetes cluster",
		Long:  `Create a Kubernetes cluster using podman containers with CRI-O runtime.`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clusterName := "kipod"
			if len(args) > 0 {
				clusterName = args[0]
			}

			fmt.Printf("Creating cluster '%s'...\n", clusterName)
			return createCluster(clusterName, configFile)
		},
	}

	cmd.Flags().StringVar(&configFile, "config", "", "path to cluster config file")

	return cmd
}

func deleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete [name]",
		Short: "Delete a Kubernetes cluster",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clusterName := "kipod"
			if len(args) > 0 {
				clusterName = args[0]
			}

			fmt.Printf("Deleting cluster '%s'...\n", clusterName)
			return deleteCluster(clusterName)
		},
	}

	return cmd
}

func getCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get cluster information",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "clusters",
		Short: "List all clusters",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listClusters()
		},
	})

	return cmd
}

func buildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build kipod components",
	}

	cmd.AddCommand(buildNodeImageCmd())

	return cmd
}

func buildNodeImageCmd() *cobra.Command {
	var k8sVersion string
	var crioVersion string
	var imageName string
	var imageTag string

	cmd := &cobra.Command{
		Use:   "node-image",
		Short: "Build a kipod node image",
		Long:  `Build a container image with Kubernetes and CRI-O pre-installed for use as a kipod node.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return buildNodeImage(k8sVersion, crioVersion, imageName, imageTag)
		},
	}

	cmd.Flags().StringVar(&k8sVersion, "k8s-version", "1.28", "Kubernetes version to install")
	cmd.Flags().StringVar(&crioVersion, "crio-version", "1.28", "CRI-O version to install")
	cmd.Flags().StringVar(&imageName, "image-name", "localhost/kipod-node", "Name for the built image")
	cmd.Flags().StringVar(&imageTag, "image-tag", "latest", "Tag for the built image")

	return cmd
}

func checkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check system prerequisites",
		Long:  `Validate that the system meets requirements for running kipod clusters.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return checkSystem()
		},
	}

	return cmd
}
