package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/alzette-systems/alzette-connect/internal/appstate"
	"github.com/alzette-systems/alzette-connect/internal/credentialstore"
	connectplatform "github.com/alzette-systems/alzette-connect/internal/platform"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "alzette-connect-core:", err)
		os.Exit(1)
	}
}

func run(arguments []string) error {
	flags := flag.NewFlagSet("alzette-connect-core", flag.ContinueOnError)
	control := flags.String("control", "", "canonical Alzette control origin")
	callback := flags.String("callback", "http://127.0.0.1:43127/callback", "registered loopback OAuth callback")
	proxyAddress := flags.String("proxy-address", "127.0.0.1:43128", "stable loopback proxy address")
	profile := flags.String("profile", "default", "local protected-login profile")
	contextID := flags.String("context", "", "exact Alzette membership context when more than one is available")
	memory := flags.Bool("memory-credential-store", false, "explicit demo mode: do not persist the rotating login")
	insecure := flags.Bool("allow-insecure-local", false, "allow loopback HTTP for local development")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *control == "" {
		return errors.New("--control is required")
	}
	childArguments := flags.Args()
	if len(childArguments) == 0 {
		return errors.New("provide a client command after --")
	}
	store := credentialstore.NewPlatform()
	if *memory {
		store = credentialstore.NewMemory()
	}
	state := appstate.New(time.Now())
	runtime, err := appstate.NewRuntime(appstate.RuntimeConfig{
		ControlURL: *control, CallbackURL: *callback, ProxyAddress: *proxyAddress,
		Profile: *profile, AllowInsecure: *insecure, CredentialStore: store,
	}, state)
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := runtime.Connect(ctx, *contextID); err != nil {
		return fmt.Errorf("%s: %w", state.Current().Message, err)
	}
	if err := runtime.StartLaunch(ctx); err != nil {
		return fmt.Errorf("create application session: %w", err)
	}
	defer func() {
		shutdown, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_ = runtime.StopLaunch(shutdown)
		_ = runtime.Stop(shutdown)
	}()
	baseURL, capability, _, ok := runtime.ClientConnection()
	if !ok {
		return errors.New("local client connection is unavailable")
	}
	command := exec.CommandContext(ctx, childArguments[0], childArguments[1:]...)
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	command.Env = connectplatform.ChildEnvironment(os.Environ(), baseURL, capability)
	fmt.Fprintf(os.Stdout, "Connected. Starting %s.\n", commandName(childArguments[0]))
	if err := command.Run(); err != nil {
		return fmt.Errorf("client exited: %w", err)
	}
	return nil
}

func commandName(value string) string {
	value = strings.ReplaceAll(value, "\\", "/")
	if index := strings.LastIndexByte(value, '/'); index >= 0 {
		return value[index+1:]
	}
	return value
}
