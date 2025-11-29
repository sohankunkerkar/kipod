package config

import (
	"fmt"
	"strconv"
	"strings"
)

// ClusterConfig represents the configuration for a kipod cluster
type ClusterConfig struct {
	// APIVersion is the config API version
	APIVersion string `yaml:"apiVersion,omitempty" json:"apiVersion,omitempty"`

	// Kind is the config kind (should be "ClusterConfig")
	Kind string `yaml:"kind,omitempty" json:"kind,omitempty"`

	// Name is the cluster name
	Name string `yaml:"name,omitempty" json:"name,omitempty"`

	// Nodes configuration
	Nodes NodesConfig `yaml:"nodes,omitempty" json:"nodes,omitempty"`

	// Image is the base image to use for nodes
	Image string `yaml:"image,omitempty" json:"image,omitempty"`

	// Versions specifies component versions
	Versions VersionsConfig `yaml:"versions,omitempty" json:"versions,omitempty"`

	// LocalBuilds specifies paths to local development builds
	LocalBuilds LocalBuildsConfig `yaml:"localBuilds,omitempty" json:"localBuilds,omitempty"`

	// Networking configuration
	Networking NetworkingConfig `yaml:"networking,omitempty" json:"networking,omitempty"`

	// CgroupManager to use (cgroupfs or systemd)
	CgroupManager string `yaml:"cgroupManager,omitempty" json:"cgroupManager,omitempty"`

	// CRIOConfig is path to a CRI-O config file to inject into /etc/crio/crio.conf.d/99-user.conf
	CRIOConfig string `yaml:"crioConfig,omitempty" json:"crioConfig,omitempty"`

	// Storage configuration
	Storage StorageConfig `yaml:"storage,omitempty" json:"storage,omitempty"`

	// Deprecated fields (kept for backward compatibility)
	// CRIOVersion is deprecated, use Versions.CRIO instead
	CRIOVersion string `yaml:"crioVersion,omitempty" json:"crioVersion,omitempty"`
	// KubernetesVersion is deprecated, use Versions.Kubernetes instead
	KubernetesVersion string `yaml:"kubernetesVersion,omitempty" json:"kubernetesVersion,omitempty"`
	// PodSubnet is deprecated, use Networking.PodSubnet instead
	PodSubnet string `yaml:"podSubnet,omitempty" json:"podSubnet,omitempty"`
	// ServiceSubnet is deprecated, use Networking.ServiceSubnet instead
	ServiceSubnet string `yaml:"serviceSubnet,omitempty" json:"serviceSubnet,omitempty"`
}

// NodesConfig defines the cluster node topology
type NodesConfig struct {
	// ControlPlanes is the number of control-plane nodes
	ControlPlanes int `yaml:"controlPlanes,omitempty" json:"controlPlanes,omitempty"`

	// Workers is the number of worker nodes
	Workers int `yaml:"workers,omitempty" json:"workers,omitempty"`

	// Deprecated: Total is deprecated, use ControlPlanes + Workers
	Total int `yaml:"total,omitempty" json:"total,omitempty"`
}

// VersionsConfig specifies component versions to install
type VersionsConfig struct {
	// Kubernetes version (e.g., "1.34.2")
	Kubernetes string `yaml:"kubernetes,omitempty" json:"kubernetes,omitempty"`

	// CRIO version (e.g., "1.34" - minor version only)
	CRIO string `yaml:"crio,omitempty" json:"crio,omitempty"`

	// Crun version (e.g., "1.25")
	Crun string `yaml:"crun,omitempty" json:"crun,omitempty"`

	// Runc version (e.g., "1.3.3")
	Runc string `yaml:"runc,omitempty" json:"runc,omitempty"`
}

// LocalBuildsConfig specifies paths to local development builds
// When a local path is provided, it will be mounted/copied into the node image
// This is useful for testing CRI-O, crun, or runc changes
type LocalBuildsConfig struct {
	// CRIOBinary path to local crio binary
	CRIOBinary string `yaml:"crioBinary,omitempty" json:"crioBinary,omitempty"`

	// CRIOSourceDir path to CRI-O source directory (for full install)
	CRIOSourceDir string `yaml:"crioSourceDir,omitempty" json:"crioSourceDir,omitempty"`

	// CrunBinary path to local crun binary
	CrunBinary string `yaml:"crunBinary,omitempty" json:"crunBinary,omitempty"`

	// RuncBinary path to local runc binary
	RuncBinary string `yaml:"runcBinary,omitempty" json:"runcBinary,omitempty"`
}

