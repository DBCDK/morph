package utils

import (
	"path/filepath"
)

func GetAbsPathRelativeTo(path string, reference string) string {
	if filepath.IsAbs(path) {
		return path
	} else {
		return filepath.Join(reference, path)
	}
}
