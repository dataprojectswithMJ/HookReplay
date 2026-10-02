package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/hookreplay/hookreplay/internal/chains"
	"github.com/hookreplay/hookreplay/pkg/apiclient"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a hookreplay.yml chain through the API",
	RunE: func(cmd *cobra.Command, args []string) error {
		file, _ := cmd.Flags().GetString("file")
		if file == "" {
			file = "hookreplay.yml"
		}
		targetFlag, _ := cmd.Flags().GetString("target")
		envFlag, _ := cmd.Flags().GetString("env")
		failOn, _ := cmd.Flags().GetBool("fail-on")

		data, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("config error: %w", err)
		}
		data = []byte(interpolate(string(data)))

		chain, err := chains.Parse(data)
		if err != nil {
			return fmt.Errorf("config error: %w", err)
		}
		if errs := chain.Validate(); len(errs) > 0 {
			for _, e := range errs {
				fmt.Fprintln(os.Stderr, e)
			}
			return fmt.Errorf("config error: %d validation error(s)", len(errs))
		}

		target := targetFlag
		if target == "" {
			target = chain.Target
		}
		if target == "" {
			return fmt.Errorf("config error: no target (set chain.target, --target, or ${TUNNEL_URL})")
		}
		env := envFlag
		if env == "" {
			env = chain.Environment
		}
		if env == "" {
			env = "local"
		}

		key := resolveAPIKey()
		if key == "" {
			return fmt.Errorf("auth error: no API key (run 'hookreplay login' or set HOOKREPLAY_API_KEY)")
		}

		runner := &chains.Runner{
			API:    apiclient.New(resolveAPIBase(), key),
			Target: target,
			Env:    env,
		}
		result, err := runner.Run(cmd.Context(), chain)
		if err != nil {
			return fmt.Errorf("run error: %w", err)
		}

		if jsonOut {
			_ = json.NewEncoder(os.Stdout).Encode(result)
		} else {
			printRunResult(result)
		}

		if !result.Passed {
			if failOn {
				return fmt.Errorf("one or more steps failed expectations")
			}
			return fmt.Errorf("one or more steps failed")
		}
		return nil
	},
}

func init() {
	runCmd.Flags().String("file", "hookreplay.yml", "chain file to run")
	runCmd.Flags().String("target", "", "override target URL")
	runCmd.Flags().String("env", "", "environment (local|staging|production)")
	runCmd.Flags().Bool("fail-on", false, "fail on unmet expect (exit 1)")
}

func interpolate(s string) string {
	if !strings.Contains(s, "${") {
		return s
	}
	return os.Expand(s, os.Getenv)
}

func printRunResult(r *chains.RunResult) {
	for _, s := range r.Steps {
		retryNote := ""
		if s.Retries > 0 {
			retryNote = fmt.Sprintf("  [%d retries: %v]", s.Retries, s.Delays)
		}
		switch {
		case s.Skipped:
			fmt.Printf("— %s  (skipped)\n", s.Name)
		case s.Failed:
			fmt.Printf("✗ %s  FAILED", s.Name)
			if len(s.ExpectErrors) > 0 {
				fmt.Printf("  %s", strings.Join(s.ExpectErrors, "; "))
			} else if s.Error != "" {
				fmt.Printf("  %s", s.Error)
			}
			fmt.Printf("%s\n", retryNote)
		default:
			fmt.Printf("✓ %s  HTTP %d  %dms%s\n", s.Name, s.Status, s.LatencyMS, retryNote)
		}
	}
	status := "PASS"
	if !r.Passed {
		status = "FAIL"
	}
	fmt.Printf("chain %q: %s\n", r.Name, status)
}
