package project

import (
	"fmt"
	"regexp"
	"strings"
)

// ParseProjectName extracts the organization and project name from a string
// formatted as "[organization_name] project_name".
// (The API returns project names in this format as it recognizes the request coming from the CLI.)
// It returns the organization name, project name, or an error if parsing fails.
func ParseProjectName(fullName string) (orgName string, projName string, err error) {
	re := regexp.MustCompile(`^\[(.*?)\]\s*(.*)$`)
	matches := re.FindStringSubmatch(fullName)

	if len(matches) == 3 {
		org := strings.TrimSpace(matches[1])
		proj := strings.TrimSpace(matches[2])
		if org == "" || proj == "" {
			return "", "", fmt.Errorf("invalid project name format: organization or project name is empty in '%s'", fullName)
		}
		return org, proj, nil
	}
	return "", "", fmt.Errorf("could not parse project name into '[organization] project' format: '%s'", fullName)
}
