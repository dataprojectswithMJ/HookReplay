package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/hookreplay/hookreplay/pkg/apiclient"
	"github.com/spf13/cobra"
)

var chainsCmd = &cobra.Command{Use: "chains", Short: "Sync chains with your workspace"}

var chainsPushCmd = &cobra.Command{
	Use:   "push [file]",
	Short: "Push a hookreplay.yml to your workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		file := "hookreplay.yml"
		if len(args) > 0 {
			file = args[0]
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		client := apiclient.New(resolveAPIBase(), resolveAPIKey())
		c, err := client.CreateChain(cmd.Context(), "", string(data), "repo")
		if err != nil {
			return err
		}
		fmt.Printf("pushed chain %s (id: %s, v%d)\n", c.Name, c.ID, c.Version)
		return nil
	},
}

var chainsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List workspace chains",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := apiclient.New(resolveAPIBase(), resolveAPIKey())
		list, err := client.ListChains(cmd.Context())
		if err != nil {
			return err
		}
		if jsonOut {
			return json.NewEncoder(os.Stdout).Encode(list)
		}
		for _, c := range list {
			fmt.Printf("%s  %s  v%d  %s\n", c.ID, c.Name, c.Version, c.Source)
		}
		return nil
	},
}

var chainsPullCmd = &cobra.Command{
	Use:   "pull <id|name>",
	Short: "Pull a workspace chain to hookreplay.yml",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := apiclient.New(resolveAPIBase(), resolveAPIKey())
		list, err := client.ListChains(cmd.Context())
		if err != nil {
			return err
		}
		for _, c := range list {
			if c.ID == args[0] || c.Name == args[0] {
				if err := os.WriteFile("hookreplay.yml", []byte(c.YAMLText), 0o644); err != nil {
					return err
				}
				fmt.Printf("pulled %s -> hookreplay.yml\n", c.Name)
				return nil
			}
		}
		return fmt.Errorf("chain %q not found", args[0])
	},
}

func init() {
	chainsCmd.AddCommand(chainsPushCmd, chainsListCmd, chainsPullCmd)
}
