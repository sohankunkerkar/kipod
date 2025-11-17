package cluster

import (
	"fmt"
	"strings"
	"time"

	"github.com/skunkerk/kipod/pkg/build"
	"github.com/skunkerk/kipod/pkg/podman"
)

// Config represents cluster configuration
type Config struct {
	Name              string
	Nodes             int
	Image             string
	KubernetesVersion string
	PodSubnet         string
	ServiceSubnet     string
	Rootless          bool
}

// Cluster represents a kipod cluster
type Cluster struct {
	config  *Config
	nodeIDs []string
}

// NewCluster creates a new cluster instance
func NewCluster(cfg *Config) (*Cluster, error) {
	if cfg.Name == "" {
		return nil, fmt.Errorf("cluster name cannot be empty")
	}

	// Set defaults
	if cfg.Nodes == 0 {
		cfg.Nodes = 1
	}
	if cfg.Image == "" {
		// Use the pre-built kipod node image
		cfg.Image = build.GetImageFullName(build.DefaultImageName, build.DefaultImageTag)
	}
	if cfg.KubernetesVersion == "" {
		cfg.KubernetesVersion = "1.28"
	}
	if cfg.PodSubnet == "" {
		cfg.PodSubnet = "10.244.0.0/16"
	}
	if cfg.ServiceSubnet == "" {
		cfg.ServiceSubnet = "10.96.0.0/12"
	}

	// Default to rootless mode with _CRIO_ROOTLESS=1 environment variable
	// This enables CRI-O to skip OOM score adjustments that require privileges
	cfg.Rootless = true

	return &Cluster{
		config:  cfg,
		nodeIDs: make([]string, 0),
	}, nil
}

// Create provisions the cluster
func (c *Cluster) Create() error {
	// Check if node image exists
	imageExists, err := build.ImageExists(c.config.Image)
	if err != nil {
		return fmt.Errorf("failed to check if node image exists: %w", err)
	}
	if !imageExists {
		return fmt.Errorf("node image '%s' not found. Please build it first with: kipod build node-image", c.config.Image)
	}

	fmt.Printf("Using node image: %s\n", c.config.Image)
	fmt.Println("Creating cluster nodes...")

	// For MVP, create a single control-plane node
	nodeID, err := c.createNode("control-plane", 0)
	if err != nil {
		return fmt.Errorf("failed to create control-plane node: %w", err)
	}
	c.nodeIDs = append(c.nodeIDs, nodeID)

	// Wait for container to be ready
	fmt.Println("Waiting for node to initialize...")
	time.Sleep(5 * time.Second)

	// Verify services are running
	fmt.Println("Verifying services...")
	if err := c.waitForServices(nodeID); err != nil {
		return fmt.Errorf("services failed to start: %w", err)
	}

	fmt.Println("Initializing Kubernetes cluster...")
	if err := c.initKubernetes(nodeID); err != nil {
		return fmt.Errorf("failed to initialize Kubernetes: %w", err)
	}

	return nil
}

func (c *Cluster) createNode(role string, index int) (string, error) {
	nodeName := fmt.Sprintf("%s-%s-%d", c.config.Name, role, index)

	opts := podman.CreateContainerOptions{
		Name:     nodeName,
		Image:    c.config.Image,
		Hostname: nodeName,
		Rootless: c.config.Rootless,
		Cgroupns: "private",
		Labels: map[string]string{
			podman.LabelCluster: c.config.Name,
			podman.LabelRole:    role,
		},
	}

	// Set _CRIO_ROOTLESS=1 to enable CRI-O rootless mode inside the container
	// This tells CRI-O to handle nested containers in a rootless-friendly way
	// Note: The outer Podman container still runs with --privileged for kubelet
	opts.Env = []string{"_CRIO_ROOTLESS=1"}

	containerID, err := podman.CreateContainer(opts)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	fmt.Printf("  Created node: %s (ID: %s)\n", nodeName, containerID[:12])

	return containerID, nil
}

func (c *Cluster) waitForServices(containerID string) error {
	// Wait for systemd to be ready
	maxRetries := 30
	for i := 0; i < maxRetries; i++ {
		output, err := podman.Exec(containerID, []string{"systemctl", "is-system-running"})
		if err == nil {
			status := strings.TrimSpace(output)
			if status == "running" || status == "degraded" {
				fmt.Printf("  Systemd is %s\n", status)
				break
			}
		}

		if i == maxRetries-1 {
			return fmt.Errorf("timeout waiting for systemd to be ready")
		}

		time.Sleep(2 * time.Second)
	}

	// Wait for CRI-O to be ready
	for i := 0; i < maxRetries; i++ {
		_, err := podman.Exec(containerID, []string{"systemctl", "is-active", "crio"})
		if err == nil {
			fmt.Println("  CRI-O is running")
			break
		}

		if i == maxRetries-1 {
			// Try to get logs
			logs, _ := podman.Exec(containerID, []string{"journalctl", "-u", "crio", "-n", "50", "--no-pager"})
			return fmt.Errorf("CRI-O failed to start. Logs:\n%s", logs)
		}

		time.Sleep(2 * time.Second)
	}

	// Verify CRI-O is functional
	_, err := podman.Exec(containerID, []string{"crictl", "info"})
	if err != nil {
		logs, _ := podman.Exec(containerID, []string{"journalctl", "-u", "crio", "-n", "50", "--no-pager"})
		return fmt.Errorf("CRI-O is not functional: %w\nLogs:\n%s", err, logs)
	}

	fmt.Println("  CRI-O is functional")
	return nil
}

