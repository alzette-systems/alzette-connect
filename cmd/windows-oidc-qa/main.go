//go:build windows

// windows-oidc-qa reads disposable Casdoor QA credentials from stdin. It never
// records credentials, request content, or model output in its evidence.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/alzette-systems/alzette-connect/internal/clientconfig"
	"github.com/alzette-systems/alzette-connect/internal/credentialstore"
	"github.com/alzette-systems/alzette-connect/internal/proxy"
	"github.com/alzette-systems/alzette-connect/internal/session"
)

func main() {
	if handled, err := clientconfig.HandleClaudeCredentialHelper(os.Args); handled {
		if err != nil {
			os.Exit(1)
		}
		return
	}
	report := map[string]any{"observed_at": time.Now().UTC().Format(time.RFC3339), "phase": "starting"}
	err := run(report)
	if err != nil {
		report["phase"] = "failed"
		report["error"] = err.Error()
	} else {
		report["phase"] = "finished"
	}
	if report["native_requests"] != nil {
		data, _ := json.MarshalIndent(report, "", "  ")
		path := `C:\AlzetteQA\native-live-chatgpt-report.json`
		if report["claude_version"] != nil {
			path = `C:\AlzetteQA\native-live-claude-report.json`
		}
		_ = os.WriteFile(path, data, 0o600)
	}
	_ = json.NewEncoder(os.Stdout).Encode(report)
}
func run(report map[string]any) error {
	var input struct {
		Username      string `json:"username"`
		Password      string `json:"password"`
		NativeChatGPT bool   `json:"native_chatgpt"`
		NativeClaude  bool   `json:"native_claude"`
		QAGatewayURL  string `json:"qa_gateway_url"`
		QAControl     bool   `json:"qa_control"`
	}
	if json.NewDecoder(io.LimitReader(os.Stdin, 8192)).Decode(&input) != nil || input.Username == "" || input.Password == "" {
		return errors.New("disposable QA identity required on stdin")
	}
	driver := &liveQACasdoorDriver{username: input.Username, password: input.Password, client: &http.Client{Timeout: 20 * time.Second}}
	input.Username = ""
	input.Password = ""
	store := credentialstore.NewPlatform()
	profile := "windows-live-qa"
	defer func() { report["refresh_deleted"] = store.Delete(context.Background(), profile) == nil }()
	var controlTransport http.RoundTripper = http.DefaultTransport
	if input.QAControl {
		controlTransport = qaControlTransport{}
	}
	s, err := session.New(session.Config{ControlURL: "https://app.alzette.systems", CallbackURL: "http://127.0.0.1:43127/callback", Profile: profile, Store: store, OpenBrowser: driver.Open, LoginTimeout: 45 * time.Second, HTTPClient: &http.Client{Transport: controlTransport, Timeout: 30 * time.Second}})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	if err = s.Connect(ctx); err != nil {
		return fmt.Errorf("Casdoor OIDC: %w", err)
	}
	report["oidc_pkce_callback"] = true
	report["native_credential_store"] = store.Kind()
	contexts := s.Contexts()
	report["context_count"] = len(contexts)
	selected := false
	for _, candidate := range contexts {
		hasA, hasB := false, false
		for _, alias := range candidate.ModelAliases {
			hasA = hasA || alias == "qa-shared-flash"
			hasB = hasB || alias == "qa-shared-pro"
		}
		if candidate.Relationship == "employee" && hasA && hasB {
			_, err = s.SelectContext(candidate.MembershipID)
			selected = err == nil
			break
		}
	}
	if !selected {
		return errors.New("QA employee is not assigned both approved QA routes")
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		report["grant_revoked"] = s.RevokeGrant(cleanup) == nil
	}()
	if input.NativeChatGPT {
		return nativeChatGPT(ctx, s, report, input.QAGatewayURL)
	}
	if input.NativeClaude {
		return nativeClaude(ctx, s, report, input.QAGatewayURL)
	}
	local, err := proxy.Start(proxy.Config{Address: "127.0.0.1:0", Provider: s, AllowedInferencePaths: []string{"/v1/responses"}})
	if err != nil {
		return err
	}
	defer local.Close(context.Background())
	statuses := map[string]int{}
	for _, model := range []string{"qa-shared-flash", "qa-shared-pro"} {
		body, _ := json.Marshal(map[string]any{"model": model, "input": "Reply with OK.", "max_output_tokens": 8, "stream": false})
		r, _ := http.NewRequestWithContext(ctx, http.MethodPost, local.BaseURL()+"/responses", bytes.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+local.Capability())
		r.Header.Set("Content-Type", "application/json")
		response, e := http.DefaultClient.Do(r)
		if e != nil {
			return errors.New("real Responses request failed")
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
		response.Body.Close()
		statuses[model] = response.StatusCode
		report["bounded_responses_statuses"] = statuses
		if response.StatusCode != 200 {
			return fmt.Errorf("real Responses route %s returned %d", model, response.StatusCode)
		}
	}
	if err = s.RevokeGrant(ctx); err != nil {
		return errors.New("remote grant revocation failed")
	}
	report["grant_revoked"] = true
	return nil
}
