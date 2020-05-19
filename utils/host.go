package utils

import "strings"

func SplitHost(host string) (string, string) {
	var parts = strings.SplitN(host, ":", 2)
	if len(parts) > 1 {
		return parts[0], parts[1]
	}
	return host, ""
}