// NetworkingConfig defines cluster networking
type NetworkingConfig struct {
	// PodSubnet is the subnet used for pod IPs
	PodSubnet string `yaml:"podSubnet,omitempty" json:"podSubnet,omitempty"`

	// ServiceSubnet is the subnet used for service IPs
	ServiceSubnet string `yaml:"serviceSubnet,omitempty" json:"serviceSubnet,omitempty"`

	// DNSdomain is the cluster DNS domain
	DNSDomain string `yaml:"dnsDomain,omitempty" json:"dnsDomain,omitempty"`
}

// StorageConfig defines container storage configuration
type StorageConfig struct {
	// Type of storage: "tmpfs" (default) or "volume"
	Type string `yaml:"type,omitempty" json:"type,omitempty"`

	// Size of storage (e.g. "10G") - primarily for tmpfs
	Size string `yaml:"size,omitempty" json:"size,omitempty"`
}

// DefaultConfig returns a default cluster configuration with latest versions
func DefaultConfig() *ClusterConfig {
	return &ClusterConfig{
		APIVersion: "v1alpha1",
		Kind:       "ClusterConfig",
		Name:       "kipod",
		Nodes: NodesConfig{
			ControlPlanes: 1,
			Workers:       0,
		},
		Image: "", // Will use build.DefaultImageName
		Versions: VersionsConfig{
			Kubernetes: "1.34.2", // Latest K8s (Nov 2025)
			CRIO:       "1.34",   // Latest CRI-O minor
			Crun:       "1.25",   // Latest crun (Nov 2025)
			Runc:       "1.3.3",  // Latest runc with security fixes
		},
		Networking: NetworkingConfig{
			PodSubnet:     "10.244.0.0/16",
			ServiceSubnet: "10.96.0.0/12",
			DNSDomain:     "cluster.local",
		},
		CgroupManager: "cgroupfs", // Default to cgroupfs for rootless
	}
}

// Normalize applies defaults and handles backward compatibility
func (c *ClusterConfig) Normalize() {
	// Set defaults for top-level fields
	if c.APIVersion == "" {
		c.APIVersion = "v1alpha1"
	}
	if c.Kind == "" {
		c.Kind = "ClusterConfig"
	}
	if c.Name == "" {
		c.Name = "kipod"
	}

	// Handle backward compatibility for deprecated fields
	if c.CRIOVersion != "" && c.Versions.CRIO == "" {
		c.Versions.CRIO = c.CRIOVersion
	}
	if c.KubernetesVersion != "" && c.Versions.Kubernetes == "" {
		c.Versions.Kubernetes = c.KubernetesVersion
	}
	if c.PodSubnet != "" && c.Networking.PodSubnet == "" {
		c.Networking.PodSubnet = c.PodSubnet
	}
	if c.ServiceSubnet != "" && c.Networking.ServiceSubnet == "" {
		c.Networking.ServiceSubnet = c.ServiceSubnet
	}

	// Set version defaults
	if c.Versions.Kubernetes == "" {
		c.Versions.Kubernetes = "1.34.2"
	}
	if c.Versions.CRIO == "" {
		c.Versions.CRIO = "1.34"
	}
	if c.Versions.Crun == "" {
		c.Versions.Crun = "1.25"
	}
	if c.Versions.Runc == "" {
		c.Versions.Runc = "1.3.3"
	}

	// Set networking defaults
	if c.Networking.PodSubnet == "" {
		c.Networking.PodSubnet = "10.244.0.0/16"
	}
	if c.Networking.ServiceSubnet == "" {
		c.Networking.ServiceSubnet = "10.96.0.0/12"
	}
	if c.Networking.DNSDomain == "" {
		c.Networking.DNSDomain = "cluster.local"
	}

	// Set node defaults
	if c.Nodes.ControlPlanes == 0 && c.Nodes.Workers == 0 && c.Nodes.Total == 0 {
		c.Nodes.ControlPlanes = 1
	}
	// Handle deprecated Total field
	if c.Nodes.Total > 0 && c.Nodes.ControlPlanes == 0 && c.Nodes.Workers == 0 {
		c.Nodes.ControlPlanes = 1
		c.Nodes.Workers = c.Nodes.Total - 1
	}

	// Set cgroup manager default
	if c.CgroupManager == "" {
		c.CgroupManager = "cgroupfs"
	}

	// Set storage defaults
	if c.Storage.Type == "" {
		c.Storage.Type = "tmpfs"
	}
	if c.Storage.Size == "" {
		c.Storage.Size = "10G"
	}
}

