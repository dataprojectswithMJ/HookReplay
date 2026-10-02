package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/hookreplay/hookreplay/internal/tunnel"
	"github.com/spf13/cobra"
)

var tunnelCmd = &cobra.Command{
	Use:   "tunnel",
	Short: "Open a WebSocket tunnel to localhost and print the public URL",
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetInt("port")
		subdomain, _ := cmd.Flags().GetString("subdomain")
		if subdomain == "" {
			subdomain = randomSubdomain()
		}

		key := resolveAPIKey()
		if key == "" {
			return fmt.Errorf("auth error: no API key (run 'hookreplay login' or set HOOKREPLAY_API_KEY)")
		}

		c := &tunnel.Client{
			APIToken:  key,
			Subdomain: subdomain,
			LocalPort: port,
			WSBaseURL: wsBaseFrom(resolveAPIBase()),
		}
		publicURL := c.PublicURL()
		fmt.Printf("Opening tunnel %s\n", publicURL)
		c.OnConnected = func() {
			fmt.Printf("Tunnel ready: %s\n", publicURL)
		}
		c.OnConnectError = func(err error) {
			fmt.Fprintf(os.Stderr, "tunnel: connection failed: %v\n", err)
			fmt.Fprintf(os.Stderr, "tunnel: is the API running and is your key valid? (try: hookreplay login)\n")
		}
		return c.Run(cmd.Context())
	},
}

func init() {
	tunnelCmd.Flags().Int("port", 3000, "local port to forward to")
	tunnelCmd.Flags().String("subdomain", "", "request a specific subdomain")
}

func wsBaseFrom(apiBase string) string {
	base := strings.TrimRight(apiBase, "/")
	if strings.HasPrefix(base, "https://") {
		return "wss://" + strings.TrimPrefix(base, "https://")
	}
	return "ws://" + strings.TrimPrefix(base, "http://")
}

func randomSubdomain() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "hrk-dev"
	}
	return "hrk-" + hex.EncodeToString(b)
}
