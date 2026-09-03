package clientconfig

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"time"
)

const PiSupportedVersion = "0.84.2"

var piVersionPattern = regexp.MustCompile(`(?:^|[^0-9])v?(0\.84\.2)(?:[^0-9]|$)`)

// QualifyPi verifies the named release only after an explicit employee launch
// action. Passive application discovery never executes a found binary.
func QualifyPi(ctx context.Context, executable string) (string, error) {
	if !filepath.IsAbs(executable) {
		return "", ErrUnsafePath
	}
	// Race-instrumented and cold-started client binaries can take longer than
	// three seconds to print their version on CI and slower Windows machines.
	check, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	command := exec.CommandContext(check, executable, "--version")
	command.Env = launchEnvironment(os.Environ())
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("%w: Pi version could not be verified", ErrUnsupported)
	}
	match := piVersionPattern.FindSubmatch(output)
	if len(match) != 2 {
		return "", ErrWrongVersion
	}
	return string(match[1]), nil
}

// LaunchPi creates a private, temporary provider extension. The child sees
// only the random loopback capability; OAuth and remote Alzette credentials
// remain owned by Connect.
func LaunchPi(ctx context.Context, executable string, connection Connection) (*Process, error) {
	validated, err := validateConnection(connection)
	if err != nil {
		return nil, err
	}
	if len(validated.Models) == 0 {
		return nil, errors.New("no compatible Alzette model is available")
	}
	temporary, err := os.MkdirTemp("", "alzette-connect-pi-")
	if err != nil {
		return nil, err
	}
	cleanup := func() { _ = os.RemoveAll(temporary) }
	extensionPath := filepath.Join(temporary, "alzette.ts")
	if err := os.WriteFile(extensionPath, []byte(piExtensionSource), 0o600); err != nil {
		cleanup()
		return nil, err
	}
	models, err := json.Marshal(validated.Models)
	if err != nil {
		cleanup()
		return nil, err
	}
	arguments := []string{"--extension", extensionPath, "--provider", "alzette-employee", "--model", validated.Models[0]}
	environment := []string{
		"ALZETTE_PI_PROXY_URL=" + validated.BaseURL,
		"ALZETTE_PI_PROXY_KEY=" + validated.Capability,
		"ALZETTE_PI_MODELS=" + string(models),
	}
	process, err := launchObserved(ctx, executable, arguments, environment, cleanup)
	if err != nil {
		cleanup()
		return nil, err
	}
	return process, nil
}

const piExtensionSource = `import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";

export default function alzetteProvider(pi: ExtensionAPI) {
  const baseUrl = process.env.ALZETTE_PI_PROXY_URL;
  const sessionKey = process.env.ALZETTE_PI_PROXY_KEY;
  if (!baseUrl || !sessionKey) throw new Error("Start Pi from Alzette Connect.");

  let aliases: string[] = [];
  try {
    const parsed = JSON.parse(process.env.ALZETTE_PI_MODELS ?? "[]");
    if (Array.isArray(parsed)) aliases = parsed.filter((value): value is string => typeof value === "string");
  } catch { throw new Error("Alzette model access is invalid."); }
  if (aliases.length === 0) throw new Error("No Alzette models are assigned to this employee.");

  pi.registerProvider("alzette-employee", {
    name: "Alzette employee access",
    baseUrl,
    apiKey: "$ALZETTE_PI_PROXY_KEY",
    authHeader: true,
    api: "openai-completions",
    models: aliases.map((id) => ({
      id, name: id, reasoning: false, input: ["text"],
      cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 },
      contextWindow: 128000, maxTokens: 32768,
      compat: { supportsDeveloperRole: false, supportsUsageInStreaming: false, supportsStore: false, maxTokensField: "max_tokens" },
    })),
  });
}
`