// Validate checks the configuration for errors
func (c *ClusterConfig) Validate() error {
	// Validate node counts
	if c.Nodes.ControlPlanes < 0 {
		return fmt.Errorf("control-plane node count cannot be negative")
	}
	if c.Nodes.Workers < 0 {
		return fmt.Errorf("worker node count cannot be negative")
	}
	if c.Nodes.ControlPlanes == 0 && c.Nodes.Workers == 0 {
		return fmt.Errorf("cluster must have at least one node")
	}

	// Validate cgroup manager
	if c.CgroupManager != "cgroupfs" && c.CgroupManager != "systemd" {
		return fmt.Errorf("cgroup manager must be 'cgroupfs' or 'systemd', got: %s", c.CgroupManager)
	}

	// Validate version compatibility (CRI-O follows Kubernetes n-2 policy)
	if err := validateVersionCompatibility(c.Versions.Kubernetes, c.Versions.CRIO); err != nil {
		return fmt.Errorf("version compatibility check failed: %w", err)
	}

	// Validate local builds exist if specified
	// (actual file existence check would happen during build)

	return nil
}

// validateVersionCompatibility ensures K8s and CRI-O versions are compatible
// CRI-O follows the Kubernetes n-2 release version skew policy
func validateVersionCompatibility(k8sVersion, crioVersion string) error {
	if k8sVersion == "" || crioVersion == "" {
		return nil // Skip validation if versions not specified
	}

	k8sMinor, err := extractMinorVersion(k8sVersion)
	if err != nil {
		return fmt.Errorf("invalid Kubernetes version %q: %w", k8sVersion, err)
	}

	crioMinor, err := extractMinorVersion(crioVersion)
	if err != nil {
		return fmt.Errorf("invalid CRI-O version %q: %w", crioVersion, err)
	}

	// CRI-O supports Kubernetes n-2 (e.g., CRI-O 1.34 supports K8s 1.32, 1.33, 1.34)
	diff := k8sMinor - crioMinor
	if diff < -2 || diff > 0 {
		return fmt.Errorf(
			"CRI-O %s is not compatible with Kubernetes %s (CRI-O follows n-2 policy: CRI-O 1.X supports K8s 1.X, 1.X-1, 1.X-2)",
			crioVersion, k8sVersion,
		)
	}

	return nil
}

// extractMinorVersion extracts the minor version number from a semantic version
// e.g., "1.34.2" -> 34, "1.34" -> 34
func extractMinorVersion(version string) (int, error) {
	// Remove 'v' prefix if present
	version = strings.TrimPrefix(version, "v")

	// Split by '.'
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return 0, fmt.Errorf("invalid version format, expected at least major.minor")
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("invalid minor version: %w", err)
	}

	return minor, nil
}

// TotalNodes returns the total number of nodes
func (c *ClusterConfig) TotalNodes() int {
	return c.Nodes.ControlPlanes + c.Nodes.Workers
}

// HasLocalBuilds returns true if any local builds are configured
func (c *ClusterConfig) HasLocalBuilds() bool {
	return c.LocalBuilds.CRIOBinary != "" ||
		c.LocalBuilds.CRIOSourceDir != "" ||
		c.LocalBuilds.CrunBinary != "" ||
		c.LocalBuilds.RuncBinary != ""
}
