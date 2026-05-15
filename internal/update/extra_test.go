package update

// Tests for previously zero/low-covered functions:
//   fetchReleaseByTag, tryAcquireLock, compareVersions, canonicalVersionTag,
//   updateStatus, shouldAutoCheck, NewManager

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// fetchReleaseByTag — via Apply with an explicit version
// ---------------------------------------------------------------------------

func TestApplyWithSpecificVersion(t *testing.T) {
	archive := mustTarGzBinary(t, "gm", []byte("tagged-binary"))
	sum := sha256.Sum256(archive)
	digest := "sha256:" + hex.EncodeToString(sum[:])

	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases/tags/v1.3.0":
			release := Release{
				TagName: "v1.3.0",
				HTMLURL: "https://example.invalid/releases/v1.3.0",
				Assets: []Asset{
					{
						Name:               "gm-v1.3.0-linux-amd64.tar.gz",
						BrowserDownloadURL: serverURL + "/assets/v1.3.0-linux-amd64.tar.gz",
						Digest:             digest,
					},
				},
			}
			if err := json.NewEncoder(w).Encode(release); err != nil {
				t.Fatalf("encode release: %v", err)
			}
		case "/assets/v1.3.0-linux-amd64.tar.gz":
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

	// Pass explicit version — exercises fetchReleaseByTag
	result, err := manager.Apply(context.Background(), "v1.3.0")
	if err != nil {
		t.Fatalf("Apply with explicit version: %v", err)
	}
	if !result.Updated {
		t.Fatalf("expected update to be applied")
	}
	if string(applied) != "tagged-binary" {
		t.Fatalf("applied payload = %q, want tagged-binary", string(applied))
	}
	if result.InstalledVersion != "v1.3.0" {
		t.Errorf("InstalledVersion = %q, want v1.3.0", result.InstalledVersion)
	}
}

func TestApplyWithVersionWithoutVPrefix(t *testing.T) {
	// canonicalVersionTag should add the "v" prefix when missing
	archive := mustTarGzBinary(t, "gm", []byte("bin"))
	sum := sha256.Sum256(archive)
	digest := "sha256:" + hex.EncodeToString(sum[:])

	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases/tags/v2.0.0":
			release := Release{
				TagName: "v2.0.0",
				Assets: []Asset{
					{
						Name:               "gm-v2.0.0-linux-amd64.tar.gz",
						BrowserDownloadURL: serverURL + "/bin",
						Digest:             digest,
					},
				},
			}
			json.NewEncoder(w).Encode(release)
		case "/bin":
			w.Write(archive)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	manager := newTestManager(t)
	manager.apiBase = server.URL
	manager.goos = "linux"
	manager.goarch = "amd64"
	manager.currentVersion = "v1.0.0"
	manager.executablePath = func() (string, error) { return "/usr/local/bin/gm", nil }
	manager.applyBinary = func(_ []byte) error { return nil }

	_, err := manager.Apply(context.Background(), "2.0.0") // no v prefix
	if err != nil {
		t.Fatalf("Apply without v prefix: %v", err)
	}
}

// ---------------------------------------------------------------------------
// tryAcquireLock
// ---------------------------------------------------------------------------

func TestTryAcquireLock_AcquireAndRelease(t *testing.T) {
	manager := newTestManager(t)

	unlock, ok, err := manager.tryAcquireLock()
	if err != nil {
		t.Fatalf("tryAcquireLock: %v", err)
	}
	if !ok {
		t.Fatal("expected lock to be acquired")
	}

	// Lock file must exist
	if _, err := os.Stat(manager.lockPath); err != nil {
		t.Fatalf("lock file should exist: %v", err)
	}

	unlock()

	// Lock file must be removed
	if _, err := os.Stat(manager.lockPath); !os.IsNotExist(err) {
		t.Fatal("lock file should be removed after unlock")
	}
}

func TestTryAcquireLock_SecondAcquireFails(t *testing.T) {
	manager := newTestManager(t)

	unlock, ok, err := manager.tryAcquireLock()
	if err != nil {
		t.Fatalf("first tryAcquireLock: %v", err)
	}
	if !ok {
		t.Fatal("expected first lock to succeed")
	}
	defer unlock()

	// Second attempt on the same lock path should return ok=false
	_, ok2, err2 := manager.tryAcquireLock()
	if err2 != nil {
		t.Fatalf("second tryAcquireLock: %v", err2)
	}
	if ok2 {
		t.Fatal("expected second lock to fail while first is held")
	}
}

