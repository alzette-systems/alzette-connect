//go:build windows

// windows-chatgpt-qa runs the real desktop application against an isolated
// Responses fixture. It neither signs in to a tenant nor calls a paid model.
// Run from the interactive Windows desktop, then use its UI to send requests.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	"github.com/alzette-systems/alzette-connect/internal/credentialstore"
	"github.com/alzette-systems/alzette-connect/internal/proxy"
)

type provider struct{ base, credential string }

func (p provider) GatewayBaseURL() string { return p.base }
func (p provider) SelectedModels() []string {
	return []string{"alzette-windows-a", "alzette-windows-b"}
}
func (p provider) EnsureHumanCredential(context.Context) (string, time.Time, error) {
	return p.credential, time.Now().Add(time.Hour), nil
}

type report struct {
	ObservedAt                string            `json:"observed_at"`
	Version                   string            `json:"chatgpt_version,omitempty"`
	Phase                     string            `json:"phase"`
	CredentialStore           bool              `json:"native_credential_store"`
	SupervisedLaunch          bool              `json:"supervised_launch"`
	ConfigSecretFree          bool              `json:"config_secret_free"`
	GatewayCredentialVerified bool              `json:"gateway_credential_verified"`
	ToolRoundTrip             bool              `json:"tool_round_trip"`
	Requests                  []requestEvidence `json:"requests"`
	ProfileRestored           bool              `json:"profile_restored"`
	OriginalBytesRestored     bool              `json:"original_bytes_restored"`
	Error                     string            `json:"error,omitempty"`
}

type requestEvidence struct {
	Model         string   `json:"model"`
	Streaming     bool     `json:"streaming"`
	ToolCount     int      `json:"tool_count"`
	ToolNames     []string `json:"tool_names,omitempty"`
	InjectedError bool     `json:"injected_error,omitempty"`
}

func main() {
	dir := flag.String("dir", filepath.Join(os.Getenv("USERPROFILE"), "Desktop", "AlzetteQA"), "QA evidence directory")
	duration := flag.Duration("duration", 12*time.Minute, "maximum interactive test duration")
	flag.Parse()
	if err := os.MkdirAll(*dir, 0o700); err != nil {
		fmt.Println("Cannot create QA directory")
		os.Exit(1)
	}
	var mu sync.Mutex
	r := report{ObservedAt: time.Now().UTC().Format(time.RFC3339), Phase: "starting", Requests: []requestEvidence{}}
	save := func() {
		data, _ := json.MarshalIndent(r, "", "  ")
		_ = os.WriteFile(filepath.Join(*dir, "native-chatgpt-report.json"), append(data, '\n'), 0o600)
	}
	defer save()
	err := run(*dir, *duration, &r, &mu, save)
	if err == nil && r.Error != "" {
		err = errors.New(r.Error)
	}
	if err != nil {
		r.Phase = "failed"
		r.Error = err.Error()
		fmt.Println(r.Error)
		save()
		os.Exit(1)
	}
	r.Phase = "finished"
	fmt.Println("QA finished; inspect native-chatgpt-report.json")
}

