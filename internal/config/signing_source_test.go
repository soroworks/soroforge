package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// SigningSource names where the signing key comes from without reading it,
// for commands like `soroforge whoami` that report provenance only.
func TestSigningSource(t *testing.T) {
	const seed = "SBTESTSECRETSEEDTHATMUSTNEVERAPPEARINANYOUTPUTXXXXXXXXXXXX"

	tests := []struct {
		name string
		env  Env
		want string
	}{
		{
			name: "secret key from the environment",
			env:  Env{SecretKey: seed},
			want: "env:" + EnvSecretKey,
		},
		{
			name: "keystore file",
			env:  Env{KeystorePath: "/run/secrets/deployer.json"},
			want: "keystore:/run/secrets/deployer.json",
		},
		{
			// SigningSeed uses the environment key first, so the label must too.
			name: "environment key wins over a keystore",
			env:  Env{SecretKey: seed, KeystorePath: "/run/secrets/deployer.json"},
			want: "env:" + EnvSecretKey,
		},
		{
			name: "no key source configured",
			env:  Env{},
			want: "",
		},
		{
			// A remote signer command is not a key source this label covers.
			name: "only a signer command",
			env:  Env{SignerCommand: "vault-sign --key deployer"},
			want: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.env.SigningSource()
			assert.Equal(t, tc.want, got)
			// The label is safe to print: it never carries the secret itself.
			assert.NotContains(t, got, seed)
		})
	}
}
