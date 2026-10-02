package main

import (
	"bytes"
	"fmt"
	"os"

	"github.com/hookreplay/hookreplay/internal/chains"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold a hookreplay.yml",
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := os.Stat("hookreplay.yml"); err == nil {
			return fmt.Errorf("hookreplay.yml already exists")
		}
		tmpl := `name: my-first-webhook
target: ${TUNNEL_URL}
environment: local
steps:
  - name: checkout-completed
    webhook: stripe/checkout.session.completed
    secret: env:STRIPE_WEBHOOK_SECRET
`
		return os.WriteFile("hookreplay.yml", []byte(tmpl), 0o644)
	},
}

var fmtCmd = &cobra.Command{
	Use:   "fmt [file]",
	Short: "Normalize a chain to block-style YAML",
	RunE: func(cmd *cobra.Command, args []string) error {
		file := "hookreplay.yml"
		if len(args) > 0 {
			file = args[0]
		}
		check, _ := cmd.Flags().GetBool("check")

		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		var node yaml.Node
		if err := yaml.Unmarshal(data, &node); err != nil {
			return fmt.Errorf("parse %s: %w", file, err)
		}
		var buf bytes.Buffer
		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2) // conventional 2-space block style
		if err := enc.Encode(&node); err != nil {
			return err
		}
		if err := enc.Close(); err != nil {
			return err
		}
		out := buf.Bytes()
		if check {
			if string(out) != string(data) {
				fmt.Println("diff detected")
				return fmt.Errorf("not formatted")
			}
			return nil
		}
		return os.WriteFile(file, out, 0o644)
	},
}

var validateCmd = &cobra.Command{
	Use:   "validate [file]",
	Short: "Lint a chain against the schema",
	RunE: func(cmd *cobra.Command, args []string) error {
		file := "hookreplay.yml"
		if len(args) > 0 {
			file = args[0]
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		chain, err := chains.Parse(data)
		if err != nil {
			return fmt.Errorf("parse %s: %w", file, err)
		}
		errs := chain.Validate()
		if len(errs) > 0 {
			for _, e := range errs {
				fmt.Println(e)
			}
			return fmt.Errorf("%d validation error(s)", len(errs))
		}
		fmt.Println("valid")
		return nil
	},
}

func init() {
	fmtCmd.Flags().Bool("check", false, "exit 1 if a diff would be written")
}
