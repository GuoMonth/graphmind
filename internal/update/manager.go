package update

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/minio/selfupdate"
)

const (
	defaultOwner           = "GuoMonth"
	defaultRepo            = "graphmind"
	defaultCheckInterval   = 24 * time.Hour
	defaultLockTTL         = 15 * time.Minute
	backgroundCheckTimeout = 750 * time.Millisecond
	maxArchiveBytes        = 100 << 20
	defaultGitHubAPIBase   = "https://api.github.com/repos/GuoMonth/graphmind"
	defaultInstallCommand  = "gm update apply"
	backgroundCheckFlag    = "--background"
	backgroundCheckSubcmd  = "check"
	backgroundParentSubcmd = "update"
)

// Manager handles release discovery, update cache state, and binary replacement.
type Manager struct {
	currentVersion string
	client         *http.Client
	apiBase        string
	cachePath      string
	lockPath       string
	stderr         io.Writer
	now            func() time.Time
	goos           string
	goarch         string

	executablePath func() (string, error)
	startProcess   func(string, ...string) error
	applyBinary    func([]byte) error
}

// NewManager constructs an updater configured for the current build and platform.
func NewManager(currentVersion string) *Manager {
	cacheDir := defaultCacheDir()
	return &Manager{
		currentVersion: currentVersion,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		apiBase:        defaultGitHubAPIBase,
		cachePath:      filepath.Join(cacheDir, "update.json"),
		lockPath:       filepath.Join(cacheDir, "update.lock"),
		stderr:         os.Stderr,
		now:            time.Now,
		goos:           runtime.GOOS,
		goarch:         runtime.GOARCH,
		executablePath: os.Executable,
		startProcess:   startDetachedProcess,
		applyBinary:    defaultApplyBinary,
	}
}

// MaybeNotifyAndCheck prints a cached update hint to stderr and schedules a background
// refresh when the cached result is missing or older than the configured interval.
func (m *Manager) MaybeNotifyAndCheck() {
	state, err := m.loadState()
	if err == nil && state.UpdateAvailable && state.CheckedVersion == m.currentVersion && state.LatestVersion != "" {
		_, _ = fmt.Fprintf(
			m.stderr,
			"gm update available: %s (current %s). Run %q to install.\n",
			state.LatestVersion,
			m.currentVersion,
			defaultInstallCommand,
		)
	}
	if !shouldAutoCheck(&state, m.currentVersion, m.now()) {
		return
	}
	_ = m.startBackgroundCheck()
}

// Check queries GitHub Releases for the latest version and updates the local cache.
func (m *Manager) Check(ctx context.Context, options CheckOptions) (CheckResult, error) {
	if options.Background {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), backgroundCheckTimeout)
		defer cancel()

		unlock, ok, err := m.tryAcquireLock()
		if err != nil || !ok {
			return CheckResult{}, nil
		}
		defer unlock()
	}

	release, err := m.fetchLatestRelease(ctx)
	if err != nil {
		_ = m.saveState(&State{
			CheckedAt:      m.now().UTC(),
			CheckedVersion: m.currentVersion,
			LastError:      err.Error(),
		})
		if options.Background {
			return CheckResult{}, nil
		}
		return CheckResult{}, err
	}

	asset, err := m.findAsset(release)
	if err != nil {
		_ = m.saveState(&State{
			CheckedAt:      m.now().UTC(),
			CheckedVersion: m.currentVersion,
			LatestVersion:  release.TagName,
			ReleaseURL:     releaseURLFor(release),
			LastError:      err.Error(),
		})
		if options.Background {
			return CheckResult{}, nil
		}
		return CheckResult{}, err
	}

	status, available := updateStatus(m.currentVersion, release.TagName)
	result := CheckResult{
		Status:          status,
		CurrentVersion:  m.currentVersion,
		LatestVersion:   release.TagName,
		UpdateAvailable: available,
		AssetName:       asset.Name,
		ReleaseURL:      releaseURLFor(release),
		InstallCommand:  defaultInstallCommand,
		CheckedAt:       m.now().UTC(),
	}
	if err := m.saveState(&State{
		CheckedAt:       result.CheckedAt,
		CheckedVersion:  m.currentVersion,
		LatestVersion:   result.LatestVersion,
		UpdateAvailable: result.UpdateAvailable,
		AssetName:       result.AssetName,
		DownloadURL:     asset.BrowserDownloadURL,
		ReleaseURL:      result.ReleaseURL,
	}); err != nil && !options.Background {
		return CheckResult{}, err
	}
	return result, nil
}

