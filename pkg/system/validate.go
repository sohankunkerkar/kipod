package system

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
)

// ValidationResult represents the result of a validation check
type ValidationResult struct {
	Name    string
	Passed  bool
	Message string
	Fatal   bool
}

// ValidateSystem validates that the host system meets requirements for kipod
func ValidateSystem() ([]ValidationResult, error) {
	results := []ValidationResult{}

	// Check if podman is installed
	results = append(results, checkPodman())

	// Check if running as non-root (rootless mode)
	results = append(results, checkNonRoot())

	// Check subuid/subgid configuration
	results = append(results, checkSubUID())
	results = append(results, checkSubGID())

	// Check cgroup v2
	results = append(results, checkCgroupV2())

	// Check user namespaces
	results = append(results, checkUserNamespaces())

	// Check fuse support
	results = append(results, checkFuseSupport())

	// Check cgroup delegation
	results = append(results, checkCgroupDelegation())

	// Check max user namespaces
	results = append(results, checkMaxUserNamespaces())

	return results, nil
}

func checkPodman() ValidationResult {
	cmd := exec.Command("podman", "--version")
	output, err := cmd.Output()
	if err != nil {
		return ValidationResult{
			Name:    "Podman Installation",
			Passed:  false,
			Message: "Podman is not installed or not in PATH",
			Fatal:   true,
		}
	}

	version := strings.TrimSpace(string(output))
	return ValidationResult{
		Name:    "Podman Installation",
		Passed:  true,
		Message: fmt.Sprintf("Found: %s", version),
		Fatal:   false,
	}
}

func checkNonRoot() ValidationResult {
	currentUser, err := user.Current()
	if err != nil {
		return ValidationResult{
			Name:    "User Check",
			Passed:  false,
			Message: "Could not determine current user",
			Fatal:   true,
		}
	}

	if currentUser.Uid == "0" {
		return ValidationResult{
			Name:    "Rootless Mode",
			Passed:  false,
			Message: "Running as root. Kipod is designed for rootless podman. Run as a regular user.",
			Fatal:   true,
		}
	}

	return ValidationResult{
		Name:    "Rootless Mode",
		Passed:  true,
		Message: fmt.Sprintf("Running as user: %s (UID: %s)", currentUser.Username, currentUser.Uid),
		Fatal:   false,
	}
}

func checkSubUID() ValidationResult {
	currentUser, err := user.Current()
	if err != nil {
		return ValidationResult{
			Name:    "SubUID Check",
			Passed:  false,
			Message: "Could not determine current user",
			Fatal:   true,
		}
	}

	file, err := os.Open("/etc/subuid")
	if err != nil {
		return ValidationResult{
			Name:    "SubUID Configuration",
			Passed:  false,
			Message: "File /etc/subuid not found",
			Fatal:   true,
		}
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, currentUser.Username+":") {
			parts := strings.Split(line, ":")
			if len(parts) >= 3 {
				count, _ := strconv.Atoi(parts[2])
				if count >= 65536 {
					return ValidationResult{
						Name:    "SubUID Configuration",
						Passed:  true,
						Message: fmt.Sprintf("Configured: %s", line),
						Fatal:   false,
					}
				}
			}
		}
	}

	return ValidationResult{
		Name:    "SubUID Configuration",
		Passed:  false,
		Message: fmt.Sprintf("User %s not found in /etc/subuid or insufficient range (need at least 65536)", currentUser.Username),
		Fatal:   true,
	}
}

func checkSubGID() ValidationResult {
	currentUser, err := user.Current()
	if err != nil {
		return ValidationResult{
			Name:    "SubGID Check",
			Passed:  false,
			Message: "Could not determine current user",
			Fatal:   true,
		}
	}

	file, err := os.Open("/etc/subgid")
	if err != nil {
		return ValidationResult{
			Name:    "SubGID Configuration",
			Passed:  false,
			Message: "File /etc/subgid not found",
			Fatal:   true,
		}
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, currentUser.Username+":") {
			parts := strings.Split(line, ":")
			if len(parts) >= 3 {
				count, _ := strconv.Atoi(parts[2])
				if count >= 65536 {
					return ValidationResult{
						Name:    "SubGID Configuration",
						Passed:  true,
						Message: fmt.Sprintf("Configured: %s", line),
						Fatal:   false,
					}
				}
			}
		}
	}

	return ValidationResult{
		Name:    "SubGID Configuration",
		Passed:  false,
		Message: fmt.Sprintf("User %s not found in /etc/subgid or insufficient range (need at least 65536)", currentUser.Username),
		Fatal:   true,
	}
}

