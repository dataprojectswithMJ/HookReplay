package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	apiKey  string
	apiBase string
	jsonOut bool
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

// version is overridable at build time via -ldflags "-X main.version=vX.Y.Z".
var version = "0.1.0"

var rootCmd = &cobra.Command{
	Use:           "hookreplay",
	Short:         "Fire provider-accurate, correctly-signed webhooks at localhost",
	Version:       version,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&apiKey, "api-key", "", "API key (default: $HOOKREPLAY_API_KEY or ~/.hookreplay/config)")
	rootCmd.PersistentFlags().StringVar(&apiBase, "api-base", "", "API base URL (default: $HOOKREPLAY_API_BASE or http://localhost:8080)")
	rootCmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "machine-readable output")

	rootCmd.AddCommand(loginCmd, initCmd, fmtCmd, validateCmd, tunnelCmd, runCmd, templatesCmd, secretsCmd, apiKeysCmd, workspacesCmd, keysCmd, chainsCmd, versionCmd, updateCmd)
}

// --- config ---

type config struct {
	APIKey  string            `json:"api_key"`
	APIBase string            `json:"api_base"`
	Keys    map[string]string `json:"keys,omitempty"`
}

func configPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".hookreplay", "config")
}

func loadConfig() config {
	c := config{Keys: map[string]string{"dev": devAPIKey}}
	data, err := os.ReadFile(configPath())
	if err != nil {
		return c
	}
	_ = json.Unmarshal(data, &c)
	if c.Keys == nil {
		c.Keys = map[string]string{}
	}
	if _, ok := c.Keys["dev"]; !ok {
		c.Keys["dev"] = devAPIKey
	}
	return c
}

func saveConfig(c config) error {
	p := configPath()
	if p == "" {
		return fmt.Errorf("cannot determine home directory")
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(c, "", "  ")
	return os.WriteFile(p, data, 0o600)
}

func resolveAPIKey() string {
	if apiKey != "" {
		return apiKey
	}
	if v := os.Getenv("HOOKREPLAY_API_KEY"); v != "" {
		return v
	}
	if c := loadConfig(); c.APIKey != "" {
		return c.APIKey
	}
	// Dev fallback: the API seeds this key by default (HOOKREPLAY_DEV_API_KEY).
	fmt.Fprintln(os.Stderr, "warning: using default dev API key (hrk_dev_local_dev_only_key); set HOOKREPLAY_API_KEY or run 'hookreplay login' to override")
	return devAPIKey
}

const devAPIKey = "hrk_dev_local_dev_only_key"

func resolveAPIBase() string {
	if apiBase != "" {
		return apiBase
	}
	if v := os.Getenv("HOOKREPLAY_API_BASE"); v != "" {
		return v
	}
	if c := loadConfig(); c.APIBase != "" {
		return c.APIBase
	}
	return "http://localhost:8080"
}
