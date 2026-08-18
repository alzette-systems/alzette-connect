package platform

import (
	"strings"
	"testing"
)

func TestChildEnvironmentRemovesInheritedSecrets(t *testing.T) {
	result := ChildEnvironment([]string{
		"PATH=/bin", "LANG=C", "OPENAI_API_KEY=old", "DEEPSEEK_API_KEY=provider",
		"ALZETTE_REFRESH_TOKEN=refresh", "ALZETTE_OAUTH_STATE=state", "DB_SECRET_FILE=/secret",
		"GITHUB_TOKEN=github", "AWS_SECRET_ACCESS_KEY=aws", "DATABASE_PASSWORD=password",
		"GOOGLE_APPLICATION_CREDENTIALS=/credentials", "SSH_AUTH_SOCK=/agent",
	}, "http://127.0.0.1:43128/v1", "alp_local")
	joined := strings.Join(result, "\n")
	for _, forbidden := range []string{"old", "provider", "refresh", "state", "/secret", "github", "aws", "password", "/credentials", "/agent"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("child environment leaked %q: %s", forbidden, joined)
		}
	}
	for _, expected := range []string{"PATH=/bin", "LANG=C", "OPENAI_BASE_URL=http://127.0.0.1:43128/v1", "OPENAI_API_KEY=alp_local"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("child environment omitted %q: %s", expected, joined)
		}
	}
}
