//go:build windows

// Native Claude Desktop qualification using an isolated Messages fixture.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/alzette-systems/alzette-connect/internal/clientconfig"
	"github.com/alzette-systems/alzette-connect/internal/proxy"
)

type provider struct{ base string }

func (p provider) GatewayBaseURL() string { return p.base }
func (p provider) SelectedModels() []string {
	return []string{"claude-sonnet-4-6", "claude-opus-4-6"}
}
func (p provider) EnsureHumanCredential(context.Context) (string, time.Time, error) {
	return "isolated-claude-fixture", time.Now().Add(time.Hour), nil
}

type evidence struct {
	Model          string          `json:"model"`
	Stream         bool            `json:"stream"`
	Keys           []string        `json:"keys"`
	ToolNames      []string        `json:"tool_names"`
	Thinking       json.RawMessage `json:"thinking,omitempty"`
	OutputConfig   json.RawMessage `json:"output_config,omitempty"`
	Query          string          `json:"query,omitempty"`
	Path           string          `json:"path"`
	CacheHintCount int             `json:"cache_hint_count"`
	ToolFields     []string        `json:"tool_fields,omitempty"`
	ToolRoundTrip  bool            `json:"tool_round_trip"`
	InjectedError  bool            `json:"injected_error"`
}

func main() {
	if handled, err := clientconfig.HandleClaudeCredentialHelper(os.Args); handled {
		if err != nil {
			os.Exit(1)
		}
		return
	}
	dir := flag.String("dir", `C:\AlzetteQA\claude-native`, "evidence directory")
	duration := flag.Duration("duration", 20*time.Minute, "test duration")
	helper := flag.String("helper", "", "Connect executable to qualify as credential helper")
	flag.Parse()
	_ = os.MkdirAll(*dir, 0o700)
	var mu sync.Mutex
	report := map[string]any{"phase": "starting", "observed_at": time.Now().UTC().Format(time.RFC3339), "requests": []evidence{}}
	save := func() {
		data, _ := json.MarshalIndent(report, "", "  ")
		_ = os.WriteFile(filepath.Join(*dir, "native-claude-report.json"), data, 0o600)
	}
	err := run(*dir, *duration, *helper, report, &mu, save)
	if err == nil && report["cleanup_error"] != nil {
		err = fmt.Errorf("native cleanup failed")
	}
	mu.Lock()
	if err != nil {
		report["phase"] = "failed"
		report["error"] = err.Error()
	} else {
		report["phase"] = "finished"
	}
	save()
	mu.Unlock()
}
func run(dir string, duration time.Duration, helper string, report map[string]any, mu *sync.Mutex, save func()) error {
	ctx := context.Background()
	executable, err := clientconfig.DiscoverWindowsClaude(ctx)
	if err != nil {
		return err
	}
	version, err := clientconfig.ObserveWindowsClaudeVersion(ctx, executable)
	if err != nil {
		return err
	}
	report["version"] = version
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]json.RawMessage
		if r.Header.Get("Authorization") != "Bearer isolated-claude-fixture" {
			http.Error(w, "rejected", 401)
			return
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<20)).Decode(&body) != nil {
			http.Error(w, "invalid request", 400)
			return
		}
		e := evidence{Path: r.URL.Path, Query: r.URL.RawQuery, Thinking: body["thinking"], OutputConfig: body["output_config"]}
		_ = json.Unmarshal(body["model"], &e.Model)
		_ = json.Unmarshal(body["stream"], &e.Stream)
		for key := range body {
			e.Keys = append(e.Keys, key)
		}
		encoded, _ := json.Marshal(body)
		e.CacheHintCount = strings.Count(string(encoded), `"cache_control"`)
		var toolObjects []map[string]json.RawMessage
		_ = json.Unmarshal(body["tools"], &toolObjects)
		fields := map[string]bool{}
		for _, item := range toolObjects {
			for key := range item {
				fields[key] = true
			}
		}
		for key := range fields {
			e.ToolFields = append(e.ToolFields, key)
		}
		var tools []struct {
			Name string `json:"name"`
		}
		_ = json.Unmarshal(body["tools"], &tools)
		for _, tool := range tools {
			e.ToolNames = append(e.ToolNames, tool.Name)
		}
		var messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		}
		_ = json.Unmarshal(body["messages"], &messages)
		last := ""
		if len(messages) > 0 {
			last = string(messages[len(messages)-1].Content)
		}
		e.InjectedError = strings.Contains(last, "ALZETTE_QA_ERROR")
		e.ToolRoundTrip = strings.Contains(last, "tool_result") && strings.Contains(last, "ALZETTE_WINDOWS_TOOL_OK")
		mu.Lock()
		report["requests"] = append(report["requests"].([]evidence), e)
		save()
		mu.Unlock()
		if e.InjectedError {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(502)
			_ = json.NewEncoder(w).Encode(map[string]any{"type": "error", "error": map[string]string{"type": "api_error", "message": "Alzette QA upstream failure"}})
			return
		}
		if r.URL.Path != "/v1/messages" {
			http.Error(w, "unsupported fixture route", 404)
			return
		}
		tool := ""
		if strings.Contains(last, "ALZETTE_QA_TOOL") && !strings.Contains(last, "tool_result") {
			for _, name := range e.ToolNames {
				if name == "Bash" || name == "bash" || name == "run_command" || name == "PowerShell" {
					tool = name
					break
				}
			}
		}
		text := "Alzette Windows Claude streaming test passed."
		if e.ToolRoundTrip {
			text = "Alzette Windows Claude tool round trip passed."
		}
		if !e.Stream {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "msg_alzette_qa", "type": "message", "role": "assistant", "model": e.Model, "content": []any{map[string]string{"type": "text", "text": text}}, "stop_reason": "end_turn", "usage": map[string]int{"input_tokens": 10, "output_tokens": 10}})
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		send := func(event string, data any) {
			b, _ := json.Marshal(data)
			_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
			w.(http.Flusher).Flush()
		}
		send("message_start", map[string]any{"type": "message_start", "message": map[string]any{"id": "msg_alzette_qa", "type": "message", "role": "assistant", "model": e.Model, "content": []any{}, "stop_reason": nil, "stop_sequence": nil, "usage": map[string]int{"input_tokens": 10, "output_tokens": 0, "cache_creation_input_tokens": 0, "cache_read_input_tokens": 0}}})
		stop := "end_turn"
		if tool != "" {
			stop = "tool_use"
			send("content_block_start", map[string]any{"type": "content_block_start", "index": 0, "content_block": map[string]any{"type": "tool_use", "id": "tool_alzette_qa", "name": tool, "input": map[string]any{}}})
			input, _ := json.Marshal(map[string]string{"command": "echo ALZETTE_WINDOWS_TOOL_OK", "description": "Verify Alzette Windows tool integration"})
			send("content_block_delta", map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]string{"type": "input_json_delta", "partial_json": string(input)}})
		} else {
			send("content_block_start", map[string]any{"type": "content_block_start", "index": 0, "content_block": map[string]string{"type": "text", "text": ""}})
			for _, part := range []string{text[:len(text)/2], text[len(text)/2:]} {
				send("content_block_delta", map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]string{"type": "text_delta", "text": part}})
				time.Sleep(100 * time.Millisecond)
			}
		}
		send("content_block_stop", map[string]any{"type": "content_block_stop", "index": 0})
		send("message_delta", map[string]any{"type": "message_delta", "delta": map[string]any{"stop_reason": stop, "stop_sequence": nil}, "usage": map[string]int{"output_tokens": 10}})
		send("message_stop", map[string]string{"type": "message_stop"})
	}))
	defer upstream.Close()
	local, err := proxy.Start(proxy.Config{Address: "127.0.0.1:0", Provider: provider{upstream.URL + "/v1"}, AllowedInferencePaths: []string{"/v1/messages"}})
	if err != nil {
		return err
	}
	defer local.Close(ctx)
	manager, err := clientconfig.New(clientconfig.Options{})
	if err != nil {
		return err
	}
	if err = manager.RecoverClaudeWindows(ctx, executable, ""); err != nil {
		return err
	}
	if helper == "" {
		helper, _ = os.Executable()
	}
	result, err := manager.ConfigureClaudeWindows(ctx, clientconfig.ClaudeRequest{Connection: clientconfig.Connection{BaseURL: local.BaseURL(), Capability: local.Capability(), Models: provider{}.SelectedModels()}, ExecutablePath: executable, HelperExecutablePath: helper, Version: version})
	if err != nil {
		return err
	}
	defer func() {
		e := result.Rollback(ctx)
		mu.Lock()
		report["profile_restored"] = e == nil
		if e != nil {
			report["cleanup_error"] = e.Error()
		}
		save()
		mu.Unlock()
	}()
	process, err := clientconfig.LaunchObserved(ctx, executable)
	if err != nil {
		return err
	}
	defer func() {
		stop, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_ = process.Stop(stop)
	}()
	mu.Lock()
	report["phase"] = "running"
	report["supervised_launch"] = true
	save()
	mu.Unlock()
	timer := time.NewTimer(duration)
	defer timer.Stop()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-timer.C:
			return nil
		case <-process.Done:
			return nil
		case <-ticker.C:
			if _, err = os.Stat(filepath.Join(dir, "stop-claude-qa.txt")); err == nil {
				return nil
			}
		}
	}
}
