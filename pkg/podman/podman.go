package podman

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

const (
	// LabelCluster is the label key for cluster name
	LabelCluster = "io.kipod.cluster"
	// LabelRole is the label key for node role
	LabelRole = "io.kipod.role"
)

// Container represents a podman container
type Container struct {
	ID     string
	Name   string
	Labels map[string]string
}

// CreateContainerOptions contains options for creating a container
type CreateContainerOptions struct {
	Name         string
	Image        string
	Labels       map[string]string
	Privileged   bool
	Volumes      []string
	Hostname     string
	Tmpfs        []string
	Cgroupns     string
	Rootless     bool
	SecurityOpts []string
	Devices      []string
	Sysctls      map[string]string
	Env          []string
	Ports        []string // Port mappings in format "hostPort:containerPort"
	Network      string
}

// CreateContainer creates a new podman container
func CreateContainer(opts CreateContainerOptions) (string, error) {
	args := []string{
		"run", "-d",
		"--name", opts.Name,
	}

	// Always use --privileged for node containers (required for kubelet)
	// even in rootless podman mode
	args = append(args, "--privileged")

	// Enable systemd in container
	args = append(args, "--systemd=always")

	// Increase file descriptor limit for CRI-O
	args = append(args, "--ulimit", "nofile=65536:65536")

	// Cgroup namespace mode
	if opts.Cgroupns != "" {
		args = append(args, "--cgroupns", opts.Cgroupns)
	} else {
		args = append(args, "--cgroupns=private")
	}

	// Security options
	for _, secOpt := range opts.SecurityOpts {
		args = append(args, "--security-opt", secOpt)
	}

	// Device access
	for _, dev := range opts.Devices {
		args = append(args, "--device", dev)
	}

	// Tmpfs mounts
	for _, tmpfs := range opts.Tmpfs {
		args = append(args, "--tmpfs", tmpfs)
	}

	if opts.Hostname != "" {
		args = append(args, "--hostname", opts.Hostname)
	}

	// Labels
	for k, v := range opts.Labels {
		args = append(args, "--label", fmt.Sprintf("%s=%s", k, v))
	}

	// Volumes (additional to those added in rootless mode)
	for _, vol := range opts.Volumes {
		// Skip /sys/fs/cgroup if we already added it in rootless mode
		if opts.Rootless && strings.Contains(vol, "/sys/fs/cgroup") {
			continue
		}
		args = append(args, "-v", vol)
	}

	// Sysctl settings (for kernel parameters)
	for k, v := range opts.Sysctls {
		args = append(args, "--sysctl", fmt.Sprintf("%s=%s", k, v))
	}

	// Environment variables
	for _, env := range opts.Env {
		args = append(args, "-e", env)
	}

	// Port mappings
	for _, port := range opts.Ports {
		args = append(args, "-p", port)
	}

	// Network
	if opts.Network != "" {
		args = append(args, "--network", opts.Network)
	}

	// Image and command
	args = append(args, opts.Image)

	cmd := exec.Command("podman", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w\nOutput: %s", err, output)
	}

	containerID := strings.TrimSpace(string(output))
	return containerID, nil
}

// DeleteContainer deletes a podman container
func DeleteContainer(nameOrID string) error {
	cmd := exec.Command("podman", "rm", "-f", nameOrID)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to delete container: %w\nOutput: %s", err, output)
	}
	return nil
}

// ListContainers lists containers with specific labels
func ListContainers(labels map[string]string) ([]Container, error) {
	args := []string{"ps", "-a", "--format", "{{.ID}}\t{{.Names}}\t{{json .Labels}}"}

	for k, v := range labels {
		args = append(args, "--filter", fmt.Sprintf("label=%s=%s", k, v))
	}

	cmd := exec.Command("podman", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w\nOutput: %s", err, output)
	}

	var containers []Container
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) >= 2 {
			container := Container{
				ID:     parts[0],
				Name:   parts[1],
				Labels: make(map[string]string),
			}

			if len(parts) >= 3 {
				labelStr := parts[2]
				if labelStr != "" {
					if err := json.Unmarshal([]byte(labelStr), &container.Labels); err != nil {
						// Ignore parsing errors, just log or skip
						// fmt.Printf("Warning: failed to parse labels for container %s: %v\n", container.ID, err)
					}
				}
			}
			containers = append(containers, container)
		}
	}

	return containers, nil
}

// Exec executes a command in a container
func Exec(containerID string, cmd []string) (string, error) {
	args := append([]string{"exec", containerID}, cmd...)
	execCmd := exec.Command("podman", args...)

	var stdout, stderr bytes.Buffer
	execCmd.Stdout = &stdout
	execCmd.Stderr = &stderr

	if err := execCmd.Run(); err != nil {
		return "", fmt.Errorf("failed to exec command: %w\nStderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// ExecInteractive executes a command in a container interactively
func ExecInteractive(containerID string, cmd []string) error {
	args := append([]string{"exec", "-it", containerID}, cmd...)
	execCmd := exec.Command("podman", args...)
	execCmd.Stdin = nil
	execCmd.Stdout = nil
	execCmd.Stderr = nil

	return execCmd.Run()
}

// GetContainerIP returns the IP address of a container
func GetContainerIP(containerID string) (string, error) {
	cmd := exec.Command("podman", "inspect", "-f", "{{.NetworkSettings.IPAddress}}", containerID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get container IP: %w\nOutput: %s", err, output)
	}

	return strings.TrimSpace(string(output)), nil
}

// NetworkExists checks if a network exists
func NetworkExists(name string) (bool, error) {
	cmd := exec.Command("podman", "network", "exists", name)
	if err := cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok && exitError.ExitCode() == 1 {
			return false, nil
		}
		return false, fmt.Errorf("failed to check network existence: %w", err)
	}
	return true, nil
}

// CreateNetwork creates a new podman network
func CreateNetwork(name string) error {
	cmd := exec.Command("podman", "network", "create", name)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create network: %w\nOutput: %s", err, output)
	}
	return nil
}

// DeleteVolume deletes a podman volume
func DeleteVolume(name string) error {
	cmd := exec.Command("podman", "volume", "rm", "-f", name)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to delete volume: %w\nOutput: %s", err, output)
	}
	return nil
}
