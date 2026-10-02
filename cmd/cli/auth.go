package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/hookreplay/hookreplay/pkg/apiclient"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Sign in via GitHub OAuth and store an API key",
	RunE: func(cmd *cobra.Command, args []string) error {
		key, _ := cmd.Flags().GetString("key")
		if key != "" {
			c := loadConfig()
			c.APIKey = key
			if err := saveConfig(c); err != nil {
				return err
			}
			fmt.Println("API key stored")
			return nil
		}

		state := "cli_" + randomState()
		client := apiclient.New(resolveAPIBase(), "")
		startURL := client.AuthStartURL(state)

		fmt.Println("Opening browser to sign in…")
		fmt.Printf("If it doesn't open, visit:\n  %s\n", startURL)
		openBrowser(startURL)

		fmt.Print("Waiting for login")
		ctx, cancel := context.WithTimeout(cmd.Context(), 120*time.Second)
		defer cancel()
		for {
			select {
			case <-ctx.Done():
				fmt.Println()
				return fmt.Errorf("login timed out. Make sure the web app is running, or use 'hookreplay login --key <key>'")
			case <-time.After(1 * time.Second):
				k, err := client.OAuthResult(ctx, state)
				if err == nil && k != "" {
					c := loadConfig()
					c.APIKey = k
					if err := saveConfig(c); err != nil {
						return err
					}
					fmt.Println("\n✓ Logged in — API key stored.")
					return nil
				}
				fmt.Print(".")
			}
		}
	},
}

var apiKeysCmd = &cobra.Command{
	Use:     "api-keys",
	Aliases: []string{"apikeys"},
	Short:   "Manage API keys",
}

var apiKeysCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an API key (shown once)",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := apiclient.New(resolveAPIBase(), resolveAPIKey())
		id, raw, err := client.CreateAPIKey(cmd.Context(), nil)
		if err != nil {
			return err
		}
		fmt.Printf("Created API key (copy now — shown only once):\nid:  %s\nkey: %s\n", id, raw)
		return nil
	},
}

var apiKeysListCmd = &cobra.Command{
	Use:   "list",
	Short: "List API keys",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := apiclient.New(resolveAPIBase(), resolveAPIKey())
		keys, err := client.ListAPIKeys(cmd.Context())
		if err != nil {
			return err
		}
		if jsonOut {
			return json.NewEncoder(os.Stdout).Encode(keys)
		}
		for _, k := range keys {
			lastUsed := "never"
			if k.LastUsedAt != nil {
				lastUsed = k.LastUsedAt.Format(time.RFC3339)
			}
			fmt.Printf("%s  %s  scopes=%v  last_used=%s\n", k.Prefix, k.ID, k.Scopes, lastUsed)
		}
		return nil
	},
}

var apiKeysRevokeCmd = &cobra.Command{
	Use:   "revoke <id>",
	Short: "Revoke an API key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := apiclient.New(resolveAPIBase(), resolveAPIKey())
		if err := client.DeleteAPIKey(cmd.Context(), args[0]); err != nil {
			return err
		}
		fmt.Println("revoked", args[0])
		return nil
	},
}

var workspacesCmd = &cobra.Command{Use: "workspaces", Short: "Manage workspaces"}

var workspacesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List workspaces",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := apiclient.New(resolveAPIBase(), resolveAPIKey())
		list, err := client.ListWorkspaces(cmd.Context())
		if err != nil {
			return err
		}
		if jsonOut {
			return json.NewEncoder(os.Stdout).Encode(list)
		}
		for _, ws := range list {
			fmt.Printf("%s  %s  tier=%s  role=%s\n", ws.ID, ws.Name, ws.Tier, ws.Role)
		}
		return nil
	},
}

var workspacesCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a workspace",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := apiclient.New(resolveAPIBase(), resolveAPIKey())
		id, err := client.CreateWorkspace(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		fmt.Printf("created workspace %q (id: %s)\n", args[0], id)
		return nil
	},
}

func randomState() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func openBrowser(url string) {
	var c *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		c = exec.Command("open", url)
	case "windows":
		c = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		c = exec.Command("xdg-open", url)
	}
	_ = c.Start()
}

func init() {
	loginCmd.Flags().String("key", "", "store a key directly instead of using OAuth")
	apiKeysCmd.AddCommand(apiKeysCreateCmd, apiKeysListCmd, apiKeysRevokeCmd)
	workspacesCmd.AddCommand(workspacesListCmd, workspacesCreateCmd)
}
