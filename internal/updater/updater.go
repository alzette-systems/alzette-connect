package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	defaultRepository = "alzette-systems/alzette-connect"
	maximumAssetSize  = 300 << 20
)

var (
	ErrNoUpdate      = errors.New("Alzette Connect is up to date")
	ErrUnsafeRelease = errors.New("the update release was not trusted")
	ErrUnsupportedOS = errors.New("updates are not supported on this platform")
)

type Release struct {
	Version    string `json:"version"`
	AssetName  string `json:"asset_name"`
	URL        string `json:"url"`
	Digest     string `json:"digest"`
	Size       int64  `json:"size"`
	PageURL    string `json:"page_url"`
	Prerelease bool   `json:"prerelease"`
}

type Client struct {
	currentVersion  string
	repository      string
	operatingSystem string
	architecture    string
	httpClient      *http.Client
	apiURL          string
}

type Options struct {
	CurrentVersion  string
	Repository      string
	OperatingSystem string
	Architecture    string
	HTTPClient      *http.Client
	APIURL          string
}

func New(options Options) (*Client, error) {
	repository := strings.TrimSpace(options.Repository)
	if repository == "" {
		repository = defaultRepository
	}
	if repository != defaultRepository {
		return nil, errors.New("update repository must be the pinned Alzette Connect repository")
	}
	current := normalizeVersion(options.CurrentVersion)
	if _, ok := parseVersion(current); !ok {
		return nil, errors.New("current application version is invalid")
	}
	operatingSystem := options.OperatingSystem
	if operatingSystem == "" {
		operatingSystem = runtime.GOOS
	}
	architecture := options.Architecture
	if architecture == "" {
		architecture = runtime.GOARCH
	}
	if _, err := assetSuffix(operatingSystem, architecture); err != nil {
		return nil, err
	}
	apiURL := strings.TrimSpace(options.APIURL)
	if apiURL == "" {
		apiURL = "https://api.github.com/repos/" + repository + "/releases?per_page=20"
	}
	parsed, err := url.Parse(apiURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host != "api.github.com" || parsed.User != nil || parsed.Fragment != "" {
		return nil, errors.New("update API URL is unsafe")
	}
	httpClient := options.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}
	return &Client{currentVersion: current, repository: repository, operatingSystem: operatingSystem, architecture: architecture, httpClient: httpClient, apiURL: apiURL}, nil
}

func (c *Client) CurrentVersion() string { return c.currentVersion }

type githubRelease struct {
	TagName    string        `json:"tag_name"`
	HTMLURL    string        `json:"html_url"`
	Draft      bool          `json:"draft"`
	Prerelease bool          `json:"prerelease"`
	Assets     []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Digest             string `json:"digest"`
	Size               int64  `json:"size"`
}

func (c *Client) Check(ctx context.Context) (Release, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiURL, nil)
	if err != nil {
		return Release{}, errors.New("prepare update check")
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("User-Agent", "Alzette-Connect/"+c.currentVersion)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return Release{}, errors.New("Alzette Connect could not check for updates")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Release{}, errors.New("the update service returned an unexpected response")
	}
	reader := io.LimitReader(response.Body, 2<<20)
	var releases []githubRelease
	if err := json.NewDecoder(reader).Decode(&releases); err != nil {
		return Release{}, errors.New("the update service returned invalid data")
	}
	current, _ := parseVersion(c.currentVersion)
	var best Release
	for _, candidate := range releases {
		version := normalizeVersion(candidate.TagName)
		if candidate.Draft || candidate.Prerelease && len(current.pre) == 0 || compareVersions(version, c.currentVersion) <= 0 {
			continue
		}
		suffix, err := assetSuffixForVersion(c.operatingSystem, c.architecture, version)
		if err != nil {
			continue
		}
		asset, ok := matchingAsset(candidate.Assets, version, suffix)
		if !ok {
			continue
		}
		pageURL, ok := trustedReleasePage(candidate.HTMLURL, version)
		if !ok {
			continue
		}
		if best.Version == "" || compareVersions(version, best.Version) > 0 {
			best = Release{Version: version, AssetName: asset.Name, URL: asset.BrowserDownloadURL, Digest: asset.Digest, Size: asset.Size, PageURL: pageURL, Prerelease: candidate.Prerelease}
		}
	}
	if best.Version == "" {
		return Release{}, ErrNoUpdate
	}
	return best, nil
}