func run(dir string, duration time.Duration, r *report, mu *sync.Mutex, save func()) error {
	ctx := context.Background()
	executable, err := clientconfig.DiscoverWindowsChatGPT(ctx)
	if err != nil {
		return fmt.Errorf("registered package discovery: %w", err)
	}
	r.Version, err = clientconfig.ObserveChatGPTVersion(ctx, executable)
	if err != nil {
		return fmt.Errorf("registered version inspection: %w", err)
	}
	store := credentialstore.NewPlatform()
	if store.Kind() != "windows-credential-manager" {
		return errors.New("native Windows credential store unavailable")
	}
	profile := "windows-qa-" + randomHex()
	value := randomHex() + randomHex()
	if err := store.Save(ctx, profile, value); err != nil {
		return err
	}
	defer store.Delete(ctx, profile)
	loaded, err := credentialstore.NewPlatform().Load(ctx, profile)
	if err != nil || loaded != value {
		return errors.New("native credential restart round trip failed")
	}
	if err := store.Delete(ctx, profile); err != nil {
		return err
	}
	if _, err := store.Load(ctx, profile); !errors.Is(err, credentialstore.ErrNotFound) {
		return errors.New("native credential deletion failed")
	}
	r.CredentialStore = true
	remoteCredential := "qa_" + randomHex()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/responses" || request.Header.Get("Authorization") != "Bearer "+remoteCredential {
			http.Error(w, "Fixture credential rejected", 401)
			return
		}
		var body struct {
			Model  string            `json:"model"`
			Stream bool              `json:"stream"`
			Tools  []json.RawMessage `json:"tools"`
			Input  json.RawMessage   `json:"input"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, request.Body, 32<<20)).Decode(&body) != nil {
			http.Error(w, "Invalid fixture request", 400)
			return
		}
		if body.Model != "alzette-windows-a" && body.Model != "alzette-windows-b" {
			http.Error(w, "Unassigned model", 400)
			return
		}
		prompt := lastPrompt(body.Input)
		injectedError := strings.Contains(prompt, "ALZETTE_QA_ERROR")
		tool, names := findShellTool(body.Tools, "")
		var history []struct {
			Type   string          `json:"type"`
			Output json.RawMessage `json:"output"`
		}
		_ = json.Unmarshal(body.Input, &history)
		toolOutput := false
		for _, item := range history {
			if item.Type == "function_call_output" {
				toolOutput = true
			}
			if item.Type == "function_call_output" && strings.Contains(string(item.Output), "ALZETTE_WINDOWS_TOOL_OK") {
				mu.Lock()
				r.ToolRoundTrip = true
				mu.Unlock()
			}
		}
		mu.Lock()
		r.GatewayCredentialVerified = true
		r.Requests = append(r.Requests, requestEvidence{Model: body.Model, Streaming: body.Stream, ToolCount: len(body.Tools), ToolNames: names, InjectedError: injectedError})
		save()
		mu.Unlock()
		if injectedError {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = fmt.Fprintln(w, `{"error":{"type":"server_error","message":"Alzette Windows QA injected upstream error","code":"qa_upstream_error"}}`)
			return
		}
		text := "Windows connection verified through Alzette with " + body.Model + "."
		message := map[string]any{"id": "msg_windows_qa", "type": "message", "status": "completed", "role": "assistant", "content": []any{map[string]any{"type": "output_text", "text": text, "annotations": []any{}}}}
		response := map[string]any{"id": "resp_windows_qa", "object": "response", "created_at": time.Now().Unix(), "status": "completed", "model": body.Model, "output": []any{message}, "usage": map[string]any{"input_tokens": 10, "output_tokens": 12, "total_tokens": 22}}
		if !body.Stream {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		sequence := 0
		emit := func(name string, data map[string]any) {
			data["type"] = name
			data["sequence_number"] = sequence
			sequence++
			encoded, _ := json.Marshal(data)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, encoded)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
		inProgress := map[string]any{"id": "resp_windows_qa", "object": "response", "created_at": time.Now().Unix(), "status": "in_progress", "model": body.Model, "output": []any{}}
		emit("response.created", map[string]any{"response": inProgress})
		emit("response.in_progress", map[string]any{"response": inProgress})
		if strings.Contains(prompt, "ALZETTE_QA_TOOL") && !toolOutput && tool.name != "" {
			arguments := `{"cmd":"echo ALZETTE_WINDOWS_TOOL_OK","yield_time_ms":1000,"max_output_tokens":100}`
			if tool.name == "shell_command" {
				arguments = `{"command":"echo ALZETTE_WINDOWS_TOOL_OK","timeout_ms":1000}`
			}
			item := map[string]any{"id": "fc_windows_qa", "type": "function_call", "status": "in_progress", "call_id": "call_windows_qa", "name": tool.name, "arguments": ""}
			if tool.namespace != "" {
				item["namespace"] = tool.namespace
			}
			emit("response.output_item.added", map[string]any{"output_index": 0, "item": item})
			emit("response.function_call_arguments.delta", map[string]any{"item_id": "fc_windows_qa", "output_index": 0, "delta": arguments})
			emit("response.function_call_arguments.done", map[string]any{"item_id": "fc_windows_qa", "output_index": 0, "arguments": arguments})
			item["arguments"] = arguments
			item["status"] = "completed"
			emit("response.output_item.done", map[string]any{"output_index": 0, "item": item})
			response["output"] = []any{item}
			emit("response.completed", map[string]any{"response": response})
			return
		}
		emit("response.output_item.added", map[string]any{"output_index": 0, "item": map[string]any{"id": "msg_windows_qa", "type": "message", "role": "assistant", "status": "in_progress", "content": []any{}}})
		emit("response.content_part.added", map[string]any{"item_id": "msg_windows_qa", "output_index": 0, "content_index": 0, "part": map[string]any{"type": "output_text", "text": "", "annotations": []any{}}})
		emit("response.output_text.delta", map[string]any{"item_id": "msg_windows_qa", "output_index": 0, "content_index": 0, "delta": text})
		emit("response.output_text.done", map[string]any{"item_id": "msg_windows_qa", "output_index": 0, "content_index": 0, "text": text})
		emit("response.content_part.done", map[string]any{"item_id": "msg_windows_qa", "output_index": 0, "content_index": 0, "part": map[string]any{"type": "output_text", "text": text, "annotations": []any{}}})
		emit("response.output_item.done", map[string]any{"output_index": 0, "item": message})
		emit("response.completed", map[string]any{"response": response})
	}))
	defer upstream.Close()
	local, err := proxy.Start(proxy.Config{Address: "127.0.0.1:0", Provider: provider{base: upstream.URL + "/v1", credential: remoteCredential}, AllowedInferencePaths: []string{"/v1/responses"}})
	if err != nil {
		return err
	}
	defer local.Close(ctx)
	connection := clientconfig.Connection{BaseURL: local.BaseURL(), Capability: local.Capability(), Models: []string{"alzette-windows-a", "alzette-windows-b"}}
	manager, err := clientconfig.New(clientconfig.Options{})
	if err != nil {
		return err
	}
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".codex", "config.toml")
	before, beforeErr := os.ReadFile(configPath)
	if beforeErr != nil && !errors.Is(beforeErr, os.ErrNotExist) {
		return beforeErr
	}
	result, err := manager.ConfigureChatGPT(ctx, clientconfig.ChatGPTRequest{ExecutablePath: executable, Connection: connection, Version: r.Version})
	if err != nil {
		return fmt.Errorf("configure native profile: %w", err)
	}
	defer func() {
		_ = local.Close(ctx)
		if err := result.Rollback(ctx); err != nil {
			r.Error = "Profile rollback failed"
			return
		}
		after, afterErr := os.ReadFile(configPath)
		r.ProfileRestored = !strings.Contains(string(after), clientconfig.ChatGPTProviderID)
		r.OriginalBytesRestored = string(before) == string(after) && (afterErr == nil || errors.Is(beforeErr, os.ErrNotExist) && errors.Is(afterErr, os.ErrNotExist))
	}()
	configured, _ := os.ReadFile(configPath)
	r.ConfigSecretFree = !strings.Contains(string(configured), connection.Capability) && !strings.Contains(string(configured), remoteCredential)
	process, err := clientconfig.LaunchChatGPT(ctx, executable, connection)
	if err != nil {
		return fmt.Errorf("native supervised launch: %w", err)
	}
	r.SupervisedLaunch = true
	r.Phase = "interactive"
	save()
	fmt.Println("ChatGPT started. Choose a folder, send a message using each QA model, then close ChatGPT or create stop-chatgpt-qa.txt in the evidence directory.")
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	timeout := time.NewTimer(duration)
	defer timeout.Stop()
	defer func() {
		stop, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := process.Stop(stop); err != nil {
			r.Error = "Supervised ChatGPT shutdown failed"
		}
	}()
	for {
		select {
		case err := <-process.Done:
			if err != nil {
				return errors.New("supervised ChatGPT process exited with an error")
			}
			return nil
		case <-timeout.C:
			return nil
		case <-ticker.C:
			if _, err := os.Stat(filepath.Join(dir, "stop-chatgpt-qa.txt")); err == nil {
				return nil
			}
		}
	}
}

func randomHex() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("QA random source unavailable")
	}
	return hex.EncodeToString(b[:])
}

func lastPrompt(input json.RawMessage) string {
	var text string
	if json.Unmarshal(input, &text) == nil {
		return text
	}
	var items []struct {
		Role    string `json:"role"`
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	_ = json.Unmarshal(input, &items)
	for i := len(items) - 1; i >= 0; i-- {
		if items[i].Role == "user" {
			var texts []string
			for _, part := range items[i].Content {
				texts = append(texts, part.Text)
			}
			return strings.Join(texts, "\n")
		}
	}
	return ""
}

type shellTool struct{ name, namespace string }

func findShellTool(tools []json.RawMessage, namespace string) (shellTool, []string) {
	var found shellTool
	var names []string
	for _, raw := range tools {
		var tool struct {
			Type  string            `json:"type"`
			Name  string            `json:"name"`
			Tools []json.RawMessage `json:"tools"`
		}
		if json.Unmarshal(raw, &tool) != nil {
			continue
		}
		if tool.Type == "namespace" {
			candidate, nested := findShellTool(tool.Tools, tool.Name)
			names = append(names, nested...)
			if candidate.name != "" {
				found = candidate
			}
			continue
		}
		name := tool.Name
		if namespace != "" {
			name = namespace + "." + name
		}
		names = append(names, name)
		if tool.Type == "function" && (tool.Name == "exec_command" || tool.Name == "shell_command") {
			found = shellTool{name: tool.Name, namespace: namespace}
		}
	}
	return found, names
}
