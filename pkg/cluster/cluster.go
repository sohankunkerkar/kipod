package cluster

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sohankunkerkar/kipod/pkg/build"
	"github.com/sohankunkerkar/kipod/pkg/podman"
	"github.com/sohankunkerkar/kipod/pkg/style"
)

// Config represents cluster configuration
type Config struct {
	Name              string
	Nodes             int
	ControlPlanes     int
	Workers           int
	Image             string
	KubernetesVersion string
	PodSubnet         string
	ServiceSubnet     string
	Rootless          bool
	// Local builds for development
	CRIOBinary    string
	CrunBinary    string
	RuncBinary    string
	CgroupManager string
	CRIOConfig    string
	StorageType   string
	StorageSize   string
	WaitDuration  time.Duration
	Retain        bool
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
		if cfg.ControlPlanes == 0 && cfg.Workers == 0 {
			cfg.Nodes = 1
			cfg.ControlPlanes = 1
		} else {
			cfg.Nodes = cfg.ControlPlanes + cfg.Workers
		}
	}
	// Ensure at least one control plane
	if cfg.ControlPlanes == 0 {
		cfg.ControlPlanes = 1
		cfg.Nodes = cfg.ControlPlanes + cfg.Workers
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
func (c *Cluster) Create() (err error) {
	defer func() {
		if err != nil {
			c.cleanupOnFailure()
		}
	}()
	// Check if node image exists
	imageExists, err := build.ImageExists(c.config.Image)
	if err != nil {
		return fmt.Errorf("failed to check if node image exists: %w", err)
	}
	if !imageExists {
		return fmt.Errorf("node image '%s' not found. Please build it first with: kipod build node-image", c.config.Image)
	}

	style.Step("Ensuring node image (%s) 🖼", c.config.Image)

	// Create shared network
	networkName := "kipod"
	exists, err := podman.NetworkExists(networkName)
	if err != nil {
		return fmt.Errorf("failed to check network existence: %w", err)
	}
	if !exists {
		style.Step("Preparing network 🌐")
		if err := podman.CreateNetwork(networkName); err != nil {
			return fmt.Errorf("failed to create network: %w", err)
		}
	}

	style.Step("Preparing nodes 📦")

	// For MVP, create a single control-plane node
	nodeID, err := c.createNode("control-plane", 0)
	if err != nil {
		return fmt.Errorf("failed to create control-plane node: %w", err)
	}
	c.nodeIDs = append(c.nodeIDs, nodeID)

	// Wait for container to be ready
	style.Step("Starting control-plane 🕹️")
	// Initial wait for systemd to start
	time.Sleep(2 * time.Second)

	// Verify services are running
	// Verifying services...
	if err := c.waitForServices(nodeID); err != nil {
		return fmt.Errorf("services failed to start: %w", err)
	}

	style.Step("Initializing Kubernetes ☸️")
	if err := c.initKubernetes(nodeID); err != nil {
		return fmt.Errorf("failed to initialize Kubernetes: %w", err)
	}

	// Warn about HA support
	if c.config.ControlPlanes > 1 {
		fmt.Printf("Warning: Multi-control-plane (HA) support is not fully implemented yet. Only the first control-plane will be initialized.\n")
	}

	// Get join command from control-plane
	// Retrieving join command...
	joinCmd, err := c.getJoinCommand(nodeID)
	if err != nil {
		return fmt.Errorf("failed to get join command: %w", err)
	}

	// Create worker nodes
	for i := 0; i < c.config.Workers; i++ {
		workerID, err := c.createNode("worker", i)
		if err != nil {
			return fmt.Errorf("failed to create worker node %d: %w", i, err)
		}
		c.nodeIDs = append(c.nodeIDs, workerID)

		style.Step("Waiting for worker-%d to initialize... ⏳", i)
		time.Sleep(5 * time.Second)

		if err := c.waitForServices(workerID); err != nil {
			return fmt.Errorf("worker-%d services failed to start: %w", i, err)
		}

		style.Step("Joining worker-%d to cluster... 🔗", i)
		if err := c.joinWorker(workerID, joinCmd); err != nil {
			return fmt.Errorf("failed to join worker-%d: %w", i, err)
		}

		// Label the worker node
		workerName := fmt.Sprintf("%s-worker-%d", c.config.Name, i)
		style.Step("Labeling worker-%d as 'worker'... 🏷️", i)
		labelCmd := fmt.Sprintf("kubectl label node %s node-role.kubernetes.io/worker=", workerName)
		if _, err := podman.Exec(nodeID, []string{"sh", "-c", labelCmd}); err != nil {
			fmt.Printf("  Warning: failed to label worker node %s: %v\n", workerName, err)
		}
	}

	style.Success("Ready")
	return nil
}

func (c *Cluster) cleanupOnFailure() {
	if c.config.Retain {
		style.Info("Retaining nodes for debugging due to --retain flag")
		return
	}

	// Only cleanup if we have created nodes
	if len(c.nodeIDs) > 0 {
		style.Info("Cleaning up failed cluster...")
		for _, nodeID := range c.nodeIDs {
			podman.DeleteContainer(nodeID)
		}
	}
}

func (c *Cluster) getJoinCommand(controlPlaneID string) (string, error) {
	// Generate a new token and print the join command
	cmd := "kubeadm token create --print-join-command"
	output, err := podman.Exec(controlPlaneID, []string{"sh", "-c", cmd})
	if err != nil {
		return "", fmt.Errorf("failed to generate join command: %w", err)
	}
	return strings.TrimSpace(output), nil
}

func (c *Cluster) joinWorker(workerID, joinCmd string) error {
	// Run the join command on the worker
	// We need to ignore preflight errors similar to init
	fullCmd := fmt.Sprintf("%s --ignore-preflight-errors=NumCPU,Mem,SystemVerification,FileContent--proc-sys-net-bridge-bridge-nf-call-iptables --v=5", joinCmd)

	output, err := podman.Exec(workerID, []string{"sh", "-c", fullCmd})
	if err != nil {
		return fmt.Errorf("kubeadm join failed: %w\nOutput:\n%s", err, output)
	}
	return nil
}

func (c *Cluster) createNode(role string, index int) (string, error) {
	nodeName := fmt.Sprintf("%s-%s-%d", c.config.Name, role, index)

	opts := c.createContainerOptions(nodeName, role)

	containerID, err := podman.CreateContainer(opts)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	// fmt.Printf("  Created node: %s (ID: %s)\n", nodeName, containerID[:12])

	if err := c.installLocalBinaries(containerID); err != nil {
		return "", err
	}

	return containerID, nil
}

func (c *Cluster) createContainerOptions(nodeName, role string) podman.CreateContainerOptions {
	// Pass KIPOD_CGROUP_MANAGER to the container
	cgroupMgr := c.config.CgroupManager
	if cgroupMgr == "" {
		cgroupMgr = os.Getenv("KIPOD_CGROUP_MANAGER")
	}

	env := []string{}
	if cgroupMgr != "" {
		env = append(env, fmt.Sprintf("KIPOD_CGROUP_MANAGER=%s", cgroupMgr))
	}

	// Only set _CRIO_ROOTLESS=1 if using cgroupfs
	// If using systemd, we want CRI-O to use the system bus (as root inside container)
	if cgroupMgr == "cgroupfs" {
		env = append(env, "_CRIO_ROOTLESS=1")
	}

	opts := podman.CreateContainerOptions{
		Name:     nodeName,
		Image:    c.config.Image,
		Hostname: nodeName,
		Rootless: c.config.Rootless,
		Cgroupns: "private",
		Network:  "kipod",
		Labels: map[string]string{
			podman.LabelCluster: c.config.Name,
			podman.LabelRole:    role,
		},
		Env: env,
	}

	// Configure container storage
	if c.config.StorageType == "volume" {
		// Use named volume for storage - enables persistence and avoids overlay-on-overlay
		// (overlay-on-bind-mount works fine)
		// We use :shared propagation to allow CRI-O to create sub-mounts visible to the container
		volName := fmt.Sprintf("kipod-storage-%s", nodeName)
		opts.Volumes = append(opts.Volumes, fmt.Sprintf("%s:/var/lib/containers/storage:shared", volName))
	} else {
		// Use tmpfs for container storage - enables native overlay support
		// (overlay-on-overlay doesn't work, but overlay-on-tmpfs does)
		size := c.config.StorageSize
		if size == "" {
			size = "10G"
		}
		opts.Tmpfs = []string{fmt.Sprintf("/var/lib/containers/storage:rw,size=%s", size)}
	}

	// Mount local builds for development
	if c.config.CRIOBinary != "" {
		opts.Volumes = append(opts.Volumes, fmt.Sprintf("%s:/usr/local/bin/crio-custom:ro", c.config.CRIOBinary))
	}
	if c.config.CrunBinary != "" {
		opts.Volumes = append(opts.Volumes, fmt.Sprintf("%s:/usr/local/bin/crun-custom:ro", c.config.CrunBinary))
	}
	if c.config.RuncBinary != "" {
		opts.Volumes = append(opts.Volumes, fmt.Sprintf("%s:/usr/local/bin/runc-custom:ro", c.config.RuncBinary))
	}

	// Mount CRI-O config if provided
	if c.config.CRIOConfig != "" {
		opts.Volumes = append(opts.Volumes, fmt.Sprintf("%s:/tmp/crio-user-config.conf:ro", c.config.CRIOConfig))
	}

	// Publish API server port for control-plane nodes
	if role == "control-plane" {
		opts.Ports = []string{"6443:6443"}
	}

	return opts
}

func (c *Cluster) installLocalBinaries(containerID string) error {
	// Replace system binaries with local builds
	if c.config.CRIOBinary != "" {
		style.Info("Installing local CRI-O binary...")
		if _, err := podman.Exec(containerID, []string{"cp", "/usr/local/bin/crio-custom", "/usr/bin/crio"}); err != nil {
			return fmt.Errorf("failed to install local CRI-O: %w", err)
		}
	}
	if c.config.CrunBinary != "" {
		style.Info("Installing local crun binary...")
		// Replace the wrapper with local build
		if _, err := podman.Exec(containerID, []string{"cp", "/usr/bin/crun.real", "/usr/bin/crun.real.bak"}); err == nil {
			if _, err := podman.Exec(containerID, []string{"cp", "/usr/local/bin/crun-custom", "/usr/bin/crun.real"}); err != nil {
				return fmt.Errorf("failed to install local crun: %w", err)
			}
		}
	}
	if c.config.RuncBinary != "" {
		style.Info("Installing local runc binary...")
		if _, err := podman.Exec(containerID, []string{"cp", "/usr/local/bin/runc-custom", "/usr/bin/runc"}); err != nil {
			return fmt.Errorf("failed to install local runc: %w", err)
		}
	}
	return nil
}

func (c *Cluster) waitForServices(containerID string) error {
	// Wait for systemd to be ready
	maxRetries := 30
	for i := 0; i < maxRetries; i++ {
		output, err := podman.Exec(containerID, []string{"systemctl", "is-system-running"})
		if err == nil {
			status := strings.TrimSpace(output)
			if status == "running" || status == "degraded" {
				// fmt.Printf("  Systemd is %s\n", status)
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
			// fmt.Println("  CRI-O is running")
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

	// fmt.Println("  CRI-O is functional")
	return nil
}

func (c *Cluster) initKubernetes(containerID string) error {
	style.Step("Writing configuration 📜")
	// fmt.Println("  Running kubeadm init (this may take a few minutes)...")
	if err := c.runKubeadmInit(containerID); err != nil {
		return err
	}

	// Set up kubeconfig for root user
	kubeconfigCmd := `mkdir -p /root/.kube && \
cp /etc/kubernetes/admin.conf /root/.kube/config && \
chmod 600 /root/.kube/config`

	if _, err := podman.Exec(containerID, []string{"sh", "-c", kubeconfigCmd}); err != nil {
		return fmt.Errorf("failed to setup kubeconfig: %w", err)
	}

	// Wait for API server to be ready
	timeout := c.config.WaitDuration
	if timeout == 0 {
		timeout = 5 * time.Minute // Default timeout
	}
	style.Step("Waiting ≤ %s for control-plane = Ready ⏳", timeout)
	maxRetries := int(timeout.Seconds() / 2)
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
	// fmt.Println("  Configuring single-node cluster...")
	taintCmd := "kubectl taint nodes --all node-role.kubernetes.io/control-plane- || true"
	if _, err := podman.Exec(containerID, []string{"sh", "-c", taintCmd}); err != nil {
		fmt.Printf("  Warning: failed to remove control-plane taint: %v\n", err)
	}

	// Patch kube-proxy to skip privileged sysctl operations
	// This is needed for rootless containers that can't set nf_conntrack_max
	// fmt.Println("  Patching kube-proxy for rootless compatibility...")
	patchCmd := `kubectl get configmap -n kube-system kube-proxy -o yaml | \
	sed 's/maxPerCore:.*/maxPerCore: 0/' | \
	kubectl apply -f - && \
	kubectl rollout restart daemonset/kube-proxy -n kube-system`
	if _, err := podman.Exec(containerID, []string{"sh", "-c", patchCmd}); err != nil {
		fmt.Printf("  Warning: failed to patch kube-proxy: %v\n", err)
	}

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

	style.Step("Deleting %d node(s)... 🗑️", len(containers))
	for _, container := range containers {
		if err := podman.DeleteContainer(container.ID); err != nil {
			return fmt.Errorf("failed to delete container %s: %w", container.Name, err)
		}
		style.Info("Deleted node: %s", container.Name)

		// Try to delete associated storage volume
		volName := fmt.Sprintf("kipod-storage-%s", container.Name)
		// We ignore errors here because the volume might not exist (if using tmpfs)
		// or might have been deleted already.
		_ = podman.DeleteVolume(volName)
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
		// Extract cluster name from labels
		if name, ok := container.Labels[podman.LabelCluster]; ok && name != "" {
			clusterMap[name] = true
		} else {
			// Fallback to extracting from container name
			parts := strings.Split(container.Name, "-")
			if len(parts) > 0 {
				clusterMap[parts[0]] = true
			}
		}
	}

	clusters := make([]string, 0, len(clusterMap))
	for name := range clusterMap {
		clusters = append(clusters, name)
	}

	return clusters, nil
}

// GetKubeconfig retrieves the kubeconfig for a cluster
func GetKubeconfig(name string) (string, error) {
	containers, err := podman.ListContainers(map[string]string{
		podman.LabelCluster: name,
		podman.LabelRole:    "control-plane",
	})
	if err != nil {
		return "", fmt.Errorf("failed to list cluster containers: %w", err)
	}

	if len(containers) == 0 {
		return "", fmt.Errorf("cluster '%s' not found", name)
	}

	// Get kubeconfig from the control-plane node
	kubeconfig, err := podman.Exec(containers[0].ID, []string{"cat", "/etc/kubernetes/admin.conf"})
	if err != nil {
		return "", fmt.Errorf("failed to retrieve kubeconfig: %w", err)
	}

	return kubeconfig, nil
}

func (c *Cluster) runKubeadmInit(containerID string) error {
	// Images will be pulled on-demand by kubeadm (optimized - no pre-loading needed)
	// Initialize Kubernetes using kubeadm
	// Include localhost and 127.0.0.1 in API server certificate SANs for port-forwarded access
	initCmd := fmt.Sprintf(`kubeadm init \
  --pod-network-cidr=%s \
  --service-cidr=%s \
  --cri-socket=unix:///var/run/crio/crio.sock \
  --apiserver-cert-extra-sans=localhost,127.0.0.1 \
  --ignore-preflight-errors=NumCPU,Mem,SystemVerification,FileContent--proc-sys-net-bridge-bridge-nf-call-iptables \
  --v=5`, c.config.PodSubnet, c.config.ServiceSubnet)

	output, err := podman.Exec(containerID, []string{"sh", "-c", initCmd})
	if err != nil {
		return fmt.Errorf("kubeadm init failed: %w\nOutput:\n%s", err, output)
	}
	return nil
}
