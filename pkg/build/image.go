package build

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	// DefaultImageName is the default name for kipod node images
	DefaultImageName = "localhost/kipod-node"

	// DefaultImageTag is the default tag
	DefaultImageTag = "latest"
)

// ImageBuildOptions contains options for building a node image
type ImageBuildOptions struct {
	// ImageName is the name for the built image
	ImageName string

	// ImageTag is the tag for the built image
	ImageTag string

	// BaseDir is the directory containing the Containerfile
	BaseDir string

	// KubernetesVersion is the Kubernetes version to install
	KubernetesVersion string

	// CRIOVersion is the CRI-O version to install
	CRIOVersion string
}

// DefaultImageBuildOptions returns default build options
func DefaultImageBuildOptions() *ImageBuildOptions {
	return &ImageBuildOptions{
		ImageName:         DefaultImageName,
		ImageTag:          DefaultImageTag,
		BaseDir:           "",
		KubernetesVersion: "1.28",
		CRIOVersion:       "1.28",
	}
}

// BuildImage builds a kipod node image using podman build
func BuildImage(opts *ImageBuildOptions) error {
	if opts == nil {
		opts = DefaultImageBuildOptions()
	}

	// Determine base directory
	baseDir := opts.BaseDir
	if baseDir == "" {
		// Try to find the images/base directory relative to the executable
		execPath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to determine executable path: %w", err)
		}
		execDir := filepath.Dir(execPath)

		// Try several possible locations
		possiblePaths := []string{
			filepath.Join(execDir, "..", "images", "base"),
			filepath.Join(execDir, "images", "base"),
			"./images/base",
			"/usr/share/kipod/images/base",
		}

		for _, path := range possiblePaths {
			if _, err := os.Stat(filepath.Join(path, "Containerfile")); err == nil {
				baseDir = path
				break
			}
		}

		if baseDir == "" {
			return fmt.Errorf("could not find Containerfile in any expected location")
		}
	}

	containerfilePath := filepath.Join(baseDir, "Containerfile")
	if _, err := os.Stat(containerfilePath); err != nil {
		return fmt.Errorf("Containerfile not found at %s: %w", containerfilePath, err)
	}

	imageTag := fmt.Sprintf("%s:%s", opts.ImageName, opts.ImageTag)

	fmt.Printf("Building kipod node image: %s\n", imageTag)
	fmt.Printf("Using Containerfile from: %s\n", baseDir)
	fmt.Printf("Kubernetes version: %s\n", opts.KubernetesVersion)
	fmt.Printf("CRI-O version: %s\n", opts.CRIOVersion)
	fmt.Println()

	// Build the image using podman build
	args := []string{
		"build",
		"--tag", imageTag,
		"--build-arg", fmt.Sprintf("K8S_VERSION=%s", opts.KubernetesVersion),
		"--build-arg", fmt.Sprintf("CRIO_VERSION=%s", opts.CRIOVersion),
		"--file", containerfilePath,
		baseDir,
	}

	cmd := exec.Command("podman", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build image: %w", err)
	}

	fmt.Printf("\n✓ Successfully built image: %s\n", imageTag)
	return nil
}

// ImageExists checks if an image exists locally
func ImageExists(imageName string) (bool, error) {
	cmd := exec.Command("podman", "image", "exists", imageName)
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Exit code 1 means image doesn't exist
			if exitErr.ExitCode() == 1 {
				return false, nil
			}
		}
		return false, fmt.Errorf("failed to check if image exists: %w", err)
	}
	return true, nil
}

// GetImageFullName returns the full image name with tag
func GetImageFullName(name, tag string) string {
	if name == "" {
		name = DefaultImageName
	}
	if tag == "" {
		tag = DefaultImageTag
	}
	return fmt.Sprintf("%s:%s", name, tag)
}

// ListImages lists kipod node images
func ListImages() ([]string, error) {
	cmd := exec.Command("podman", "images",
		"--filter", "reference=*/kipod-node:*",
		"--format", "{{.Repository}}:{{.Tag}}")

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %w", err)
	}

	images := []string{}
	lines := string(output)
	if lines != "" {
		for _, line := range []string{lines} {
			if line != "" {
				images = append(images, line)
			}
		}
	}

	return images, nil
}