func matchingAsset(assets []githubAsset, version, suffix string) (githubAsset, bool) {
	want := "Alzette-Connect-" + version + suffix
	for _, asset := range assets {
		if asset.Name != want || asset.Size <= 0 || asset.Size > maximumAssetSize || !validSHA256(asset.Digest) {
			continue
		}
		parsed, err := url.Parse(asset.BrowserDownloadURL)
		prefix := "/" + defaultRepository + "/releases/download/connect-v" + version + "/"
		if err != nil || parsed.Scheme != "https" || parsed.Host != "github.com" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != prefix+asset.Name {
			continue
		}
		return asset, true
	}
	return githubAsset{}, false
}

func trustedReleasePage(raw, version string) (string, bool) {
	parsed, err := url.Parse(raw)
	want := "/" + defaultRepository + "/releases/tag/connect-v" + version
	if err != nil || parsed.Scheme != "https" || parsed.Host != "github.com" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != want {
		return "", false
	}
	return parsed.String(), true
}

func (c *Client) Download(ctx context.Context, release Release) (string, error) {
	suffix, err := assetSuffixForVersion(c.operatingSystem, c.architecture, release.Version)
	if err != nil {
		return "", err
	}
	if _, ok := matchingAsset([]githubAsset{{Name: release.AssetName, BrowserDownloadURL: release.URL, Digest: release.Digest, Size: release.Size}}, release.Version, suffix); !ok {
		return "", ErrUnsafeRelease
	}
	directory, err := os.MkdirTemp("", "alzette-connect-update-")
	if err != nil {
		return "", errors.New("prepare update download")
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		_ = os.RemoveAll(directory)
		return "", errors.New("protect update download")
	}
	path := filepath.Join(directory, release.AssetName)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, release.URL, nil)
	if err != nil {
		_ = os.RemoveAll(directory)
		return "", errors.New("prepare update download")
	}
	request.Header.Set("Accept", "application/octet-stream")
	request.Header.Set("User-Agent", "Alzette-Connect/"+c.currentVersion)
	response, err := c.downloadClient().Do(request)
	if err != nil {
		_ = os.RemoveAll(directory)
		return "", errors.New("Alzette Connect could not download the update")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.ContentLength > maximumAssetSize || (response.ContentLength > 0 && response.ContentLength != release.Size) {
		_ = os.RemoveAll(directory)
		return "", errors.New("the update download returned an unexpected response")
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		_ = os.RemoveAll(directory)
		return "", errors.New("prepare update file")
	}
	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(file, hash), io.LimitReader(response.Body, maximumAssetSize+1))
	closeErr := file.Close()
	wantDigest, _ := hex.DecodeString(strings.TrimPrefix(release.Digest, "sha256:"))
	if copyErr != nil || closeErr != nil || written != release.Size || written > maximumAssetSize || !equalBytes(hash.Sum(nil), wantDigest) {
		_ = os.RemoveAll(directory)
		return "", errors.New("the downloaded update failed integrity verification")
	}
	return path, nil
}

func (c *Client) downloadClient() *http.Client {
	copy := *c.httpClient
	copy.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if len(via) > 5 || request.URL.Scheme != "https" || !trustedDownloadHost(request.URL.Hostname()) {
			return ErrUnsafeRelease
		}
		request.Header.Del("Authorization")
		return nil
	}
	return &copy
}

func trustedDownloadHost(host string) bool {
	return host == "github.com" || host == "release-assets.githubusercontent.com"
}

func validSHA256(value string) bool {
	raw := strings.TrimPrefix(value, "sha256:")
	decoded, err := hex.DecodeString(raw)
	return strings.HasPrefix(value, "sha256:") && err == nil && len(decoded) == sha256.Size
}

