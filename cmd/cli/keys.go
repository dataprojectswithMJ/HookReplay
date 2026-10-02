package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"

	"github.com/spf13/cobra"
)

var keysCmd = &cobra.Command{Use: "keys", Short: "Generate signing keys for custom schemes"}

var keysGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate an Ed25519 keypair (Resend / custom ed25519)",
	RunE: func(cmd *cobra.Command, args []string) error {
		pub, priv, err := ed25519.GenerateKey(nil)
		if err != nil {
			return err
		}
		fmt.Printf("private key (base64): %s\n", base64.StdEncoding.EncodeToString(priv))
		fmt.Printf("public key  (base64): %s\n", base64.StdEncoding.EncodeToString(pub))
		return nil
	},
}

func init() {
	keysCmd.AddCommand(keysGenerateCmd)
}