// Apply downloads the matching release asset and replaces the current binary.
func (m *Manager) Apply(ctx context.Context, targetVersion string) (ApplyResult, error) {
	release, err := m.fetchRelease(ctx, targetVersion)
	if err != nil {
		return ApplyResult{}, err
	}
	status, available := updateStatus(m.currentVersion, release.TagName)
	if strings.TrimSpace(targetVersion) == "" && !available && status == "up_to_date" {
		executablePath, _ := m.executablePath()
		return ApplyResult{
			Status:           "up_to_date",
			PreviousVersion:  m.currentVersion,
			InstalledVersion: release.TagName,
			Updated:          false,
			TargetPath:       executablePath,
			ReleaseURL:       releaseURLFor(release),
		}, nil
	}

	asset, err := m.findAsset(release)
	if err != nil {
		return ApplyResult{}, err
	}
	archive, err := m.downloadAsset(ctx, asset)
	if err != nil {
		return ApplyResult{}, err
	}
	if err := verifyAssetDigest(archive, asset.Digest); err != nil {
		return ApplyResult{}, err
	}
	binary, err := extractBinary(archive, m.goos)
	if err != nil {
		return ApplyResult{}, err
	}
	if err := m.applyBinary(binary); err != nil {
		return ApplyResult{}, err
	}

	executablePath, _ := m.executablePath()
	result := ApplyResult{
		Status:           "updated",
		PreviousVersion:  m.currentVersion,
		InstalledVersion: release.TagName,
		Updated:          true,
		AssetName:        asset.Name,
		ReleaseURL:       releaseURLFor(release),
		TargetPath:       executablePath,
	}
	_ = m.saveState(&State{
		CheckedAt:       m.now().UTC(),
		CheckedVersion:  release.TagName,
		LatestVersion:   release.TagName,
		UpdateAvailable: false,
		AssetName:       asset.Name,
		DownloadURL:     asset.BrowserDownloadURL,
		ReleaseURL:      result.ReleaseURL,
	})
	return result, nil
}

func (m *Manager) fetchRelease(ctx context.Context, targetVersion string) (Release, error) {
	tag := canonicalVersionTag(targetVersion)
	if tag == "" {
		return m.fetchLatestRelease(ctx)
	}
	return m.fetchReleaseByTag(ctx, tag)
}

func (m *Manager) fetchLatestRelease(ctx context.Context) (Release, error) {
	return m.getRelease(ctx, m.apiBase+"/releases/latest")
}

func (m *Manager) fetchReleaseByTag(ctx context.Context, tag string) (Release, error) {
	return m.getRelease(ctx, m.apiBase+"/releases/tags/"+tag)
}

func (m *Manager) getRelease(ctx context.Context, endpoint string) (Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return Release{}, fmt.Errorf("create update request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "graphmind/"+m.currentVersion)

	resp, err := m.client.Do(req)
	if err != nil {
		return Release{}, fmt.Errorf("fetch release metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return Release{}, fmt.Errorf("fetch release metadata: unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return Release{}, fmt.Errorf("decode release metadata: %w", err)
	}
	if release.TagName == "" {
		return Release{}, fmt.Errorf("decode release metadata: missing tag_name")
	}
	return release, nil
}

func (m *Manager) findAsset(release Release) (Asset, error) {
	expected := archiveName(release.TagName, m.goos, m.goarch)
	for _, asset := range release.Assets {
		if asset.Name == expected {
			return asset, nil
		}
	}
	return Asset{}, fmt.Errorf("no release asset for %s/%s (expected %s)", m.goos, m.goarch, expected)
}

func (m *Manager) downloadAsset(ctx context.Context, asset Asset) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.BrowserDownloadURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create asset request: %w", err)
	}
	req.Header.Set("User-Agent", "graphmind/"+m.currentVersion)

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download update archive: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("download update archive: unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if resp.ContentLength > maxArchiveBytes {
		return nil, fmt.Errorf("download update archive: asset too large (%d bytes)", resp.ContentLength)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxArchiveBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read update archive: %w", err)
	}
	if int64(len(data)) > maxArchiveBytes {
		return nil, fmt.Errorf("download update archive: asset exceeded %d bytes", maxArchiveBytes)
	}
	return data, nil
}

