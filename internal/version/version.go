package version

import "strings"

var buildVersion = "dev"

func String() string {
	resolved := strings.TrimSpace(buildVersion)
	if resolved == "" {
		return "dev"
	}

	return resolved
}
