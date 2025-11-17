package config

// ClusterConfig represents the configuration for a kipod cluster
type ClusterConfig struct {
	// Name is the cluster name
	Name string `yaml:"name,omitempty" json:"name,omitempty"`

	// Nodes is the number of nodes in the cluster
	Nodes int `yaml:"nodes,omitempty" json:"nodes,omitempty"`

	// Image is the base image to use for nodes
	Image string `yaml:"image,omitempty" json:"image,omitempty"`

	// CRIOVersion is the CRI-O version to use
	CRIOVersion string `yaml:"crioVersion,omitempty" json:"crioVersion,omitempty"`

	// KubernetesVersion is the Kubernetes version to install
	KubernetesVersion string `yaml:"kubernetesVersion,omitempty" json:"kubernetesVersion,omitempty"`

	// PodSubnet is the subnet used for pod IPs
	PodSubnet string `yaml:"podSubnet,omitempty" json:"podSubnet,omitempty"`

	// ServiceSubnet is the subnet used for service IPs
	ServiceSubnet string `yaml:"serviceSubnet,omitempty" json:"serviceSubnet,omitempty"`
}

// DefaultConfig returns a default cluster configuration
func DefaultConfig() *ClusterConfig {
	return &ClusterConfig{
		Name:              "kipod",
		Nodes:             1,
		Image:             "fedora:latest",
		CRIOVersion:       "1.28",
		KubernetesVersion: "1.28.0",
		PodSubnet:         "10.244.0.0/16",
		ServiceSubnet:     "10.96.0.0/12",
	}
}