func (m *Manager) loadState() (State, error) {
	data, err := os.ReadFile(m.cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return State{}, nil
		}
		return State{}, fmt.Errorf("read update cache: %w", err)
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, fmt.Errorf("decode update cache: %w", err)
	}
	return state, nil
}

func (m *Manager) saveState(state *State) error {
	if err := os.MkdirAll(filepath.Dir(m.cachePath), 0o755); err != nil {
		return fmt.Errorf("create update cache directory: %w", err)
	}
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode update cache: %w", err)
	}
	tmpPath := m.cachePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("write update cache: %w", err)
	}
	if err := os.Rename(tmpPath, m.cachePath); err != nil {
		return fmt.Errorf("replace update cache: %w", err)
	}
	return nil
}

func (m *Manager) tryAcquireLock() (unlock func(), ok bool, err error) {
	if err := os.MkdirAll(filepath.Dir(m.lockPath), 0o755); err != nil {
		return nil, false, fmt.Errorf("create update lock directory: %w", err)
	}

	for attempt := 0; attempt < 2; attempt++ {
		file, err := os.OpenFile(m.lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			_, _ = file.WriteString(m.now().UTC().Format(time.RFC3339))
			_ = file.Close()
			return func() {
				_ = os.Remove(m.lockPath)
			}, true, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, false, fmt.Errorf("create update lock: %w", err)
		}
		info, statErr := os.Stat(m.lockPath)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				continue
			}
			return nil, false, fmt.Errorf("stat update lock: %w", statErr)
		}
		if m.now().Sub(info.ModTime()) <= defaultLockTTL {
			return nil, false, nil
		}
		if removeErr := os.Remove(m.lockPath); removeErr != nil && !os.IsNotExist(removeErr) {
			return nil, false, fmt.Errorf("remove stale update lock: %w", removeErr)
		}
	}
	return nil, false, nil
}

func (m *Manager) startBackgroundCheck() error {
	executablePath, err := m.executablePath()
	if err != nil {
		return err
	}
	return m.startProcess(executablePath, backgroundParentSubcmd, backgroundCheckSubcmd, backgroundCheckFlag)
}

func shouldAutoCheck(state *State, currentVersion string, now time.Time) bool {
	if state == nil {
		return true
	}
	if state.CheckedAt.IsZero() {
		return true
	}
	if state.CheckedVersion != currentVersion {
		return true
	}
	return now.Sub(state.CheckedAt) >= defaultCheckInterval
}

func verifyAssetDigest(payload []byte, digest string) error {
	digest = strings.TrimSpace(digest)
	if digest == "" {
		return fmt.Errorf("missing asset digest")
	}
	const prefix = "sha256:"
	if !strings.HasPrefix(digest, prefix) {
		return fmt.Errorf("unsupported asset digest format %q", digest)
	}
	expected, err := hex.DecodeString(strings.TrimPrefix(digest, prefix))
	if err != nil {
		return fmt.Errorf("decode asset digest: %w", err)
	}
	sum := sha256.Sum256(payload)
	if !bytes.Equal(sum[:], expected) {
		return fmt.Errorf("update archive checksum mismatch")
	}
	return nil
}

func releaseURLFor(release Release) string {
	if release.HTMLURL != "" {
		return release.HTMLURL
	}
	return fmt.Sprintf("https://github.com/%s/%s/releases/tag/%s", defaultOwner, defaultRepo, release.TagName)
}

func defaultCacheDir() string {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "graphmind")
	}
	return filepath.Join(cacheDir, "graphmind")
}

func defaultApplyBinary(binary []byte) error {
	err := selfupdate.Apply(bytes.NewReader(binary), selfupdate.Options{})
	if err == nil {
		return nil
	}
	if rollbackErr := selfupdate.RollbackError(err); rollbackErr != nil {
		return fmt.Errorf("apply update: %w (rollback failed: %v)", err, rollbackErr)
	}
	return fmt.Errorf("apply update: %w", err)
}
