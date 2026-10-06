//go:build windows

package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/alzette-systems/alzette-connect/internal/clientconfig"
	"github.com/alzette-systems/alzette-connect/internal/proxy"
	"github.com/alzette-systems/alzette-connect/internal/session"
)

// qaGatewayTransport is confined to this VM harness. The product always uses
// the server-advertised HTTPS URL. Only the host's isolated QA listener is valid.
type qaGatewayTransport struct {
	base   http.RoundTripper
	target *url.URL
}

func (t qaGatewayTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if t.target != nil {
		r = r.Clone(r.Context())
		r.URL.Scheme, r.URL.Host = t.target.Scheme, t.target.Host
		r.Host = t.target.Host
	}
	return t.base.RoundTrip(r)
}

type qaControlTransport struct{}

func (qaControlTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Host == "app.alzette.systems" {
		r = r.Clone(r.Context())
		r.URL.Scheme, r.URL.Host = "http", "192.168.139.1:19086"
		r.Host = "app.alzette.systems"
	}
	return http.DefaultTransport.RoundTrip(r)
}

func qaGatewayTarget(qaURL string) (*url.URL, error) {
	if qaURL == "" {
		return nil, nil
	}
	target, err := url.Parse(qaURL)
	if err != nil || target.Scheme != "http" || target.Host != "192.168.139.1:19085" || target.User != nil || target.Path != "" || target.RawQuery != "" || target.Fragment != "" {
		return nil, errors.New("invalid isolated QA gateway")
	}
	return target, nil
}

func nativeClaude(ctx context.Context, s *session.Session, report map[string]any, qaURL string) error {
	var mu sync.Mutex
	save := func() {
		data, _ := json.MarshalIndent(report, "", "  ")
		_ = os.WriteFile(`C:\AlzetteQA\native-live-claude-report.json`, data, 0o600)
	}
	report["native_requests"] = []map[string]any{}
	executable, err := clientconfig.DiscoverWindowsClaude(ctx)
	if err != nil {
		return err
	}
	version, err := clientconfig.ObserveWindowsClaudeVersion(ctx, executable)
	if err != nil {
		return err
	}
	report["claude_version"] = version
	connection := clientconfig.Connection{Models: s.SelectedModels()}
	for _, model := range s.SelectedModelCatalog() {
		connection.Catalog = append(connection.Catalog, clientconfig.Model{Alias: model.Alias, DisplayName: model.DisplayName, Capabilities: model.Capabilities, ContextWindowTokens: model.ContextWindowTokens})
	}
	connection, err = clientconfig.ClaudeConnection(connection)
	if err != nil {
		return err
	}
	report["model_mapping"] = connection.ModelAliases
	var target *url.URL
	if qaURL != "" {
		target, err = url.Parse(qaURL)
		if err != nil || target.Scheme != "http" || target.Host != "192.168.139.1:19085" || target.User != nil || target.Path != "" || target.RawQuery != "" || target.Fragment != "" {
			return errors.New("invalid isolated QA gateway")
		}
	}
	transport := liveTransport{&mu, report, save, qaGatewayTransport{http.DefaultTransport, target}}
	local, err := proxy.Start(proxy.Config{Address: "127.0.0.1:0", Provider: s, AllowedInferencePaths: []string{"/v1/messages"}, ModelAliases: connection.ModelAliases, HTTPTransport: transport})
	if err != nil {
		return err
	}
	defer local.Close(context.Background())
	connection.BaseURL, connection.Capability = local.BaseURL(), local.Capability()
	manager, err := clientconfig.New(clientconfig.Options{})
	if err != nil {
		return err
	}
	if err = manager.RecoverClaudeWindows(ctx, executable, ""); err != nil {
		return err
	}
	helper, _ := os.Executable()
	result, err := manager.ConfigureClaudeWindows(ctx, clientconfig.ClaudeRequest{Connection: connection, ExecutablePath: executable, HelperExecutablePath: helper, Version: version})
	if err != nil {
		return err
	}
	defer func() {
		e := result.Rollback(context.Background())
		mu.Lock()
		report["native_profile_restored"] = e == nil
		save()
		mu.Unlock()
	}()
	process, err := clientconfig.LaunchObserved(ctx, executable)
	if err != nil {
		return err
	}
	defer func() {
		stop, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		e := process.Stop(stop)
		mu.Lock()
		report["native_process_stopped"] = e == nil
		save()
		mu.Unlock()
	}()
	mu.Lock()
	report["phase"] = "native_running"
	save()
	mu.Unlock()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if _, e := os.Stat(`C:\AlzetteQA\stop-live-claude.txt`); e == nil {
				return nil
			}
		case <-process.Done:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
