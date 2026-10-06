//go:build windows

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/alzette-systems/alzette-connect/internal/clientconfig"
	"github.com/alzette-systems/alzette-connect/internal/proxy"
	"github.com/alzette-systems/alzette-connect/internal/session"
)

type liveTransport struct {
	mu     *sync.Mutex
	report map[string]any
	save   func()
	base   http.RoundTripper
}

func (t liveTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	e := map[string]any{"path": r.URL.Path}
	if r.Body != nil {
		body, err := io.ReadAll(io.LimitReader(r.Body, 32<<20))
		if err != nil {
			return nil, err
		}
		r.Body.Close()
		r.Body = io.NopCloser(bytes.NewReader(body))
		var payload map[string]json.RawMessage
		if json.Unmarshal(body, &payload) == nil {
			var model string
			_ = json.Unmarshal(payload["model"], &model)
			e["model"] = model
			var stream bool
			_ = json.Unmarshal(payload["stream"], &stream)
			e["stream"] = stream
			keys := []string{}
			for key := range payload {
				keys = append(keys, key)
			}
			e["fields"] = keys
		}
	}
	response, err := t.base.RoundTrip(r)
	if response != nil {
		e["status"] = response.StatusCode
		e["request_id"] = response.Header.Get("X-Alzette-Request-ID")
		if response.StatusCode >= 400 && strings.HasPrefix(response.Header.Get("Content-Type"), "application/json") {
			body, _ := io.ReadAll(io.LimitReader(response.Body, 8192))
			_ = response.Body.Close()
			response.Body = io.NopCloser(bytes.NewReader(body))
			var envelope struct {
				Error struct {
					Type    string `json:"type"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if json.Unmarshal(body, &envelope) == nil {
				e["error_type"] = envelope.Error.Type
				e["error_message"] = envelope.Error.Message
			}
		}
	}
	t.mu.Lock()
	t.report["native_requests"] = append(t.report["native_requests"].([]map[string]any), e)
	t.save()
	t.mu.Unlock()
	return response, err
}
func nativeChatGPT(ctx context.Context, s *session.Session, report map[string]any, qaURL string) error {
	var mu sync.Mutex
	save := func() {
		data, _ := json.MarshalIndent(report, "", "  ")
		_ = os.WriteFile(`C:\AlzetteQA\native-live-chatgpt-report.json`, data, 0o600)
	}
	report["native_requests"] = []map[string]any{}
	save()
	executable, err := clientconfig.DiscoverWindowsChatGPT(ctx)
	if err != nil {
		return err
	}
	version, err := clientconfig.ObserveChatGPTVersion(ctx, executable)
	if err != nil {
		return err
	}
	report["chatgpt_version"] = version
	target, err := qaGatewayTarget(qaURL)
	if err != nil {
		return err
	}
	local, err := proxy.Start(proxy.Config{Address: "127.0.0.1:0", Provider: s, AllowedInferencePaths: []string{"/v1/responses"}, HTTPTransport: liveTransport{&mu, report, save, qaGatewayTransport{http.DefaultTransport, target}}})
	if err != nil {
		return err
	}
	defer local.Close(context.Background())
	manager, err := clientconfig.New(clientconfig.Options{})
	if err != nil {
		return err
	}
	if _, err = manager.RecoverChatGPT(ctx, executable); err != nil {
		return err
	}
	connection := clientconfig.Connection{BaseURL: local.BaseURL(), Capability: local.Capability(), Models: s.SelectedModels()}
	for _, model := range s.SelectedModelCatalog() {
		connection.Catalog = append(connection.Catalog, clientconfig.Model{Alias: model.Alias, DisplayName: model.DisplayName, Capabilities: model.Capabilities, ContextWindowTokens: model.ContextWindowTokens})
	}
	result, err := manager.ConfigureChatGPT(ctx, clientconfig.ChatGPTRequest{Connection: connection, ExecutablePath: executable, Version: version})
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
	process, err := clientconfig.LaunchChatGPT(ctx, executable, connection)
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
	timer := time.NewTimer(5 * time.Minute)
	defer timer.Stop()
	for {
		select {
		case <-ticker.C:
			if _, e := os.Stat(filepath.Join(`C:\AlzetteQA`, "stop-live-chatgpt.txt")); e == nil {
				return nil
			}
		case <-timer.C:
			return nil
		case <-process.Done:
			return nil
		case <-ctx.Done():
			return errors.New("native QA timed out")
		}
	}
}
