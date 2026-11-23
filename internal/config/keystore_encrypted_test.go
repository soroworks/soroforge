package config

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/soroworks/soroforge/internal/keyseal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const encSeed = "SBFGFF27Y64ZUGFAIG5AMJGQODZZKV2YQKAVUUN4HNE24XZXD2OEUVUP"

func writeEncryptedKeystore(t *testing.T, seed, passphrase string) string {
	t.Helper()
	data, err := keyseal.EncryptWithParams([]byte(seed), []byte(passphrase),
		keyseal.Params{Time: 1, MemoryKiB: 64, Threads: 1})
	require.NoError(t, err)
	return writeKeystore(t, 0o600, string(data))
}

func TestSigningSeedDecryptsWithEnvPassphrase(t *testing.T) {
	path := writeEncryptedKeystore(t, encSeed, "hunter2 with spaces ")
	env := Env{KeystorePath: path, KeystorePassphrase: "hunter2 with spaces "}

	seed, source, err := env.SigningSeed()
	require.NoError(t, err)
	assert.Equal(t, encSeed, seed)
	assert.Equal(t, "keystore:"+path, source)
}

func TestSigningSeedPromptsWhenPassphraseUnset(t *testing.T) {
	path := writeEncryptedKeystore(t, encSeed, "prompted")
	var asked string
	env := Env{KeystorePath: path, PromptPassphrase: func(p string) (string, error) {
		asked = p
		return "prompted", nil
	}}

	seed, _, err := env.SigningSeed()
	require.NoError(t, err)
	assert.Equal(t, encSeed, seed)
	assert.Equal(t, path, asked, "the prompt names the file being unlocked")
}

func TestSigningSeedEnvPassphraseWinsOverPrompt(t *testing.T) {
	path := writeEncryptedKeystore(t, encSeed, "from-env")
	env := Env{KeystorePath: path, KeystorePassphrase: "from-env",
		PromptPassphrase: func(string) (string, error) {
			t.Fatal("must not prompt when the environment supplies the passphrase")
			return "", nil
		}}
	_, _, err := env.SigningSeed()
	require.NoError(t, err)
}

func TestSigningSeedEncryptedWithoutPassphraseIsClearError(t *testing.T) {
	path := writeEncryptedKeystore(t, encSeed, "pw")
	_, _, err := Env{KeystorePath: path}.SigningSeed()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is encrypted")
	assert.Contains(t, err.Error(), EnvKeystorePassphrase)
}

func TestSigningSeedWrongPassphraseIsClearAndLeaksNothing(t *testing.T) {
	path := writeEncryptedKeystore(t, encSeed, "right-passphrase")
	_, _, err := Env{KeystorePath: path, KeystorePassphrase: "wrong-passphrase"}.SigningSeed()
	require.ErrorIs(t, err, keyseal.ErrWrongPassphrase)
	assert.Contains(t, err.Error(), path)
	assert.Contains(t, err.Error(), "wrong passphrase")
	assert.NotContains(t, err.Error(), "wrong-passphrase")
	assert.NotContains(t, err.Error(), "right-passphrase")
	assert.NotContains(t, err.Error(), encSeed)
}

func TestSigningSeedPromptErrorAndEmptyEntry(t *testing.T) {
	path := writeEncryptedKeystore(t, encSeed, "pw")

	_, _, err := Env{KeystorePath: path, PromptPassphrase: func(string) (string, error) {
		return "", errors.New("interrupted")
	}}.SigningSeed()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "interrupted")

	_, _, err = Env{KeystorePath: path, PromptPassphrase: func(string) (string, error) {
		return "", nil
	}}.SigningSeed()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no passphrase was entered")
}

func TestSigningSeedPlainKeystoreStillWorksWithPassphraseSet(t *testing.T) {
	path := writeKeystore(t, 0o600, encSeed+"\n")
	seed, _, err := Env{KeystorePath: path, KeystorePassphrase: "ignored"}.SigningSeed()
	require.NoError(t, err)
	assert.Equal(t, encSeed, seed)
}

func TestEncryptedKeystoreStillChecksPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no POSIX permission bits on Windows")
	}
	path := writeEncryptedKeystore(t, encSeed, "pw")
	require.NoError(t, os.Chmod(path, 0o644))
	_, _, err := Env{KeystorePath: path, KeystorePassphrase: "pw"}.SigningSeed()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "readable by group or others")
}

func TestCosignerSeedsDecryptMixedKeystores(t *testing.T) {
	plain := writeKeystore(t, 0o600, encSeed)
	enc := writeEncryptedKeystore(t, encSeed, "pw")
	env := Env{
		CosignerKeystores:  strings.Join([]string{plain, enc}, string(filepath.ListSeparator)),
		KeystorePassphrase: "pw",
	}
	seeds, sources, err := env.CosignerSeeds()
	require.NoError(t, err)
	assert.Equal(t, []string{encSeed, encSeed}, seeds)
	assert.Equal(t, []string{"keystore:" + plain, "keystore:" + enc}, sources)

	env.KeystorePassphrase = "nope"
	_, _, err = env.CosignerSeeds()
	require.ErrorIs(t, err, keyseal.ErrWrongPassphrase)
	assert.Contains(t, err.Error(), "co-signer keystore")
}

func TestPassphraseIsRedactedAndNeverLogged(t *testing.T) {
	t.Setenv(EnvKeystorePassphrase, "super-secret-passphrase")
	env := LoadEnv()
	assert.Equal(t, "super-secret-passphrase", env.KeystorePassphrase)

	red := env.Redacted()
	assert.Equal(t, "(set)", red[EnvKeystorePassphrase])

	var buf bytes.Buffer
	slog.New(slog.NewTextHandler(&buf, nil)).Info("env", "env", red)
	assert.NotContains(t, buf.String(), "super-secret-passphrase")
}
