package main

import (
	"fmt"

	"github.com/skunkerk/kipod/pkg/build"
	"github.com/skunkerk/kipod/pkg/config"
)

func buildNodeImage(configFile, k8sVersion, crioVersion, image string, rebuild bool) error {
	// Load config from file or use defaults
	var cfg *config.ClusterConfig
	var err error

	if configFile != "" {
		cfg, err = config.LoadFromFile(configFile)
		if err != nil {
			return fmt.Errorf("failed to load config file: %w", err)
		}
		if !quietMode {
			fmt.Printf("Using configuration from: %s\n", configFile)
		}
	} else {
		cfg = config.DefaultConfig()
	}

	// Command-line flags override config file
	finalK8sVersion := cfg.Versions.Kubernetes
	if k8sVersion != "" {
		finalK8sVersion = k8sVersion
	}

	finalCRIOVersion := cfg.Versions.CRIO
	if crioVersion != "" {
		finalCRIOVersion = crioVersion
	}

	// Parse image name and tag from image string (format: name:tag)
	imageName := image
	imageTag := "latest"

	// Split on last : to get name and tag
	if idx := len(image) - 1; idx >= 0 {
		for i := idx; i >= 0; i-- {
			if image[i] == ':' {
				imageName = image[:i]
				imageTag = image[i+1:]
				break
			}
		}
	}

	opts := &build.ImageBuildOptions{
		ImageName:         imageName,
		ImageTag:          imageTag,
		KubernetesVersion: finalK8sVersion,
		CRIOVersion:       finalCRIOVersion,
		Rebuild:           rebuild,
	}

	if err := build.BuildImage(opts); err != nil {
		return fmt.Errorf("failed to build node image: %w", err)
	}

	return nil
}