func equalBytes(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	var different byte
	for index := range left {
		different |= left[index] ^ right[index]
	}
	return different == 0
}

func assetSuffix(operatingSystem, architecture string) (string, error) {
	arch := map[string]string{"amd64": "x64", "arm64": "arm64"}[architecture]
	if arch == "" {
		return "", ErrUnsupportedOS
	}
	switch operatingSystem {
	case "darwin":
		return "-macOS-" + arch + "-unsigned-demo.zip", nil
	case "windows":
		return "-windows-" + arch + "-unsigned-demo.exe", nil
	case "linux":
		return "-linux-" + arch + "-unsigned-demo.deb", nil
	default:
		return "", ErrUnsupportedOS
	}
}

func assetSuffixForVersion(operatingSystem, architecture, version string) (string, error) {
	parsed, ok := parseVersion(version)
	if !ok {
		return "", ErrUnsafeRelease
	}
	arch := map[string]string{"amd64": "x64", "arm64": "arm64"}[architecture]
	if operatingSystem == "darwin" && arch != "" && len(parsed.pre) == 0 {
		return "-macOS-" + arch + ".zip", nil
	}
	return assetSuffix(operatingSystem, architecture)
}

func normalizeVersion(value string) string {
	return strings.TrimPrefix(strings.TrimSpace(value), "connect-v")
}

type parsedVersion struct {
	major, minor, patch int
	pre                 []string
}

func parseVersion(value string) (parsedVersion, bool) {
	value = normalizeVersion(value)
	parts := strings.SplitN(value, "-", 2)
	core := strings.Split(parts[0], ".")
	if len(core) != 3 {
		return parsedVersion{}, false
	}
	numbers := make([]int, 3)
	for index, raw := range core {
		if raw == "" || (len(raw) > 1 && raw[0] == '0') {
			return parsedVersion{}, false
		}
		number, err := strconv.Atoi(raw)
		if err != nil || number < 0 {
			return parsedVersion{}, false
		}
		numbers[index] = number
	}
	parsed := parsedVersion{major: numbers[0], minor: numbers[1], patch: numbers[2]}
	if len(parts) == 2 {
		if parts[1] == "" {
			return parsedVersion{}, false
		}
		parsed.pre = strings.Split(parts[1], ".")
		for _, identifier := range parsed.pre {
			if identifier == "" {
				return parsedVersion{}, false
			}
			for _, character := range identifier {
				if !(character >= 'A' && character <= 'Z') && !(character >= 'a' && character <= 'z') && !(character >= '0' && character <= '9') && character != '-' {
					return parsedVersion{}, false
				}
			}
		}
	}
	return parsed, true
}

func compareVersions(left, right string) int {
	a, okA := parseVersion(left)
	b, okB := parseVersion(right)
	if !okA || !okB {
		return 0
	}
	for _, pair := range [][2]int{{a.major, b.major}, {a.minor, b.minor}, {a.patch, b.patch}} {
		if pair[0] < pair[1] {
			return -1
		}
		if pair[0] > pair[1] {
			return 1
		}
	}
	if len(a.pre) == 0 && len(b.pre) > 0 {
		return 1
	}
	if len(a.pre) > 0 && len(b.pre) == 0 {
		return -1
	}
	for index := 0; index < len(a.pre) && index < len(b.pre); index++ {
		leftNumber, leftErr := strconv.Atoi(a.pre[index])
		rightNumber, rightErr := strconv.Atoi(b.pre[index])
		if leftErr == nil && rightErr == nil {
			if leftNumber < rightNumber {
				return -1
			}
			if leftNumber > rightNumber {
				return 1
			}
			continue
		}
		if leftErr == nil {
			return -1
		}
		if rightErr == nil {
			return 1
		}
		if a.pre[index] < b.pre[index] {
			return -1
		}
		if a.pre[index] > b.pre[index] {
			return 1
		}
	}
	if len(a.pre) < len(b.pre) {
		return -1
	}
	if len(a.pre) > len(b.pre) {
		return 1
	}
	return 0
}

func (r Release) String() string {
	return fmt.Sprintf("Alzette Connect %s", r.Version)
}
