package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// updateRepo is the GitHub repo releases are published from.
const updateRepo = "dataprojectswithMJ/HookReplay"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the CLI version",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("hookreplay version %s\n", version)
		return nil
	},
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update the CLI to the latest release",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return selfUpdate(cmd)
	},
}

func selfUpdate(cmd *cobra.Command) error {
	latest, err := latestVersion(cmd)
	if err != nil {
		return err
	}
	if !isNewer(latest, version) {
		fmt.Printf("already up to date (v%s)\n", version)
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("update: locate current binary: %w", err)
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return fmt.Errorf("update: resolve current binary: %w", err)
	}

	fmt.Printf("updating %s → %s…\n", version, latest)
	asset := fmt.Sprintf("hookreplay_%s_%s_%s.%s", latest, runtime.GOOS, runtime.GOARCH, archiveExt())
	body, err := downloadAsset(latest, asset)
	if err != nil {
		return err
	}
	binary, err := extractBinary(asset, body)
	if err != nil {
		return err
	}

	dir := filepath.Dir(exe)
	tmp, err := os.CreateTemp(dir, ".hookreplay-update-*")
	if err != nil {
		return fmt.Errorf("update: create temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(binary); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("update: write new binary: %w", err)
	}
	if err := tmp.Chmod(0o755); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("update: chmod new binary: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("update: close new binary: %w", err)
	}

	if err := os.Rename(tmpName, exe); err != nil {
		return fmt.Errorf("update: replace %s: %w — if it's a system install, run `sudo hookreplay update` or re-run the installer", exe, err)
	}

	fmt.Printf("updated to v%s\n", latest)
	return nil
}

func latestVersion(cmd *cobra.Command) (string, error) {
	req, err := http.NewRequestWithContext(cmd.Context(), http.MethodGet,
		"https://api.github.com/repos/"+updateRepo+"/releases/latest", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return "", fmt.Errorf("update: check latest release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("update: check latest release: HTTP %d", resp.StatusCode)
	}
	var rel struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&rel); err != nil {
		return "", fmt.Errorf("update: decode release: %w", err)
	}
	return strings.TrimPrefix(rel.TagName, "v"), nil
}

func downloadAsset(version, asset string) ([]byte, error) {
	url := fmt.Sprintf("https://github.com/%s/releases/download/v%s/%s", updateRepo, version, asset)
	resp, err := (&http.Client{Timeout: 5 * time.Minute}).Get(url)
	if err != nil {
		return nil, fmt.Errorf("update: download %s: %w", asset, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update: download %s: HTTP %d", asset, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, fmt.Errorf("update: read %s: %w", asset, err)
	}
	if err := verifyChecksum(version, asset, body); err != nil {
		return nil, err
	}
	return body, nil
}

// verifyChecksum checks the asset's SHA-256 against checksums.txt. It fails
// closed only when a matching checksum line exists but mismatches; a missing
// checksums.txt is tolerated (best-effort).
func verifyChecksum(version, asset string, body []byte) error {
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Get(
		fmt.Sprintf("https://github.com/%s/releases/download/v%s/checksums.txt", updateRepo, version))
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[1] != asset {
			continue
		}
		sum := sha256.Sum256(body)
		if fields[0] != hex.EncodeToString(sum[:]) {
			return fmt.Errorf("update: checksum mismatch for %s", asset)
		}
		return nil
	}
	return nil
}

func archiveExt() string {
	if runtime.GOOS == "windows" {
		return "zip"
	}
	return "tar.gz"
}

func extractBinary(asset string, body []byte) ([]byte, error) {
	if strings.HasSuffix(asset, ".zip") {
		zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
		if err != nil {
			return nil, fmt.Errorf("update: unzip: %w", err)
		}
		for _, f := range zr.File {
			if base := filepath.Base(f.Name); base == "hookreplay" || base == "hookreplay.exe" {
				rc, err := f.Open()
				if err != nil {
					return nil, err
				}
				defer rc.Close()
				return io.ReadAll(io.LimitReader(rc, 64<<20))
			}
		}
		return nil, fmt.Errorf("update: hookreplay binary not found in %s", asset)
	}

	gz, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("update: gunzip: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("update: tar: %w", err)
		}
		if hdr.Typeflag == tar.TypeReg && filepath.Base(hdr.Name) == "hookreplay" {
			return io.ReadAll(io.LimitReader(tr, 64<<20))
		}
	}
	return nil, fmt.Errorf("update: hookreplay binary not found in %s", asset)
}

// isNewer reports whether candidate is a higher semver than current. Non-semver
// versions fall back to a simple inequality.
func isNewer(candidate, current string) bool {
	cm, cn, cp, cok := parseSemver(candidate)
	km, kn, kp, kok := parseSemver(current)
	if !cok || !kok {
		return candidate != current
	}
	if cm != km {
		return cm > km
	}
	if cn != kn {
		return cn > kn
	}
	return cp > kp
}

func parseSemver(s string) (int, int, int, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	parts := strings.SplitN(s, ".", 3)
	if len(parts) != 3 {
		return 0, 0, 0, false
	}
	var n [3]int
	for i, p := range parts {
		v, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return 0, 0, 0, false
		}
		n[i] = v
	}
	return n[0], n[1], n[2], true
}
