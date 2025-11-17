package main

import (
	"github.com/skunkerk/kipod/pkg/system"
)

func checkSystem() error {
	results, err := system.ValidateSystem()
	if err != nil {
		return err
	}

	system.PrintValidationResults(results)

	return nil
}