func TestTryAcquireLock_StaleLocksAreEvicted(t *testing.T) {
	manager := newTestManager(t)
	// Create a stale lock file (modification time older than TTL)
	if err := os.MkdirAll(filepath.Dir(manager.lockPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(manager.lockPath, []byte("stale"), 0o644); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	// Set mtime far in the past
	past := time.Now().Add(-30 * time.Minute)
	if err := os.Chtimes(manager.lockPath, past, past); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	// Override now() to return a time that makes the lock appear stale
	manager.now = func() time.Time { return time.Now() }

	unlock, ok, err := manager.tryAcquireLock()
	if err != nil {
		t.Fatalf("tryAcquireLock with stale lock: %v", err)
	}
	if !ok {
		t.Fatal("expected stale lock to be evicted and new lock acquired")
	}
	unlock()
}

// ---------------------------------------------------------------------------
// compareVersions — additional branches
// ---------------------------------------------------------------------------

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		current, latest string
		wantCmp         int
		wantOK          bool
	}{
		// major difference
		{"v1.0.0", "v2.0.0", -1, true},
		{"v2.0.0", "v1.0.0", 1, true},
		// minor difference
		{"v1.1.0", "v1.2.0", -1, true},
		{"v1.2.0", "v1.1.0", 1, true},
		// patch difference
		{"v1.0.0", "v1.0.1", -1, true},
		{"v1.0.1", "v1.0.0", 1, true},
		// equal
		{"v1.2.3", "v1.2.3", 0, true},
		// unparseable inputs
		{"invalid", "v1.0.0", 0, false},
		{"v1.0.0", "invalid", 0, false},
		{"", "v1.0.0", 0, false},
	}

	for _, tc := range cases {
		got, ok := compareVersions(tc.current, tc.latest)
		if ok != tc.wantOK {
			t.Errorf("compareVersions(%q, %q): ok=%v, want %v", tc.current, tc.latest, ok, tc.wantOK)
			continue
		}
		if ok && got != tc.wantCmp {
			t.Errorf("compareVersions(%q, %q): cmp=%d, want %d", tc.current, tc.latest, got, tc.wantCmp)
		}
	}
}

// ---------------------------------------------------------------------------
// canonicalVersionTag
// ---------------------------------------------------------------------------