func (c *Cluster) initKubernetes(containerID string) error {
	// Load pre-downloaded Kubernetes images to avoid nested image pulling issues
	fmt.Println("  Loading pre-downloaded Kubernetes images...")
	loadImagesCmd := `for tarball in /kind/images/*.tar; do
		if [ -f "$tarball" ]; then
			echo "Loading $(basename $tarball)..."
			# Convert filename back to image reference: replace all _ with /, then last / with :
			image_ref=$(basename "$tarball" .tar | sed 's/_/\//g; s/\/\([^/]*\)$/:\1/')
			skopeo copy docker-archive:$tarball containers-storage:$image_ref
		fi
	done`

	if _, err := podman.Exec(containerID, []string{"sh", "-c", loadImagesCmd}); err != nil {
		fmt.Printf("Warning: Failed to load some images: %v\n", err)
	}

	// Initialize Kubernetes using kubeadm
	initCmd := fmt.Sprintf(`kubeadm init \
  --pod-network-cidr=%s \
  --service-cidr=%s \
  --cri-socket=unix:///var/run/crio/crio.sock \
  --ignore-preflight-errors=NumCPU,Mem,SystemVerification,FileContent--proc-sys-net-bridge-bridge-nf-call-iptables \
  --v=5`, c.config.PodSubnet, c.config.ServiceSubnet)

	fmt.Println("  Running kubeadm init (this may take a few minutes)...")
	output, err := podman.Exec(containerID, []string{"sh", "-c", initCmd})
	if err != nil {
		return fmt.Errorf("kubeadm init failed: %w\nOutput:\n%s", err, output)
	}

	// Set up kubeconfig for root user
	kubeconfigCmd := `mkdir -p /root/.kube && \
cp /etc/kubernetes/admin.conf /root/.kube/config && \
chmod 600 /root/.kube/config`

	if _, err := podman.Exec(containerID, []string{"sh", "-c", kubeconfigCmd}); err != nil {
		return fmt.Errorf("failed to setup kubeconfig: %w", err)
	}

	// Wait for API server to be ready
	fmt.Println("  Waiting for API server...")
	maxRetries := 60
	for i := 0; i < maxRetries; i++ {
		_, err := podman.Exec(containerID, []string{"kubectl", "get", "nodes"})
		if err == nil {
			break
		}

		if i == maxRetries-1 {
			return fmt.Errorf("timeout waiting for API server")
		}

		time.Sleep(2 * time.Second)
	}

	// Remove control-plane taint (for single-node cluster)
	fmt.Println("  Configuring single-node cluster...")
	taintCmd := "kubectl taint nodes --all node-role.kubernetes.io/control-plane- || true"
	if _, err := podman.Exec(containerID, []string{"sh", "-c", taintCmd}); err != nil {
		fmt.Printf("  Warning: failed to remove control-plane taint: %v\n", err)
	}

	fmt.Println("  Kubernetes cluster initialized successfully")
	return nil
}

// Delete deletes a cluster by name
func Delete(name string) error {
	containers, err := podman.ListContainers(map[string]string{
		podman.LabelCluster: name,
	})
	if err != nil {
		return fmt.Errorf("failed to list cluster containers: %w", err)
	}

	if len(containers) == 0 {
		return fmt.Errorf("cluster '%s' not found", name)
	}

	fmt.Printf("Deleting %d node(s)...\n", len(containers))
	for _, container := range containers {
		if err := podman.DeleteContainer(container.ID); err != nil {
			return fmt.Errorf("failed to delete container %s: %w", container.Name, err)
		}
		fmt.Printf("  Deleted node: %s\n", container.Name)
	}

	return nil
}

// List returns a list of all cluster names
func List() ([]string, error) {
	containers, err := podman.ListContainers(map[string]string{
		podman.LabelCluster: "",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	clusterMap := make(map[string]bool)
	for _, container := range containers {
		// Extract cluster name from labels or container name
		parts := strings.Split(container.Name, "-")
		if len(parts) > 0 {
			clusterMap[parts[0]] = true
		}
	}

	clusters := make([]string, 0, len(clusterMap))
	for name := range clusterMap {
		clusters = append(clusters, name)
	}

	return clusters, nil
}
