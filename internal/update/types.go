package update

import "time"

// Release is the subset of GitHub release metadata used by gm updates.
type Release struct {
	TagName string  `json:"tag_name"`
	HTMLURL string  `json:"html_url"`
	Assets  []Asset `json:"assets"`
}

// Asset is one downloadable archive attached to a GitHub release.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Digest             string `json:"digest,omitempty"`
}

// State caches the last update check result for non-blocking CLI startup checks.
type State struct {
	CheckedAt       time.Time `json:"checked_at"`
	CheckedVersion  string    `json:"checked_version"`
	LatestVersion   string    `json:"latest_version,omitempty"`
	UpdateAvailable bool      `json:"update_available"`
	AssetName       string    `json:"asset_name,omitempty"`
	DownloadURL     string    `json:"download_url,omitempty"`
	ReleaseURL      string    `json:"release_url,omitempty"`
	LastError       string    `json:"last_error,omitempty"`
}

// CheckOptions controls how an update check runs.
type CheckOptions struct {
	Background bool
}

// CheckResult is the structured stdout payload for gm update check.
type CheckResult struct {
	Status          string    `json:"status"`
	CurrentVersion  string    `json:"current_version"`
	LatestVersion   string    `json:"latest_version,omitempty"`
	UpdateAvailable bool      `json:"update_available"`
	AssetName       string    `json:"asset_name,omitempty"`
	ReleaseURL      string    `json:"release_url,omitempty"`
	InstallCommand  string    `json:"install_command,omitempty"`
	CheckedAt       time.Time `json:"checked_at"`
}

// ApplyResult is the structured stdout payload for gm update apply.
type ApplyResult struct {
	Status           string `json:"status"`
	PreviousVersion  string `json:"previous_version"`
	InstalledVersion string `json:"installed_version"`
	Updated          bool   `json:"updated"`
	AssetName        string `json:"asset_name,omitempty"`
	ReleaseURL       string `json:"release_url,omitempty"`
	TargetPath       string `json:"target_path,omitempty"`
}
