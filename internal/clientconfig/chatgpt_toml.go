package clientconfig

// These narrow TOML editing helpers adapt Ollama's MIT-licensed Codex App
// launcher approach. They preserve unrelated values and comments instead of
// serialising the employee's entire configuration into a new shape.

import (
	"fmt"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

type chatGPTConfig struct {
	values map[string]any
}

func parseChatGPTConfig(text string) (chatGPTConfig, error) {
	values := map[string]any{}
	if strings.TrimSpace(text) != "" {
		if err := toml.Unmarshal([]byte(text), &values); err != nil {
			return chatGPTConfig{}, fmt.Errorf("invalid ChatGPT configuration: %w", err)
		}
	}
	return chatGPTConfig{values: values}, nil
}

func (c chatGPTConfig) string(path ...string) (string, bool) {
	var current any = c.values
	for _, part := range path {
		table, ok := current.(map[string]any)
		if !ok {
			return "", false
		}
		current, ok = table[part]
		if !ok {
			return "", false
		}
	}
	value, ok := current.(string)
	return value, ok
}

func (c chatGPTConfig) exists(path ...string) bool {
	var current any = c.values
	for _, part := range path {
		table, ok := current.(map[string]any)
		if !ok {
			return false
		}
		current, ok = table[part]
		if !ok {
			return false
		}
	}
	return true
}

func chatGPTSetRootString(text, key, value string) string {
	lines := strings.SplitAfter(text, "\n")
	rootEnd := len(lines)
	for index, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "[") {
			rootEnd = index
			break
		}
	}
	assignment := fmt.Sprintf("%s = %q", key, value)
	for index := range rootEnd {
		trimmed := strings.TrimSpace(lines[index])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || !chatGPTRootLineHasKey(trimmed, key) {
			continue
		}
		if strings.HasSuffix(lines[index], "\n") {
			assignment += "\n"
		}
		lines[index] = assignment
		return strings.Join(lines, "")
	}
	root := strings.Join(lines[:rootEnd], "")
	rest := strings.Join(lines[rootEnd:], "")
	if root != "" && !strings.HasSuffix(root, "\n") {
		root += "\n"
	}
	insert := assignment + "\n"
	if rest != "" {
		insert += "\n"
	}
	return root + insert + rest
}

func chatGPTRemoveRoot(text, key string) string {
	lines := strings.SplitAfter(text, "\n")
	rootEnd := len(lines)
	for index, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "[") {
			rootEnd = index
			break
		}
	}
	out := make([]string, 0, len(lines))
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if index < rootEnd && trimmed != "" && !strings.HasPrefix(trimmed, "#") && chatGPTRootLineHasKey(trimmed, key) {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "")
}

func chatGPTRestoreRootString(text, key string, before chatGPTRestoreValue) string {
	if before.Present {
		return chatGPTSetRootString(text, key, before.Value)
	}
	return chatGPTRemoveRoot(text, key)
}

func chatGPTRootLineHasKey(line, key string) bool {
	values := map[string]any{}
	if err := toml.Unmarshal([]byte(line+"\n"), &values); err != nil {
		return false
	}
	_, ok := values[key]
	return ok
}

func chatGPTUpsertSection(text, header string, lines []string) string {
	block := strings.Join(append([]string{header}, lines...), "\n") + "\n"
	target, ok := chatGPTTablePath(header)
	if ok {
		if start, end, found := chatGPTSectionRange(text, target); found {
			return text[:start] + block + text[end:]
		}
	}
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	if text != "" {
		text += "\n"
	}
	return text + block
}

func chatGPTRemoveSection(text, header string) string {
	target, ok := chatGPTTablePath(header)
	if !ok {
		return text
	}
	start, end, found := chatGPTSectionRange(text, target)
	if !found {
		return text
	}
	trimmed := text[:start] + text[end:]
	for strings.Contains(trimmed, "\n\n\n") {
		trimmed = strings.ReplaceAll(trimmed, "\n\n\n", "\n\n")
	}
	return strings.TrimRight(trimmed, "\n") + "\n"
}

func chatGPTSectionRange(text string, target []string) (int, int, bool) {
	lines := strings.SplitAfter(text, "\n")
	offset, start := 0, -1
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "#") {
			offset += len(line)
			continue
		}
		if start >= 0 {
			return start, offset, true
		}
		if path, ok := chatGPTTablePath(trimmed); ok && sameStringPath(path, target) {
			start = offset
		}
		offset += len(line)
	}
	if start >= 0 {
		return start, len(text), true
	}
	return 0, 0, false
}

func chatGPTTablePath(header string) ([]string, bool) {
	trimmed := strings.TrimSpace(header)
	if !strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "[[") {
		return nil, false
	}
	const probe = "__alzette_connect_probe"
	values := map[string]any{}
	if err := toml.Unmarshal([]byte(trimmed+"\n"+probe+" = true\n"), &values); err != nil {
		return nil, false
	}
	return findChatGPTProbe(values, probe, nil)
}

func findChatGPTProbe(value any, probe string, path []string) ([]string, bool) {
	table, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	if found, ok := table[probe].(bool); ok && found {
		return path, true
	}
	for key, child := range table {
		if key == probe {
			continue
		}
		if result, found := findChatGPTProbe(child, probe, append(path, key)); found {
			return result, true
		}
	}
	return nil, false
}

func sameStringPath(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