func checkCgroupV2() ValidationResult {
	// Check if cgroup v2 is mounted
	data, err := os.ReadFile("/proc/filesystems")
	if err != nil {
		return ValidationResult{
			Name:    "Cgroup v2",
			Passed:  false,
			Message: "Could not read /proc/filesystems",
			Fatal:   false,
		}
	}

	if !strings.Contains(string(data), "cgroup2") {
		return ValidationResult{
			Name:    "Cgroup v2",
			Passed:  false,
			Message: "Cgroup v2 not available in kernel",
			Fatal:   true,
		}
	}

	// Check if cgroup v2 is mounted
	data, err = os.ReadFile("/proc/mounts")
	if err != nil {
		return ValidationResult{
			Name:    "Cgroup v2 Mount",
			Passed:  false,
			Message: "Could not read /proc/mounts",
			Fatal:   false,
		}
	}

	if strings.Contains(string(data), "cgroup2") {
		return ValidationResult{
			Name:    "Cgroup v2",
			Passed:  true,
			Message: "Cgroup v2 is mounted and available",
			Fatal:   false,
		}
	}

	return ValidationResult{
		Name:    "Cgroup v2",
		Passed:  false,
		Message: "Cgroup v2 not mounted (expected at /sys/fs/cgroup)",
		Fatal:   true,
	}
}

func checkUserNamespaces() ValidationResult {
	// Check if user namespaces are enabled
	data, err := os.ReadFile("/proc/sys/kernel/unprivileged_userns_clone")
	if err != nil {
		// File might not exist on all systems, try another check
		data, err = os.ReadFile("/proc/sys/user/max_user_namespaces")
		if err != nil {
			return ValidationResult{
				Name:    "User Namespaces",
				Passed:  true,
				Message: "Cannot verify, but likely enabled (no restrictions found)",
				Fatal:   false,
			}
		}
		maxNS := strings.TrimSpace(string(data))
		if maxNS == "0" {
			return ValidationResult{
				Name:    "User Namespaces",
				Passed:  false,
				Message: "User namespaces are disabled (max_user_namespaces=0)",
				Fatal:   true,
			}
		}
		return ValidationResult{
			Name:    "User Namespaces",
			Passed:  true,
			Message: fmt.Sprintf("Enabled (max_user_namespaces=%s)", maxNS),
			Fatal:   false,
		}
	}

	value := strings.TrimSpace(string(data))
	if value == "0" {
		return ValidationResult{
			Name:    "User Namespaces",
			Passed:  false,
			Message: "User namespaces are disabled. Enable with: sysctl -w kernel.unprivileged_userns_clone=1",
			Fatal:   true,
		}
	}

	return ValidationResult{
		Name:    "User Namespaces",
		Passed:  true,
		Message: "User namespaces enabled",
		Fatal:   false,
	}
}

func checkFuseSupport() ValidationResult {
	// Check if /dev/fuse exists
	if _, err := os.Stat("/dev/fuse"); err != nil {
		return ValidationResult{
			Name:    "FUSE Support",
			Passed:  false,
			Message: "/dev/fuse not found. Install fuse package.",
			Fatal:   true,
		}
	}

	// Check if fuse-overlayfs is installed
	cmd := exec.Command("which", "fuse-overlayfs")
	if err := cmd.Run(); err != nil {
		return ValidationResult{
			Name:    "FUSE Support",
			Passed:  false,
			Message: "fuse-overlayfs not installed. Install it for rootless container storage.",
			Fatal:   true,
		}
	}

	return ValidationResult{
		Name:    "FUSE Support",
		Passed:  true,
		Message: "/dev/fuse and fuse-overlayfs available",
		Fatal:   false,
	}
}

