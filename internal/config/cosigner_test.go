package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeCosignerKeystore(t *testing.T, dir, name, content string, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), mode))
	require.NoError(t, os.Chmod(path, mode))
	return path
}

func TestCosignerSeedsReadsEachFileInOrder(t *testing.T) {
	dir := t.TempDir()
	a := writeCosignerKeystore(t, dir, "a", "SAAAA\n", 0o600)
	b := writeCosignerKeystore(t, dir, "b", "  SBBBB  ", 0o600)

	env := Env{CosignerKeystores: strings.Join([]string{a, "", b}, string(os.PathListSeparator))}
	seeds, sources, err := env.CosignerSeeds()
	require.NoError(t, err)
	assert.Equal(t, []string{"SAAAA", "SBBBB"}, seeds)
	assert.Equal(t, []string{"keystore:" + a, "keystore:" + b}, sources)
}

func TestCosignerSeedsNoneConfigured(t *testing.T) {
	seeds, sources, err := Env{}.CosignerSeeds()
	require.NoError(t, err)
	assert.Empty(t, seeds)
	assert.Empty(t, sources)
}

func TestCosignerSeedsErrorsNeverEchoSeeds(t *testing.T) {
	dir := t.TempDir()
	const secret = "SSECRETTHATMUSTNOTAPPEAR"

	empty := writeCosignerKeystore(t, dir, "empty", "  \n", 0o600)
	_, _, err := Env{CosignerKeystores: empty}.CosignerSeeds()
	require.Error(t, err)
	assert.Contains(t, err.Error(), empty)
	assert.Contains(t, err.Error(), "is empty")

	missing := filepath.Join(dir, "missing")
	_, _, err = Env{CosignerKeystores: missing}.CosignerSeeds()
	require.Error(t, err)
	assert.Contains(t, err.Error(), missing)

	if runtime.GOOS != "windows" {
		open := writeCosignerKeystore(t, dir, "open", secret, 0o644)
		_, _, err = Env{CosignerKeystores: open}.CosignerSeeds()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "chmod 600")
		assert.NotContains(t, err.Error(), secret)

		seeds, _, err := Env{CosignerKeystores: open, AllowInsecureKeystore: true}.CosignerSeeds()
		require.NoError(t, err)
		assert.Equal(t, []string{secret}, seeds)
	}
}

func TestValidateSigningConfigRequiresPrimaryForCosigners(t *testing.T) {
	err := Env{CosignerKeystores: "/tmp/cosigner"}.ValidateSigningConfig()
	require.Error(t, err)
	assert.Contains(t, err.Error(), EnvCosignerKeystores)
	assert.Contains(t, err.Error(), "no primary signing key")

	assert.NoError(t, Env{CosignerKeystores: "/tmp/cosigner", KeystorePath: "/tmp/primary"}.ValidateSigningConfig())
}

func TestRedactedShowsCosignerPaths(t *testing.T) {
	env := Env{CosignerKeystores: "/keys/a:/keys/b"}
	assert.Equal(t, "/keys/a:/keys/b", env.Redacted()[EnvCosignerKeystores])
}
