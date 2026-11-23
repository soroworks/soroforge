package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/soroworks/soroforge/internal/config"
	"github.com/soroworks/soroforge/internal/keyseal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const keysTestSeed = "SBFGFF27Y64ZUGFAIG5AMJGQODZZKV2YQKAVUUN4HNE24XZXD2OEUVUP"

func plainKeystore(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "deployer.key")
	require.NoError(t, os.WriteFile(path, []byte(keysTestSeed+"\n"), 0o600))
	return path
}

// stubPassphrases makes the terminal prompt return answers in order.
func stubPassphrases(t *testing.T, tty bool, answers ...string) *[]string {
	t.Helper()
	var prompts []string
	origRead, origTTY := readPassphrase, passphraseIsTerminal
	t.Cleanup(func() { readPassphrase, passphraseIsTerminal = origRead, origTTY })
	passphraseIsTerminal = func() bool { return tty }
	readPassphrase = func(prompt string) (string, error) {
		prompts = append(prompts, prompt)
		require.NotEmpty(t, answers, "unexpected prompt %q", prompt)
		a := answers[0]
		answers = answers[1:]
		return a, nil
	}
	return &prompts
}

func TestKeysEncryptInPlaceWithEnvPassphrase(t *testing.T) {
	path := plainKeystore(t)
	stubPassphrases(t, false)
	var out bytes.Buffer

	require.NoError(t, encryptKeystore(&out, config.Env{KeystorePassphrase: "pw"}, path, ""))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.True(t, keyseal.IsEncrypted(data))
	assert.NotContains(t, string(data), keysTestSeed)
	assert.NotContains(t, out.String(), keysTestSeed)
	assert.NotContains(t, out.String(), "pw\n")
	assert.Contains(t, out.String(), "GA", "reports the public address it encrypted")

	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}

	// And the result loads through the normal signing path.
	seed, _, err := config.Env{KeystorePath: path, KeystorePassphrase: "pw"}.SigningSeed()
	require.NoError(t, err)
	assert.Equal(t, keysTestSeed, seed)
}

func TestKeysEncryptToOutputKeepsOriginal(t *testing.T) {
	path := plainKeystore(t)
	dest := filepath.Join(filepath.Dir(path), "deployer.key.enc")
	prompts := stubPassphrases(t, true, "typed", "typed")

	require.NoError(t, encryptKeystore(&bytes.Buffer{}, config.Env{}, path, dest))
	assert.Len(t, *prompts, 2, "a new passphrase is asked for twice")

	orig, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, keysTestSeed+"\n", string(orig))

	seed, _, err := config.Env{KeystorePath: dest, KeystorePassphrase: "typed"}.SigningSeed()
	require.NoError(t, err)
	assert.Equal(t, keysTestSeed, seed)
}

func TestKeysEncryptRefusals(t *testing.T) {
	t.Run("mismatched passphrases", func(t *testing.T) {
		path := plainKeystore(t)
		stubPassphrases(t, true, "one", "two")
		err := encryptKeystore(&bytes.Buffer{}, config.Env{}, path, "")
		require.ErrorContains(t, err, "do not match")
		data, _ := os.ReadFile(path)
		assert.Equal(t, keysTestSeed+"\n", string(data), "file untouched on failure")
	})

	t.Run("no terminal and no env", func(t *testing.T) {
		stubPassphrases(t, false)
		err := encryptKeystore(&bytes.Buffer{}, config.Env{}, plainKeystore(t), "")
		require.ErrorContains(t, err, config.EnvKeystorePassphrase)
	})

	t.Run("empty passphrase", func(t *testing.T) {
		stubPassphrases(t, true, "")
		err := encryptKeystore(&bytes.Buffer{}, config.Env{}, plainKeystore(t), "")
		require.ErrorIs(t, err, keyseal.ErrEmptyPassphrase)
	})

	t.Run("already encrypted", func(t *testing.T) {
		path := plainKeystore(t)
		stubPassphrases(t, false)
		env := config.Env{KeystorePassphrase: "pw"}
		require.NoError(t, encryptKeystore(&bytes.Buffer{}, env, path, ""))
		err := encryptKeystore(&bytes.Buffer{}, env, path, "")
		require.ErrorContains(t, err, "already encrypted")
	})

	t.Run("not a seed", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "junk.key")
		require.NoError(t, os.WriteFile(path, []byte("SNOTASEED"), 0o600))
		stubPassphrases(t, false)
		err := encryptKeystore(&bytes.Buffer{}, config.Env{KeystorePassphrase: "pw"}, path, "")
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "SNOTASEED")
	})
}

func TestKeysEncryptCommandIsRegistered(t *testing.T) {
	cmd, _, err := newRootCmd().Find([]string{"keys", "encrypt"})
	require.NoError(t, err)
	assert.Equal(t, "encrypt <keystore-file>", cmd.Use)
}
