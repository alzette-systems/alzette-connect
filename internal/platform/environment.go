package platform

import "strings"

var deniedEnvironmentNames = map[string]struct{}{
	"OPENAI_API_KEY": {}, "OPENAI_BASE_URL": {},
	"ALZETTE_PI_PROXY_KEY": {}, "ALZETTE_PI_PROXY_URL": {},
	"SSH_AUTH_SOCK": {}, "GPG_AGENT_INFO": {},
}

// ChildEnvironment removes credentials commonly inherited from shells before
// adding the one-session localhost connection. Unknown provider secret
// variables are also removed by suffix rather than copied into the child.
func ChildEnvironment(parent []string, baseURL, capability string) []string {
	result := make([]string, 0, len(parent)+2)
	for _, entry := range parent {
		name := entry
		if index := strings.IndexByte(entry, '='); index >= 0 {
			name = entry[:index]
		}
		upper := strings.ToUpper(name)
		if _, denied := deniedEnvironmentNames[upper]; denied || sensitiveName(upper) {
			continue
		}
		result = append(result, entry)
	}
	return append(result, "OPENAI_BASE_URL="+baseURL, "OPENAI_API_KEY="+capability)
}

func sensitiveName(name string) bool {
	return strings.Contains(name, "OAUTH") ||
		strings.Contains(name, "CREDENTIAL") ||
		strings.Contains(name, "PRIVATE_KEY") ||
		strings.Contains(name, "SECRET") ||
		strings.Contains(name, "PASSWORD") ||
		strings.HasSuffix(name, "_TOKEN") ||
		strings.HasSuffix(name, "_API_KEY") ||
		strings.HasSuffix(name, "_PASS")
}
