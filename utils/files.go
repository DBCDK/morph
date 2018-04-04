package utils

import (
	"fmt"
	"path/filepath"
)

func GetAbsPathRelativeTo(path string, reference string) string {
	if filepath.IsAbs(path) {
		fmt.Println("abs!")
		return path
	} else {
		return filepath.Join(reference, path)
	}
}
