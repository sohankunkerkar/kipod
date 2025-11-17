package crio

import (
	"fmt"
	"strings"
)

const (
	// CRIOConfigPath is the path to CRI-O config file
	CRIOConfigPath = "/etc/crio/crio.conf"

	// CRIODropinPath is the path to CRI-O dropin directory
	CRIODropinPath = "/etc/crio/crio.conf.d"
)

// Config represents CRI-O configuration
type Config struct {
	// PauseImage is the pause container image
	PauseImage string

	// CgroupManager is the cgroup manager (systemd or cgroupfs)
	CgroupManager string

	// ConmonCgroup is the cgroup for conmon
	ConmonCgroup string

	// RuntimeType is the OCI runtime type
	RuntimeType string

	// RuntimePath is the path to the OCI runtime
	RuntimePath string
}

// DefaultConfig returns default CRI-O configuration
func DefaultConfig() *Config {
	return &Config{
		PauseImage:    "registry.k8s.io/pause:3.9",
		CgroupManager: "cgroupfs",
		ConmonCgroup:  "pod",
		RuntimeType:   "oci",
		RuntimePath:   "/usr/bin/runc",
	}
}

// GenerateConfig generates CRI-O configuration content
func GenerateConfig(cfg *Config) string {
	return fmt.Sprintf(`# CRI-O configuration for kipod
[crio]
  storage_driver = "overlay"

[crio.api]
  listen = "/var/run/crio/crio.sock"

[crio.runtime]
  cgroup_manager = "%s"
  conmon_cgroup = "%s"
  default_runtime = "runc"

[crio.runtime.runtimes.runc]
  runtime_path = "%s"
  runtime_type = "%s"

[crio.image]
  pause_image = "%s"
  pause_command = "/pause"

[crio.network]
  network_dir = "/etc/cni/net.d/"
  plugin_dirs = ["/opt/cni/bin/"]
`,
		cfg.CgroupManager,
		cfg.ConmonCgroup,
		cfg.RuntimePath,
		cfg.RuntimeType,
		cfg.PauseImage,
	)
}

// InstallScript returns a script to install and configure CRI-O
func InstallScript(version string) string {
	return fmt.Sprintf(`#!/bin/bash
set -e

echo "Installing CRI-O %s..."

# Set up repositories (Fedora example)
if [ -f /etc/fedora-release ]; then
    dnf install -y cri-o cri-tools

# Ubuntu/Debian
elif [ -f /etc/debian_version ]; then
    OS="xUbuntu_22.04"
    VERSION="%s"

    echo "deb [signed-by=/usr/share/keyrings/libcontainers-archive-keyring.gpg] https://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/$OS/ /" > /etc/apt/sources.list.d/devel:kubic:libcontainers:stable.list
    echo "deb [signed-by=/usr/share/keyrings/libcontainers-crio-archive-keyring.gpg] https://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable:/cri-o:/$VERSION/$OS/ /" > /etc/apt/sources.list.d/devel:kubic:libcontainers:stable:cri-o:$VERSION.list

    mkdir -p /usr/share/keyrings
    curl -L https://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/$OS/Release.key | gpg --dearmor -o /usr/share/keyrings/libcontainers-archive-keyring.gpg
    curl -L https://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable:/cri-o:/$VERSION/$OS/Release.key | gpg --dearmor -o /usr/share/keyrings/libcontainers-crio-archive-keyring.gpg

    apt-get update
    apt-get install -y cri-o cri-o-runc cri-tools
fi

# Enable and start CRI-O
systemctl daemon-reload
systemctl enable crio
systemctl start crio

echo "CRI-O installed successfully"
`, version, version)
}

// SetupCommands returns commands to configure CRI-O in a container
func SetupCommands() [][]string {
	return [][]string{
		// Create config directories
		{"mkdir", "-p", CRIODropinPath},
		{"mkdir", "-p", "/etc/cni/net.d"},
		{"mkdir", "-p", "/opt/cni/bin"},

		// Install CNI plugins
		{"sh", "-c", "curl -L https://github.com/containernetworking/plugins/releases/download/v1.3.0/cni-plugins-linux-amd64-v1.3.0.tgz | tar -C /opt/cni/bin -xz"},
	}
}

