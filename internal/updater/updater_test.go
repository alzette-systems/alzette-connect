package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type rewriteTransport struct {
	server string
}

func (t rewriteTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	copy := request.Clone(request.Context())
	copy.URL.Scheme = "http"
	copy.URL.Host = strings.TrimPrefix(t.server, "http://")
	return http.DefaultTransport.RoundTrip(copy)
}

func TestCheckSelectsExactNewerPlatformAsset(t *testing.T) {
	payload := []byte("verified demo package")
	digest := sha256.Sum256(payload)
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/releases":
			response.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(response, `[{"tag_name":"connect-v0.2.0-demo.2","html_url":"https://github.com/alzette-systems/alzette-connect/releases/tag/connect-v0.2.0-demo.2","draft":false,"prerelease":true,"assets":[{"name":"Alzette-Connect-0.2.0-demo.2-macOS-arm64-unsigned-demo.zip","browser_download_url":"https://github.com/alzette-systems/alzette-connect/releases/download/connect-v0.2.0-demo.2/Alzette-Connect-0.2.0-demo.2-macOS-arm64-unsigned-demo.zip","digest":"sha256:%s","size":%d}]}]`, hex.EncodeToString(digest[:]), len(payload))
		case "/alzette-systems/alzette-connect/releases/download/connect-v0.2.0-demo.2/Alzette-Connect-0.2.0-demo.2-macOS-arm64-unsigned-demo.zip":
			response.Header().Set("Content-Length", fmt.Sprint(len(payload)))
			_, _ = response.Write(payload)
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()
	client, err := New(Options{CurrentVersion: "0.2.0-demo.1", OperatingSystem: "darwin", Architecture: "arm64", APIURL: "https://api.github.com/releases", HTTPClient: &http.Client{Transport: rewriteTransport{server: server.URL}}})
	if err != nil {
		t.Fatal(err)
	}
	asset := githubAsset{Name: "Alzette-Connect-0.2.0-demo.2-macOS-arm64-unsigned-demo.zip", BrowserDownloadURL: "https://github.com/alzette-systems/alzette-connect/releases/download/connect-v0.2.0-demo.2/Alzette-Connect-0.2.0-demo.2-macOS-arm64-unsigned-demo.zip", Digest: "sha256:" + hex.EncodeToString(digest[:]), Size: int64(len(payload))}
	if _, ok := matchingAsset([]githubAsset{asset}, "0.2.0-demo.2", "-macOS-arm64-unsigned-demo.zip"); !ok {
		t.Fatal("valid release asset was not accepted")
	}
	if _, ok := trustedReleasePage("https://github.com/alzette-systems/alzette-connect/releases/tag/connect-v0.2.0-demo.2", "0.2.0-demo.2"); !ok {
		t.Fatal("valid release page was not accepted")
	}
	release, err := client.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if release.Version != "0.2.0-demo.2" {
		t.Fatalf("unexpected release: %#v", release)
	}
	path, err := client.Download(context.Background(), release)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(mustOpen(t, path))
	if err != nil || string(got) != string(payload) {
		t.Fatalf("download=%q err=%v", got, err)
	}
}

func TestCheckRejectsWrongAssetDigestOrRepository(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(response, `[{"tag_name":"connect-v9.0.0","html_url":"https://github.com/attacker/repo/releases/tag/connect-v9.0.0","draft":false,"prerelease":true,"assets":[{"name":"Alzette-Connect-9.0.0-macOS-arm64-unsigned-demo.zip","browser_download_url":"https://github.com/attacker/repo/releases/download/connect-v9.0.0/payload.zip","digest":"sha256:00","size":12}]}]`)
	}))
	defer server.Close()
	client, err := New(Options{CurrentVersion: "0.2.0-demo.1", OperatingSystem: "darwin", Architecture: "arm64", APIURL: "https://api.github.com/releases", HTTPClient: &http.Client{Transport: rewriteTransport{server: server.URL}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Check(context.Background()); err != ErrNoUpdate {
		t.Fatalf("got %v, want ErrNoUpdate", err)
	}
}

func TestStableMacChecksSignedReleaseAndIgnoresPreview(t *testing.T) {
	payload := []byte("signed stable package")
	digest := sha256.Sum256(payload)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(response, `[{"tag_name":"connect-v0.3.6-demo.1","html_url":"https://github.com/alzette-systems/alzette-connect/releases/tag/connect-v0.3.6-demo.1","draft":false,"prerelease":true,"assets":[]},{"tag_name":"connect-v0.3.5","html_url":"https://github.com/alzette-systems/alzette-connect/releases/tag/connect-v0.3.5","draft":false,"prerelease":false,"assets":[{"name":"Alzette-Connect-0.3.5-macOS-arm64.zip","browser_download_url":"https://github.com/alzette-systems/alzette-connect/releases/download/connect-v0.3.5/Alzette-Connect-0.3.5-macOS-arm64.zip","digest":"sha256:%s","size":%d}]}]`, hex.EncodeToString(digest[:]), len(payload))
	}))
	defer server.Close()
	client, err := New(Options{CurrentVersion: "0.3.4", OperatingSystem: "darwin", Architecture: "arm64", APIURL: "https://api.github.com/releases", HTTPClient: &http.Client{Transport: rewriteTransport{server: server.URL}}})
	if err != nil {
		t.Fatal(err)
	}
	release, err := client.Check(context.Background())
	if err != nil || release.Version != "0.3.5" || release.Prerelease || release.AssetName != "Alzette-Connect-0.3.5-macOS-arm64.zip" {
		t.Fatalf("stable release=%#v err=%v", release, err)
	}
}

func TestDownloadRejectsTamperedContent(t *testing.T) {
	want := sha256.Sum256([]byte("expected package"))
	tampered := []byte("tampered package")
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Length", fmt.Sprint(len(tampered)))
		_, _ = response.Write(tampered)
	}))
	defer server.Close()
	client, err := New(Options{CurrentVersion: "0.2.0-demo.1", OperatingSystem: "darwin", Architecture: "arm64", HTTPClient: &http.Client{Transport: rewriteTransport{server: server.URL}}})
	if err != nil {
		t.Fatal(err)
	}
	release := Release{
		Version:   "0.2.0-demo.2",
		AssetName: "Alzette-Connect-0.2.0-demo.2-macOS-arm64-unsigned-demo.zip",
		URL:       "https://github.com/alzette-systems/alzette-connect/releases/download/connect-v0.2.0-demo.2/Alzette-Connect-0.2.0-demo.2-macOS-arm64-unsigned-demo.zip",
		Digest:    "sha256:" + hex.EncodeToString(want[:]),
		Size:      int64(len(tampered)),
	}
	if _, err := client.Download(context.Background(), release); err == nil || !strings.Contains(err.Error(), "integrity") {
		t.Fatalf("got %v, want integrity failure", err)
	}
}

func TestVersionOrdering(t *testing.T) {
	tests := []struct {
		left, right string
		want        int
	}{
		{"0.2.0-demo.2", "0.2.0-demo.1", 1},
		{"0.2.0", "0.2.0-demo.9", 1},
		{"0.2.1-demo.1", "0.2.0", 1},
		{"0.2.0-demo.1", "0.2.0-demo.1", 0},
	}
	for _, test := range tests {
		if got := compareVersions(test.left, test.right); got != test.want {
			t.Fatalf("compareVersions(%q,%q)=%d want %d", test.left, test.right, got, test.want)
		}
	}
}

func mustOpen(t *testing.T, path string) *os.File {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close(); _ = os.RemoveAll(filepath.Dir(path)) })
	return file
}
