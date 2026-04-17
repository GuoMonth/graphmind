package cli

import (
	"fmt"

	"github.com/senguoyun-guosheng/graphmind/internal/update"
	"github.com/spf13/cobra"
)

var (
	updateApplyVersion    string
	updateCheckBackground bool
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Check for or apply gm updates",
	Long: `Manage GraphMind CLI updates published through GitHub Releases.

Use "gm update check" to perform a blocking version check now.
Use "gm update apply" to download and install a newer release.

Regular gm commands may trigger a non-blocking background update check at most
once every 24 hours. If a newer release is already known, gm prints a short
notice to stderr without changing stdout JSON output.`,
}

var updateCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check for a newer release",
	Long: `Check GitHub Releases for a newer GraphMind CLI build.

This command performs a blocking network request and refreshes the local update
cache immediately. Normal gm commands use a separate non-blocking background
check path instead.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		result, err := svc.updater.Check(cmd.Context(), update.CheckOptions{
			Background: updateCheckBackground,
		})
		if err != nil {
			return err
		}
		if updateCheckBackground {
			return nil
		}

		summary := fmt.Sprintf("Checked for updates. Current: %s. Latest: %s.", result.CurrentVersion, result.LatestVersion)
		next := []string{"gm update apply  — download and install the latest release"}
		if !result.UpdateAvailable {
			summary = fmt.Sprintf("Already up to date at %s.", result.CurrentVersion)
			next = []string{"gm --version  — confirm the active version"}
		}
		outputSuccess(result, summary, next)
		return nil
	},
}

var updateApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Download and install a release",
	Long: `Download the matching GitHub Release archive for the current platform and
replace the current gm executable.

By default this installs the latest release. Use --version to pin a specific
tag such as v0.3.1.

gm verifies the GitHub-provided sha256 digest for the selected release asset
before replacing the current binary.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		result, err := svc.updater.Apply(cmd.Context(), updateApplyVersion)
		if err != nil {
			return err
		}

		summary := fmt.Sprintf("Installed %s.", result.InstalledVersion)
		next := []string{"gm --version  — confirm the installed version"}
		if !result.Updated {
			summary = fmt.Sprintf("Already up to date at %s.", result.InstalledVersion)
		}
		outputSuccess(result, summary, next)
		return nil
	},
}

func init() {
	updateCmd.AddCommand(updateCheckCmd)
	updateCmd.AddCommand(updateApplyCmd)

	updateCheckCmd.Flags().BoolVar(&updateCheckBackground, "background", false, "Run as a silent background refresh")
	_ = updateCheckCmd.Flags().MarkHidden("background")

	updateApplyCmd.Flags().StringVar(&updateApplyVersion, "version", "", "Install a specific release tag (for example v0.3.1)")
}