// WriteConfigCommand returns the command to write CRI-O config
func WriteConfigCommand(config string) []string {
	return []string{
		"sh", "-c",
		fmt.Sprintf("cat > %s/99-kipod.conf << 'EOF'\n%s\nEOF", CRIODropinPath, config),
	}
}

// RestartCommand returns the command to restart CRI-O
func RestartCommand() []string {
	return []string{"systemctl", "restart", "crio"}
}

// ConfigureForKubernetes adds Kubernetes-specific CRI-O configuration
func ConfigureForKubernetes() string {
	return `# Kubernetes-specific CRI-O configuration
[crio.runtime]
  # Enable pids limit for pods
  pids_limit = 8192

  # Log level
  log_level = "info"

[crio.network]
  # CNI configuration
  cni_default_network = "kipod"
`
}

// GetCNIConfig returns a basic CNI network configuration
func GetCNIConfig(podSubnet string) string {
	return fmt.Sprintf(`{
  "cniVersion": "0.4.0",
  "name": "kipod",
  "plugins": [
    {
      "type": "bridge",
      "bridge": "cni0",
      "isGateway": true,
      "ipMasq": true,
      "hairpinMode": true,
      "ipam": {
        "type": "host-local",
        "routes": [
          { "dst": "0.0.0.0/0" }
        ],
        "ranges": [
          [{ "subnet": "%s" }]
        ]
      }
    },
    {
      "type": "portmap",
      "capabilities": {
        "portMappings": true
      }
    }
  ]
}`, podSubnet)
}

// WriteCNIConfigCommand returns command to write CNI config
func WriteCNIConfigCommand(config string) []string {
	return []string{
		"sh", "-c",
		fmt.Sprintf("cat > /etc/cni/net.d/10-kipod.conflist << 'EOF'\n%s\nEOF", config),
	}
}

// ValidateCommands returns commands to validate CRI-O installation
func ValidateCommands() [][]string {
	return [][]string{
		{"crictl", "version"},
		{"crictl", "info"},
	}
}

// ConfigureCrictlCommand configures crictl to use CRI-O socket
func ConfigureCrictlCommand() []string {
	crictl := `runtime-endpoint: unix:///var/run/crio/crio.sock
image-endpoint: unix:///var/run/crio/crio.sock
timeout: 10
debug: false
`
	return []string{
		"sh", "-c",
		fmt.Sprintf("mkdir -p /etc && cat > /etc/crictl.yaml << 'EOF'\n%s\nEOF", crictl),
	}
}

// DisableSwapCommands returns commands to disable swap (required for Kubernetes)
func DisableSwapCommands() [][]string {
	return [][]string{
		{"swapoff", "-a"},
		{"sh", "-c", "sed -i '/ swap / s/^\\(.*\\)$/#\\1/g' /etc/fstab || true"},
	}
}

// ConfigureKernelModulesCommands returns commands to load required kernel modules
func ConfigureKernelModulesCommands() [][]string {
	modules := strings.Join([]string{
		"overlay",
		"br_netfilter",
	}, "\n")

	return [][]string{
		{"sh", "-c", fmt.Sprintf("cat > /etc/modules-load.d/kipod.conf << 'EOF'\n%s\nEOF", modules)},
		{"modprobe", "overlay"},
		{"modprobe", "br_netfilter"},
	}
}

// ConfigureSysctlCommands returns commands to configure sysctl for Kubernetes
func ConfigureSysctlCommands() [][]string {
	sysctl := `net.bridge.bridge-nf-call-iptables  = 1
net.bridge.bridge-nf-call-ip6tables = 1
net.ipv4.ip_forward                 = 1
`
	return [][]string{
		{"sh", "-c", fmt.Sprintf("cat > /etc/sysctl.d/99-kubernetes-cri.conf << 'EOF'\n%s\nEOF", sysctl)},
		{"sysctl", "--system"},
	}
}
