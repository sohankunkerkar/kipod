package main

import (
	"fmt"
	"os"
	"regexp"

	"time"

	"github.com/sohankunkerkar/kipod/pkg/cluster"
	"github.com/sohankunkerkar/kipod/pkg/config"
	"github.com/sohankunkerkar/kipod/pkg/style"
)

func createCluster(name, configFile, nodeImage, kubeconfigPath string, retain bool, waitDuration string) error {
	// TODO: Implement nodeImage, kubeconfigPath, retain, and waitDuration support

	// Load config from file or use defaults
	var kipodCfg *config.ClusterConfig
	var err error

	if configFile != "" {
		kipodCfg, err = config.LoadFromFile(configFile)
		if err != nil {
			return fmt.Errorf("failed to load config file: %w", err)
		}
	} else {
		kipodCfg = config.DefaultConfig()
	}

	// Override cluster name if provided via flag
	if name != "" {
		kipodCfg.Name = name
	}

	// Print header now that we know the cluster name
	if !quietMode {
		style.Header("Creating cluster %q ...", kipodCfg.Name)
		if configFile != "" {
			style.Header("Using configuration from: %s", configFile)
		}
	}

	// Map config to cluster.Config
	cfg := &cluster.Config{
		Name:          kipodCfg.Name,
		Nodes:         kipodCfg.Nodes.ControlPlanes + kipodCfg.Nodes.Workers,
		ControlPlanes: kipodCfg.Nodes.ControlPlanes,
		Workers:       kipodCfg.Nodes.Workers,
		Image:         nodeImage, // Use flag value if provided
		PodSubnet:     kipodCfg.Networking.PodSubnet,
		ServiceSubnet: kipodCfg.Networking.ServiceSubnet,
		CgroupManager: kipodCfg.CgroupManager,
		// Local builds
		CRIOBinary: kipodCfg.LocalBuilds.CRIOBinary,
		CrunBinary: kipodCfg.LocalBuilds.CrunBinary,
		RuncBinary: kipodCfg.LocalBuilds.RuncBinary,
		Retain:     retain,
	}

	if waitDuration != "" {
		d, err := time.ParseDuration(waitDuration)
		if err != nil {
			return fmt.Errorf("invalid wait duration: %w", err)
		}
		cfg.WaitDuration = d
	}

	// Validate local build paths exist
	if cfg.CRIOBinary != "" {
		if _, err := os.Stat(cfg.CRIOBinary); err != nil {
			return fmt.Errorf("CRI-O binary not found at %s: %w", cfg.CRIOBinary, err)
		}
		if !quietMode {
			style.Header("Using local CRI-O binary: %s", cfg.CRIOBinary)
		}
	}
	if cfg.CrunBinary != "" {
		if _, err := os.Stat(cfg.CrunBinary); err != nil {
			return fmt.Errorf("crun binary not found at %s: %w", cfg.CrunBinary, err)
		}
		if !quietMode {
			style.Header("Using local crun binary: %s", cfg.CrunBinary)
		}
	}
	if cfg.RuncBinary != "" {
		if _, err := os.Stat(cfg.RuncBinary); err != nil {
			return fmt.Errorf("runc binary not found at %s: %w", cfg.RuncBinary, err)
		}
		if !quietMode {
			style.Header("Using local runc binary: %s", cfg.RuncBinary)
		}
	}

	c, err := cluster.NewCluster(cfg)
	if err != nil {
		return fmt.Errorf("failed to create cluster: %w", err)
	}

	if err := c.Create(); err != nil {
		return fmt.Errorf("failed to provision cluster: %w", err)
	}

	// Use the final cluster name (from config or flag override)
	clusterName := kipodCfg.Name

	// Automatically export kubeconfig
	// fmt.Printf("\nExporting kubeconfig...\n")
	kubeconfig, err := cluster.GetKubeconfig(clusterName)
	if err != nil {
		return fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	// Patch kubeconfig to use localhost instead of the container/host IP
	// This is necessary because the API server is published on localhost:6443
	kubeconfigPatched := patchKubeconfigServer(kubeconfig)

	// Create .kube directory if it doesn't exist
	kubeconfigDir := fmt.Sprintf("%s/.kube", os.Getenv("HOME"))
	if err := os.MkdirAll(kubeconfigDir, 0755); err != nil {
		return fmt.Errorf("failed to create .kube directory: %w", err)
	}

	// Write kubeconfig to file
	exportedPath := fmt.Sprintf("%s/%s-config", kubeconfigDir, clusterName)
	if kubeconfigPath != "" {
		exportedPath = kubeconfigPath
	}
	if err := os.WriteFile(exportedPath, []byte(kubeconfigPatched), 0600); err != nil {
		return fmt.Errorf("failed to write kubeconfig: %w", err)
	}

	if !quietMode {
		style.Header("\nCluster %q created successfully!", clusterName)
		style.Header("\nTo start using your cluster, run:")
		style.Header("  export KUBECONFIG=%s", exportedPath)
		style.Header("  kubectl get nodes")
	}

	return nil
}

func deleteCluster(name, kubeconfigPath string) error {
	// TODO: Implement kubeconfigPath support (for removing cluster from kubeconfig)
	if err := cluster.Delete(name); err != nil {
		return fmt.Errorf("failed to delete cluster: %w", err)
	}

	if !quietMode {
		style.Header("Cluster %q deleted successfully!", name)
	}
	return nil
}

func getKubeconfig(name string, internal bool) error {
	kubeconfig, err := cluster.GetKubeconfig(name)
	if err != nil {
		return fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	// Patch kubeconfig based on internal flag
	kubeconfigOutput := kubeconfig
	if !internal {
		kubeconfigOutput = patchKubeconfigServer(kubeconfig)
	}

	fmt.Print(kubeconfigOutput)
	return nil
}

func exportKubeconfig(name, kubeconfigPath string, internal bool) error {
	// TODO: Implement kubeconfigPath support and merging with existing kubeconfig
	// For now, just print the kubeconfig like get kubeconfig
	return getKubeconfig(name, internal)
}

func listClusters() error {
	clusters, err := cluster.List()
	if err != nil {
		return fmt.Errorf("failed to list clusters: %w", err)
	}

	if len(clusters) == 0 {
		fmt.Println("No clusters found.")
		return nil
	}

	fmt.Println("NAME")
	for _, c := range clusters {
		fmt.Println(c)
	}

	return nil
}

// patchKubeconfigServer replaces the server address in kubeconfig with localhost:6443
func patchKubeconfigServer(kubeconfig string) string {
	// Replace any server address with localhost:6443
	re := regexp.MustCompile(`server:\s+https://[^\s:]+:6443`)
	return re.ReplaceAllString(kubeconfig, "server: https://localhost:6443")
}
