package main

import (
	"fmt"
	"os"

	"github.com/skunkerk/kipod/pkg/style"
	"github.com/spf13/cobra"
)

var (
	version = "0.1.0"

	// Global flags
	quietMode bool
	verbosity int
)

func main() {
	rootCmd := &cobra.Command{
		Use:          "kipod",
		Short:        "Kubernetes in Podman with CRI-O",
		Long:         `kipod creates and manages local Kubernetes clusters using Podman container 'nodes' with CRI-O runtime`,
		Version:      version,
		SilenceUsage: true,
	}

	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&quietMode, "quiet", "q", false, "silence all stderr output")
	rootCmd.PersistentFlags().IntVarP(&verbosity, "verbosity", "v", 0, "info log verbosity, higher value produces more output")

	// Add commands
	rootCmd.AddCommand(buildCmd())
	rootCmd.AddCommand(createCmd())
	rootCmd.AddCommand(deleteCmd())
	rootCmd.AddCommand(exportCmd())
	rootCmd.AddCommand(getCmd())
	rootCmd.AddCommand(checkCmd())

	if err := rootCmd.Execute(); err != nil {
		if !quietMode {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		os.Exit(1)
	}
}

func createCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Creates one of [cluster]",
	}

	cmd.AddCommand(createClusterCmd())

	return cmd
}

func createClusterCmd() *cobra.Command {
	var (
		configFile     string
		clusterName    string
		nodeImage      string
		kubeconfigPath string
		retain         bool
		waitDuration   string
	)

	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Creates a local Kubernetes cluster",
		Long:  `Creates a local Kubernetes cluster using Podman container 'nodes'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check positional args for cluster name
			if len(args) > 0 {
				clusterName = args[0]
			}

			// Default cluster name
			if clusterName == "" {
				clusterName = "kipod"
			}

			if !quietMode {
				style.Header("Creating cluster %q ...", clusterName)
			}
			return createCluster(clusterName, configFile, nodeImage, kubeconfigPath, retain, waitDuration)
		},
	}

	cmd.Flags().StringVar(&configFile, "config", "", "path to a kipod config file")
	cmd.Flags().StringVarP(&clusterName, "name", "n", "", "cluster name, overrides KIPOD_CLUSTER_NAME, config (default kipod)")
	cmd.Flags().StringVar(&nodeImage, "image", "", "node image to use for booting the cluster")
	cmd.Flags().StringVar(&kubeconfigPath, "kubeconfig", "", "sets kubeconfig path instead of $KUBECONFIG or $HOME/.kube/config")
	cmd.Flags().BoolVar(&retain, "retain", false, "retain nodes for debugging when cluster creation fails")
	cmd.Flags().StringVar(&waitDuration, "wait", "0s", "wait for control plane node to be ready (default 0s)")

	return cmd
}

func deleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Deletes one of [cluster]",
	}

	cmd.AddCommand(deleteClusterCmd())

	return cmd
}

func deleteClusterCmd() *cobra.Command {
	var (
		clusterName    string
		kubeconfigPath string
	)

	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Deletes a kipod cluster",
		Long: `Deletes a kipod cluster from the system.

This is an idempotent operation, meaning it may be called multiple times without
failing (like "rm -f"). If the cluster resources exist they will be deleted, and
if the cluster is already gone it will just return success.

Errors will only occur if the cluster resources exist and are not able to be deleted.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check positional args for cluster name
			if len(args) > 0 {
				clusterName = args[0]
			}

			// Default cluster name
			if clusterName == "" {
				clusterName = "kipod"
			}

			if !quietMode {
				style.Header("Deleting cluster %q ...", clusterName)
			}
			return deleteCluster(clusterName, kubeconfigPath)
		},
	}

	cmd.Flags().StringVarP(&clusterName, "name", "n", "", "the cluster name (default kipod)")
	cmd.Flags().StringVar(&kubeconfigPath, "kubeconfig", "", "sets kubeconfig path instead of $KUBECONFIG or $HOME/.kube/config")

	return cmd
}

func getCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Gets one of [clusters, kubeconfig]",
	}

	cmd.AddCommand(getClustersCmd())
	cmd.AddCommand(getKubeconfigCmd())

	return cmd
}

func getClustersCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clusters",
		Short: "Lists existing kipod clusters by their name",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listClusters()
		},
	}
}

func getKubeconfigCmd() *cobra.Command {
	var (
		clusterName string
		internal    bool
	)

	cmd := &cobra.Command{
		Use:   "kubeconfig",
		Short: "Prints cluster kubeconfig",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Default cluster name
			if clusterName == "" {
				clusterName = "kipod"
			}

			return getKubeconfig(clusterName, internal)
		},
	}

	cmd.Flags().StringVarP(&clusterName, "name", "n", "", "the cluster context name (default kipod)")
	cmd.Flags().BoolVar(&internal, "internal", false, "use internal address instead of external")

	return cmd
}

func buildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build one of [node-image]",
	}

	cmd.AddCommand(buildNodeImageCmd())

	return cmd
}

func buildNodeImageCmd() *cobra.Command {
	var (
		configFile  string
		k8sVersion  string
		crioVersion string
		image       string
		rebuild     bool
	)

	cmd := &cobra.Command{
		Use:   "node-image",
		Short: "Build the node image which contains Kubernetes build artifacts and other kipod requirements",
		RunE: func(cmd *cobra.Command, args []string) error {
			return buildNodeImage(configFile, k8sVersion, crioVersion, image, rebuild)
		},
	}

	cmd.Flags().StringVar(&configFile, "config", "", "path to a kipod config file")
	cmd.Flags().StringVar(&k8sVersion, "k8s-version", "", "Kubernetes version to install (overrides config)")
	cmd.Flags().StringVar(&crioVersion, "crio-version", "", "CRI-O version to install (overrides config)")
	cmd.Flags().StringVar(&image, "image", "localhost/kipod-node:latest", "name:tag of the resulting image to be built")
	cmd.Flags().BoolVar(&rebuild, "rebuild", false, "force rebuild even if image already exists")

	return cmd
}

func exportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Exports one of [kubeconfig]",
	}

	cmd.AddCommand(exportKubeconfigCmd())

	return cmd
}

func exportKubeconfigCmd() *cobra.Command {
	var (
		clusterName    string
		kubeconfigPath string
		internal       bool
	)

	cmd := &cobra.Command{
		Use:   "kubeconfig",
		Short: "Exports cluster kubeconfig",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Default cluster name
			if clusterName == "" {
				clusterName = "kipod"
			}

			return exportKubeconfig(clusterName, kubeconfigPath, internal)
		},
	}

	cmd.Flags().StringVarP(&clusterName, "name", "n", "", "the cluster context name (default kipod)")
	cmd.Flags().StringVar(&kubeconfigPath, "kubeconfig", "", "sets kubeconfig path instead of $KUBECONFIG or $HOME/.kube/config")
	cmd.Flags().BoolVar(&internal, "internal", false, "use internal address instead of external")

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
