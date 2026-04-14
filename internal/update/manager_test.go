package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCheckWritesStateAndFindsPlatformAsset(t *testing.T) {
	release := Release{
		TagName: "v1.2.0",
		HTMLURL: "https://example.invalid/releases/v1.2.0",
		Assets: []Asset{
			{Name: "gm-v1.2.0-darwin-arm64.tar.gz", BrowserDownloadURL: "https://example.invalid/darwin-arm64.tar.gz"},
		},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/releases/latest" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewEncoder(w).Encode(release); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	manager := newTestManager(t)
	manager.apiBase = server.URL
	manager.goos = "darwin"
	manager.goarch = "arm64"
	manager.currentVersion = "v1.1.0"
	manager.now = func() time.Time { return time.Date(2026, 4, 14, 8, 0, 0, 0, time.UTC) }

	result, err := manager.Check(context.Background(), CheckOptions{})
	if err != nil {
		t.Fatalf("check updates: %v", err)
	}

	if result.Status != "update_available" {
		t.Fatalf("status = %q, want update_available", result.Status)
	}
	if !result.UpdateAvailable {
		t.Fatalf("expected update available")
	}
	if result.AssetName != "gm-v1.2.0-darwin-arm64.tar.gz" {
		t.Fatalf("asset name = %q", result.AssetName)
	}

	state, err := manager.loadState()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if state.LatestVersion != "v1.2.0" {
		t.Fatalf("latest version = %q", state.LatestVersion)
	}
	if !state.UpdateAvailable {
		t.Fatalf("expected cached update availability")
	}
}

func TestMaybeNotifyAndCheckUsesCachedAvailability(t *testing.T) {
	manager := newTestManager(t)
	manager.currentVersion = "v1.0.0"
	manager.now = func() time.Time { return time.Date(2026, 4, 14, 8, 0, 0, 0, time.UTC) }

	if err := manager.saveState(State{
		CheckedAt:       manager.now(),
		CheckedVersion:  "v1.0.0",
		LatestVersion:   "v1.1.0",
		UpdateAvailable: true,
	}); err != nil {
		t.Fatalf("save state: %v", err)
	}

	var spawned []string
	manager.startProcess = func(name string, args ...string) error {
		spawned = append([]string{name}, args...)
		return nil
	}

	manager.MaybeNotifyAndCheck()

	if got := manager.stderr.(*bytes.Buffer).String(); !strings.Contains(got, `gm update available: v1.1.0 (current v1.0.0). Run "gm update apply" to install.`) {
		t.Fatalf("stderr = %q", got)
	}
	if len(spawned) != 0 {
		t.Fatalf("expected no background spawn for fresh cache, got %v", spawned)
	}
}

func TestMaybeNotifyAndCheckStartsBackgroundWhenStale(t *testing.T) {
	manager := newTestManager(t)
	manager.currentVersion = "v1.0.0"
	now := time.Date(2026, 4, 14, 8, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return now }

	if err := manager.saveState(State{
		CheckedAt:      now.Add(-25 * time.Hour),
		CheckedVersion: "v1.0.0",
		LatestVersion:  "v1.0.0",
	}); err != nil {
		t.Fatalf("save state: %v", err)
	}

	var spawned []string
	manager.executablePath = func() (string, error) { return "/tmp/gm", nil }
	manager.startProcess = func(name string, args ...string) error {
		spawned = append([]string{name}, args...)
		return nil
	}

	manager.MaybeNotifyAndCheck()

	want := []string{"/tmp/gm", "update", "check", "--background"}
	if strings.Join(spawned, " ") != strings.Join(want, " ") {
		t.Fatalf("spawned = %v, want %v", spawned, want)
	}
}

func TestApplyDownloadsArchiveVerifiesDigestAndPassesBinary(t *testing.T) {
	archive := mustTarGzBinary(t, "gm", []byte("new-binary"))
	sum := sha256.Sum256(archive)
	digest := "sha256:" + hex.EncodeToString(sum[:])

	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases/latest":
			release := Release{
				TagName: "v1.2.0",
				HTMLURL: "https://example.invalid/releases/v1.2.0",
				Assets: []Asset{
					{
						Name:               "gm-v1.2.0-linux-amd64.tar.gz",
						BrowserDownloadURL: serverURL + "/assets/linux-amd64.tar.gz",
						Digest:             digest,
					},
				},
			}
			if err := json.NewEncoder(w).Encode(release); err != nil {
				t.Fatalf("encode release: %v", err)
			}
		case "/assets/linux-amd64.tar.gz":
			_, _ = w.Write(archive)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	manager := newTestManager(t)
	manager.apiBase = server.URL
	manager.goos = "linux"
	manager.goarch = "amd64"
	manager.currentVersion = "v1.1.0"
	manager.executablePath = func() (string, error) { return "/usr/local/bin/gm", nil }

	var applied []byte
	manager.applyBinary = func(binary []byte) error {
		applied = append([]byte(nil), binary...)
		return nil
	}

	result, err := manager.Apply(context.Background(), "")
	if err != nil {
		t.Fatalf("apply update: %v", err)
	}
	if !result.Updated {
		t.Fatalf("expected update to be applied")
	}
	if string(applied) != "new-binary" {
		t.Fatalf("applied payload = %q", string(applied))
	}
}

func TestApplyReturnsUpToDateWithoutDownload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		release := Release{
			TagName: "v1.2.0",
			Assets: []Asset{
				{Name: "gm-v1.2.0-linux-amd64.tar.gz", BrowserDownloadURL: "https://example.invalid/linux-amd64.tar.gz"},
			},
		}
		if err := json.NewEncoder(w).Encode(release); err != nil {
			t.Fatalf("encode release: %v", err)
		}
	}))
	defer server.Close()

	manager := newTestManager(t)
	manager.apiBase = server.URL
	manager.goos = "linux"
	manager.goarch = "amd64"
	manager.currentVersion = "v1.2.0"
	manager.executablePath = func() (string, error) { return "/usr/local/bin/gm", nil }
	manager.applyBinary = func(binary []byte) error {
		t.Fatalf("applyBinary should not be called when already up to date")
		return nil
	}

	result, err := manager.Apply(context.Background(), "")
	if err != nil {
		t.Fatalf("apply update: %v", err)
	}
	if result.Status != "up_to_date" || result.Updated {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestDownloadAssetRejectsOversizedPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "104857601")
		_, _ = w.Write([]byte("x"))
	}))
	defer server.Close()

	manager := newTestManager(t)
	_, err := manager.downloadAsset(context.Background(), Asset{
		Name:               "gm-v1.2.0-linux-amd64.tar.gz",
		BrowserDownloadURL: server.URL,
	})
	if err == nil || !strings.Contains(err.Error(), "asset too large") {
		t.Fatalf("err = %v, want asset too large", err)
	}
}

