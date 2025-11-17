package main

import (
	"fmt"

	"github.com/skunkerk/kipod/pkg/build"
)

func buildNodeImage(k8sVersion, crioVersion, imageName, imageTag string) error {
	opts := &build.ImageBuildOptions{
		ImageName:         imageName,
		ImageTag:          imageTag,
		KubernetesVersion: k8sVersion,
		CRIOVersion:       crioVersion,
	}

	if err := build.BuildImage(opts); err != nil {
		return fmt.Errorf("failed to build node image: %w", err)
	}

	return nil
}
