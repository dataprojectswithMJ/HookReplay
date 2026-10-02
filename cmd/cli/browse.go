package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/hookreplay/hookreplay/pkg/apiclient"
	"github.com/hookreplay/hookreplay/templates"
	"github.com/spf13/cobra"
)

var templatesCmd = &cobra.Command{Use: "templates", Short: "Browse the provider template library"}

var templatesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all templates",
	RunE: func(cmd *cobra.Command, args []string) error {
		list, err := templates.List()
		if err != nil {
			return err
		}
		if jsonOut {
			return json.NewEncoder(os.Stdout).Encode(list)
		}
		for _, t := range list {
			fmt.Printf("%s/%s\n", t.Provider, t.Event)
		}
		return nil
	},
}

var templatesShowCmd = &cobra.Command{
	Use:   "show provider/event",
	Short: "Show a template's payload and signing details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		provider, event, ok := strings.Cut(args[0], "/")
		if !ok {
			return fmt.Errorf("expected provider/event")
		}
		t, err := templates.Get(provider, event)
		if err != nil {
			return err
		}
		fmt.Printf("provider: %s\n", t.Provider)
		fmt.Printf("event:    %s\n", t.Event)
		fmt.Printf("scheme:   %s\n", t.Sign.Scheme)
		fmt.Printf("header:   %s\n", t.Sign.SignatureHeader)
		fmt.Printf("note:     %s\n", t.Sign.Note)
		fmt.Printf("payload:\n%s\n", string(t.Payload))
		return nil
	},
}

var secretsCmd = &cobra.Command{Use: "secrets", Short: "Write-only workspace secrets"}

var secretsSetCmd = &cobra.Command{
	Use:   "set NAME",
	Short: "Store a secret (write-only; never read back)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		env, _ := cmd.Flags().GetString("env")
		value, _ := cmd.Flags().GetString("value")
		if value == "" {
			fmt.Print("Secret value: ")
			reader := bufio.NewReader(os.Stdin)
			line, _ := reader.ReadString('\n')
			value = strings.TrimRight(line, "\r\n")
		}
		if value == "" {
			return fmt.Errorf("a value is required")
		}
		key := resolveAPIKey()
		if key == "" {
			return fmt.Errorf("auth error: no API key")
		}
		client := apiclient.New(resolveAPIBase(), key)
		if err := client.SetSecret(cmd.Context(), env, args[0], value); err != nil {
			return err
		}
		fmt.Printf("stored %s (env: %s)\n", args[0], env)
		return nil
	},
}

var secretsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List secret names (never values)",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := apiclient.New(resolveAPIBase(), resolveAPIKey())
		list, err := client.ListSecrets(cmd.Context())
		if err != nil {
			return err
		}
		if jsonOut {
			return json.NewEncoder(os.Stdout).Encode(list)
		}
		for _, s := range list {
			fmt.Printf("%s  %s  %s\n", s.Env, s.Name, s.UpdatedAt.Format(time.RFC3339))
		}
		return nil
	},
}

var secretsDeleteCmd = &cobra.Command{
	Use:   "delete NAME",
	Short: "Delete a secret",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		env, _ := cmd.Flags().GetString("env")
		client := apiclient.New(resolveAPIBase(), resolveAPIKey())
		if err := client.DeleteSecret(cmd.Context(), env, args[0]); err != nil {
			return err
		}
		fmt.Printf("deleted %s (env: %s)\n", args[0], env)
		return nil
	},
}

func init() {
	templatesCmd.AddCommand(templatesListCmd, templatesShowCmd)
	secretsCmd.AddCommand(secretsSetCmd, secretsListCmd, secretsDeleteCmd)
	secretsSetCmd.Flags().String("env", "local", "environment scope")
	secretsSetCmd.Flags().String("value", "", "secret value (prompts if omitted)")
	secretsDeleteCmd.Flags().String("env", "local", "environment scope")
}
