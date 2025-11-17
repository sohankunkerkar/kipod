package main

import (
	"fmt"

	"github.com/skunkerk/kipod/pkg/cluster"
)

func createCluster(name, configFile string) error {
	cfg := &cluster.Config{
		Name: name,
		Nodes: 1,
	}

	c, err := cluster.NewCluster(cfg)
	if err != nil {
		return fmt.Errorf("failed to create cluster: %w", err)
	}

	if err := c.Create(); err != nil {
		return fmt.Errorf("failed to provision cluster: %w", err)
	}

	fmt.Printf("\nCluster '%s' created successfully!\n", name)
	fmt.Printf("\nTo interact with your cluster:\n")
	fmt.Printf("  podman exec %s-control-plane-0 cat /etc/kubernetes/admin.conf > ~/.kube/%s-config\n", name, name)
	fmt.Printf("  export KUBECONFIG=~/.kube/%s-config\n", name)
	fmt.Printf("  kubectl get nodes\n")

	return nil
}

func deleteCluster(name string) error {
	if err := cluster.Delete(name); err != nil {
		return fmt.Errorf("failed to delete cluster: %w", err)
	}

	fmt.Printf("Cluster '%s' deleted successfully!\n", name)
	return nil
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