func TestCanonicalVersionTag(t *testing.T) {
	cases := []struct {
		input, want string
	}{
		{"", ""},
		{"v1.2.3", "v1.2.3"},
		{"1.2.3", "v1.2.3"},     // no v → add v
		{"  1.2.3  ", "v1.2.3"}, // trim whitespace
		{"  v1.2.3  ", "v1.2.3"},
		{"not-a-version", "not-a-version"}, // non-semver kept as-is
		{"  ", ""},                         // whitespace only → empty
	}

	for _, tc := range cases {
		got := canonicalVersionTag(tc.input)
		if got != tc.want {
			t.Errorf("canonicalVersionTag(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// updateStatus
// ---------------------------------------------------------------------------

func TestUpdateStatus(t *testing.T) {
	cases := []struct {
		current, latest string
		wantStatus      string
		wantAvail       bool
	}{
		{"v1.0.0", "", "unknown", false},
		{"v1.0.0", "v1.0.0", "up_to_date", false},
		{"v1.0.0", "v1.1.0", "update_available", true},
		{"v1.1.0", "v1.0.0", "up_to_date", false},
		// current > latest (dev build)
		{"v2.0.0", "v1.9.9", "up_to_date", false},
		// unparseable → update_available
		{"invalid", "v1.0.0", "update_available", true},
		{"", "v1.0.0", "update_available", true},
	}

	for _, tc := range cases {
		status, avail := updateStatus(tc.current, tc.latest)
		if status != tc.wantStatus || avail != tc.wantAvail {
			t.Errorf("updateStatus(%q, %q) = (%q, %v), want (%q, %v)",
				tc.current, tc.latest, status, avail, tc.wantStatus, tc.wantAvail)
		}
	}
}

// ---------------------------------------------------------------------------
// shouldAutoCheck
// ---------------------------------------------------------------------------

func TestShouldAutoCheck(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name    string
		state   *State
		version string
		want    bool
	}{
		{"nil state", nil, "v1.0.0", true},
		{"zero checked_at", &State{CheckedVersion: "v1.0.0"}, "v1.0.0", true},
		{"version mismatch", &State{
			CheckedAt:      now.Add(-1 * time.Hour),
			CheckedVersion: "v0.9.0",
		}, "v1.0.0", true},
		{"fresh cache same version", &State{
			CheckedAt:      now.Add(-1 * time.Hour),
			CheckedVersion: "v1.0.0",
		}, "v1.0.0", false},
		{"stale cache same version", &State{
			CheckedAt:      now.Add(-25 * time.Hour),
			CheckedVersion: "v1.0.0",
		}, "v1.0.0", true},
	}

	for _, tc := range cases {
		got := shouldAutoCheck(tc.state, tc.version, now)
		if got != tc.want {
			t.Errorf("%s: shouldAutoCheck = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// NewManager smoke-test (covers defaultCacheDir, defaultApplyBinary path)
// ---------------------------------------------------------------------------

func TestNewManager(t *testing.T) {
	m := NewManager("v1.0.0")
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
	if m.currentVersion != "v1.0.0" {
		t.Errorf("currentVersion = %q, want v1.0.0", m.currentVersion)
	}
	if m.client == nil {
		t.Error("expected non-nil http client")
	}
	if !strings.Contains(m.apiBase, "github") {
		t.Errorf("apiBase = %q, expected GitHub URL", m.apiBase)
	}
}

// ---------------------------------------------------------------------------
// Background Check path (covers Check with Background:true)
// ---------------------------------------------------------------------------

func TestCheckBackground(t *testing.T) {
	sum := sha256.Sum256([]byte("asset"))
	release := Release{
		TagName: "v9.0.0",
		HTMLURL: "https://example.invalid/v9.0.0",
		Assets: []Asset{{
			Name:               "gm-v9.0.0-linux-amd64.tar.gz",
			BrowserDownloadURL: "https://github.com/GuoMonth/graphmind/releases/download/v9.0.0/gm-v9.0.0-linux-amd64.tar.gz",
			Digest:             "sha256:" + hex.EncodeToString(sum[:]),
		}},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(release)
	}))
	defer server.Close()

	manager := newTestManager(t)
	manager.apiBase = server.URL
	manager.currentVersion = "v1.0.0"

	// Background mode acquires lock, checks, returns empty result (no error)
	result, err := manager.Check(context.Background(), CheckOptions{Background: true})
	if err != nil {
		t.Fatalf("Check background: %v", err)
	}
	// Background returns empty result
	_ = result
}

func TestCheckBackgroundLockAlreadyHeld(t *testing.T) {
	manager := newTestManager(t)

	// Pre-create a fresh lock so tryAcquireLock returns ok=false
	if err := os.MkdirAll(filepath.Dir(manager.lockPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(manager.lockPath, []byte("held"), 0o644); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	// Use current time so the lock is NOT stale
	manager.now = func() time.Time { return time.Now() }

	// Should return immediately with empty result (lock busy)
	result, err := manager.Check(context.Background(), CheckOptions{Background: true})
	if err != nil {
		t.Fatalf("Check background with busy lock: %v", err)
	}
	_ = result
}

// ---------------------------------------------------------------------------
// parseAssetDigest edge cases
// ---------------------------------------------------------------------------

func TestParseAssetDigestEdgeCases(t *testing.T) {
	cases := []struct {
		input   string
		wantErr string
	}{
		{"", "missing asset digest"},
		{"   ", "missing asset digest"},
		{"md5:abcd", "unsupported asset digest format"},
		{"sha256:notvalidhex", "decode asset digest"},
		{"sha256:" + strings.Repeat("00", 16), "invalid asset digest length"}, // 16 bytes, want 32
	}

	for _, tc := range cases {
		_, err := parseAssetDigest(tc.input)
		if err == nil {
			t.Errorf("parseAssetDigest(%q): expected error, got nil", tc.input)
			continue
		}
		if !strings.Contains(err.Error(), tc.wantErr) {
			t.Errorf("parseAssetDigest(%q): error = %q, want to contain %q", tc.input, err.Error(), tc.wantErr)
		}
	}
}

// ---------------------------------------------------------------------------
// getRelease — non-200 status code
// ---------------------------------------------------------------------------

func TestGetReleaseNon200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	}))
	defer server.Close()

	manager := newTestManager(t)
	manager.apiBase = server.URL

	_, err := manager.fetchLatestRelease(context.Background())
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("err = %v, want 404 status error", err)
	}
}
