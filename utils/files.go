package utils

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func GetAbsPathRelativeTo(path string, reference string) string {
	if filepath.IsAbs(path) {
		return path
	} else {
		return filepath.Join(reference, path)
	}
}

func ValidateEnvironment(dependencies ...string) {
	missingDepencies := make([]string, 0)
	for _, dependency := range dependencies {
		_, err := exec.LookPath(dependency)
		if err != nil {
			missingDepencies = append(missingDepencies, dependency)
		}
	}

	if len(missingDepencies) > 0 {
		fmt.Fprint(os.Stderr, errors.New("Missing dependencies: '"+strings.Join(missingDepencies, ", ")+"' on $PATH"))
		Exit(1)
	}
}