func checkCgroupDelegation() ValidationResult {
	currentUser, err := user.Current()
	if err != nil {
		return ValidationResult{
			Name:    "Cgroup Delegation",
			Passed:  false,
			Message: "Could not determine current user",
			Fatal:   false,
		}
	}

	// Check if user cgroup delegation is configured
	cgroupPath := fmt.Sprintf("/sys/fs/cgroup/user.slice/user-%s.slice", currentUser.Uid)
	if _, err := os.Stat(cgroupPath); err != nil {
		return ValidationResult{
			Name:    "Cgroup Delegation",
			Passed:  false,
			Message: fmt.Sprintf("User cgroup not found at %s. Systemd user session may not be running.", cgroupPath),
			Fatal:   false,
		}
	}

	// Check if controllers are delegated
	controllersPath := fmt.Sprintf("%s/cgroup.controllers", cgroupPath)
	data, err := os.ReadFile(controllersPath)
	if err != nil {
		return ValidationResult{
			Name:    "Cgroup Delegation",
			Passed:  false,
			Message: "Could not read cgroup.controllers",
			Fatal:   false,
		}
	}

	controllers := string(data)
	hasMemory := strings.Contains(controllers, "memory")
	hasCPU := strings.Contains(controllers, "cpu")
	hasPids := strings.Contains(controllers, "pids")

	if hasMemory && hasCPU && hasPids {
		return ValidationResult{
			Name:    "Cgroup Delegation",
			Passed:  true,
			Message: fmt.Sprintf("Controllers delegated: %s", strings.TrimSpace(controllers)),
			Fatal:   false,
		}
	}

	return ValidationResult{
		Name:    "Cgroup Delegation",
		Passed:  false,
		Message: fmt.Sprintf("Some controllers missing. Found: %s (need: memory, cpu, pids)", strings.TrimSpace(controllers)),
		Fatal:   false,
	}
}

func checkMaxUserNamespaces() ValidationResult {
	data, err := os.ReadFile("/proc/sys/user/max_user_namespaces")
	if err != nil {
		return ValidationResult{
			Name:    "Max User Namespaces",
			Passed:  true,
			Message: "Cannot check, but likely sufficient",
			Fatal:   false,
		}
	}

	maxNS, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return ValidationResult{
			Name:    "Max User Namespaces",
			Passed:  false,
			Message: "Could not parse max_user_namespaces value",
			Fatal:   false,
		}
	}

	// We need enough user namespaces for nested containers
	// Recommend at least 15000
	if maxNS < 15000 {
		return ValidationResult{
			Name:    "Max User Namespaces",
			Passed:  false,
			Message: fmt.Sprintf("max_user_namespaces=%d is low. Recommend at least 15000. Set with: sysctl -w user.max_user_namespaces=28633", maxNS),
			Fatal:   false,
		}
	}

	return ValidationResult{
		Name:    "Max User Namespaces",
		Passed:  true,
		Message: fmt.Sprintf("max_user_namespaces=%d (sufficient)", maxNS),
		Fatal:   false,
	}
}

// PrintValidationResults prints validation results in a nice format
func PrintValidationResults(results []ValidationResult) {
	fmt.Println("\n=== System Validation ===\n")

	fatalErrors := false
	warnings := false

	for _, result := range results {
		status := "✓"
		if !result.Passed {
			if result.Fatal {
				status = "✗"
				fatalErrors = true
			} else {
				status = "⚠"
				warnings = true
			}
		}

		fmt.Printf("%s %s: %s\n", status, result.Name, result.Message)
	}

	fmt.Println()

	if fatalErrors {
		fmt.Println("❌ Fatal errors detected. Please fix the issues above before proceeding.")
		os.Exit(1)
	}

	if warnings {
		fmt.Println("⚠️  Warnings detected. Kipod may not work correctly.")
	} else {
		fmt.Println("✅ All checks passed! System is ready for kipod.")
	}
}
