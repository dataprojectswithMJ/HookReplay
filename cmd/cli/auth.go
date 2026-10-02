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
	"strings"
	"time"

	"github.com/hookreplay/hookreplay/internal/auth"
	"github.com/hookreplay/hookreplay/pkg/apiclient"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Sign in via the web login page or switch to a named API key",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		if name != "" {
			c := loadConfig()
			raw, ok := c.Keys[name]
			if !ok {
				return fmt.Errorf("no key named %q — create it with:\n  hookreplay api-keys create --name %s", name, name)
			}
			c.APIKey = raw
			if err := saveConfig(c); err != nil {
				return err
			}
			fmt.Printf("now using key %q (%s…)\n", name, auth.DisplayPrefix(raw))
			fmt.Println("Tip: list keys with `hookreplay api-keys list`; switch with `hookreplay api-keys use <name>`")
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
				return fmt.Errorf("login timed out. Make sure the web app is running, or switch to a stored key with 'hookreplay login --name <name>'")
			case <-time.After(1 * time.Second):
				tok, err := client.OAuthResult(ctx, state)
				if err == nil && tok != "" {
					c := loadConfig()
					c.UserToken = tok
					if err := saveConfig(c); err != nil {
						return err
					}
					fmt.Println("\n✓ Logged in.")
					fmt.Println("Tip: create a key with `hookreplay api-keys create --name <name>`; switch with `hookreplay api-keys use <name>`")
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
	Short: "Create a named API key (shown once)",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			return fmt.Errorf("--name is required (API keys must have a name)")
		}
		client := apiclient.New(resolveAPIBase(), resolveUserToken())
		id, raw, err := client.CreateAPIKey(cmd.Context(), name, nil)
		if err != nil {
			return err
		}
		c := loadConfig()
		if c.Keys == nil {
			c.Keys = map[string]string{}
		}
		c.Keys[name] = raw
		if err := saveConfig(c); err != nil {
			return err
		}
		fmt.Printf("Created key %q (copy now — shown only once):\nid:  %s\nkey: %s\n", name, id, raw)
		fmt.Printf("Remembered it — switch to it any time with: hookreplay api-keys use %s\n", name)
		return nil
	},
}

var apiKeysListCmd = &cobra.Command{
	Use:   "list",
	Short: "List API keys",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := apiclient.New(resolveAPIBase(), resolveUserToken())
		keys, err := client.ListAPIKeys(cmd.Context())
		if err != nil {
			return err
		}
		if jsonOut {
			return json.NewEncoder(os.Stdout).Encode(keys)
		}
		activePrefix := ""
		if active := resolveAPIKey(); active != "" {
			activePrefix = auth.DisplayPrefix(active)
		}
		for _, k := range keys {
			name := k.Name
			if name == "" {
				name = "(unnamed)"
			}
			mark := "  "
			if activePrefix != "" && k.Prefix == activePrefix {
				mark = "* "
			}
			lastUsed := "never"
			if k.LastUsedAt != nil {
				lastUsed = k.LastUsedAt.Format(time.RFC3339)
			}
			fmt.Printf("%s%-20s  %s  scopes=%s  last_used=%s\n", mark, name, k.Prefix, strings.Join(k.Scopes, ","), lastUsed)
		}
		return nil
	},
}

var apiKeysRevokeCmd = &cobra.Command{
	Use:   "revoke <id>",
	Short: "Revoke an API key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := apiclient.New(resolveAPIBase(), resolveUserToken())
		if err := client.DeleteAPIKey(cmd.Context(), args[0]); err != nil {
			return err
		}
		fmt.Println("revoked", args[0])
		return nil
	},
}

var apiKeysUseCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Switch to a named API key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		c := loadConfig()
		raw, ok := c.Keys[name]
		if !ok {
			return fmt.Errorf("no local copy of key %q — create it with:\n  hookreplay api-keys create --name %s", name, name)
		}
		c.APIKey = raw
		if err := saveConfig(c); err != nil {
			return err
		}
		fmt.Printf("now using key %q (%s…)\n", name, auth.DisplayPrefix(raw))
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
	loginCmd.Flags().String("name", "", "switch to a named API key (e.g. dev); omit to open the web login page")
	apiKeysCreateCmd.Flags().String("name", "", "name for the key (required; lets you `hookreplay api-keys use <name>`)")
	apiKeysCmd.AddCommand(apiKeysCreateCmd, apiKeysListCmd, apiKeysUseCmd, apiKeysRevokeCmd)
	workspacesCmd.AddCommand(workspacesListCmd, workspacesCreateCmd)
}