func TestExtractZipBinary(t *testing.T) {
	archive := mustZipBinary(t, "gm.exe", []byte("windows-binary"))
	got, err := extractBinary(archive, "windows")
	if err != nil {
		t.Fatalf("extract binary: %v", err)
	}
	if string(got) != "windows-binary" {
		t.Fatalf("binary = %q", string(got))
	}
}

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	dir := t.TempDir()
	return &Manager{
		currentVersion: "v0.0.0",
		client:         &http.Client{Timeout: 2 * time.Second},
		apiBase:        "https://example.invalid",
		cachePath:      filepath.Join(dir, "update.json"),
		lockPath:       filepath.Join(dir, "update.lock"),
		stderr:         &bytes.Buffer{},
		now:            time.Now,
		goos:           "linux",
		goarch:         "amd64",
		executablePath: func() (string, error) { return "/tmp/gm", nil },
		startProcess:   func(string, ...string) error { return nil },
		applyBinary:    func([]byte) error { return nil },
	}
}

func mustTarGzBinary(t *testing.T, name string, payload []byte) []byte {
	t.Helper()
	var archive bytes.Buffer
	gzw := gzip.NewWriter(&archive)
	tw := tar.NewWriter(gzw)
	header := &tar.Header{
		Name: name,
		Mode: 0o755,
		Size: int64(len(payload)),
	}
	if err := tw.WriteHeader(header); err != nil {
		t.Fatalf("write tar header: %v", err)
	}
	if _, err := tw.Write(payload); err != nil {
		t.Fatalf("write tar payload: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	return archive.Bytes()
}

func mustZipBinary(t *testing.T, name string, payload []byte) []byte {
	t.Helper()
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	file, err := zw.Create(name)
	if err != nil {
		t.Fatalf("create zip file: %v", err)
	}
	if _, err := io.Copy(file, bytes.NewReader(payload)); err != nil {
		t.Fatalf("write zip payload: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	return archive.Bytes()
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
