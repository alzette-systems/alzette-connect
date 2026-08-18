//go:build linux

package credentialstore

import (
	"context"
	"reflect"
	"sync"
	"testing"
)

type recordedCall struct {
	arguments []string
	input     string
}

type fakeRunner struct {
	mu      sync.Mutex
	calls   []recordedCall
	outputs [][]byte
	errors  []error
}

func (r *fakeRunner) Run(_ context.Context, arguments []string, input []byte) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, recordedCall{append([]string(nil), arguments...), string(input)})
	index := len(r.calls) - 1
	var output []byte
	var err error
	if index < len(r.outputs) {
		output = r.outputs[index]
	}
	if index < len(r.errors) {
		err = r.errors[index]
	}
	return output, err
}

func TestLinuxSecretServicePassesCredentialOnlyOnStdin(t *testing.T) {
	runner := &fakeRunner{outputs: [][]byte{nil, []byte("refresh-secret-value\n"), nil}}
	store := &LinuxSecretService{runner: runner, runtimeDir: t.TempDir()}
	if err := store.Save(context.Background(), "work", "refresh-secret-value"); err != nil {
		t.Fatal(err)
	}
	value, err := store.Load(context.Background(), "work")
	if err != nil || value != "refresh-secret-value" {
		t.Fatalf("load value=%q err=%v", value, err)
	}
	if err := store.Delete(context.Background(), "work"); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 3 || runner.calls[0].input != "refresh-secret-value\n" {
		t.Fatalf("calls=%#v", runner.calls)
	}
	for _, argument := range runner.calls[0].arguments {
		if argument == "refresh-secret-value" {
			t.Fatal("credential appeared in process arguments")
		}
	}
	if !reflect.DeepEqual(runner.calls[1].arguments, []string{"lookup", "service", "alzette-connect", "profile", "work"}) {
		t.Fatalf("lookup args=%q", runner.calls[1].arguments)
	}
}
