package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteScaffoldMissingDestinationDirectory(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "missing", "soroforge.yaml")

	_, err := WriteScaffold(dir, dest, false)
	require.Error(t, err)

	_, statErr := os.Stat(dest)
	assert.True(t, os.IsNotExist(statErr), "no partial file may be left behind")
	_, statErr = os.Stat(filepath.Dir(dest))
	assert.True(t, os.IsNotExist(statErr), "the missing directory must not be created")
}

// A scan directory that does not exist has no built contracts in it, which is
// the same situation as a fresh project: the placeholder config is written so
// `soroforge init` still produces a file that loads. DiscoverContracts globs,
// and a glob over a missing directory matches nothing rather than failing.
func TestWriteScaffoldMissingScanDirectoryWritesPlaceholder(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "soroforge.yaml")

	found, err := WriteScaffold(filepath.Join(dir, "does-not-exist"), dest, false)
	require.NoError(t, err)
	assert.Empty(t, found)

	body, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Contains(t, string(body), placeholderAlias+":")

	_, err = Load(dest)
	assert.NoError(t, err, "the placeholder config must load")
}

func TestWriteScaffoldForceOverReadOnlyFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		// Windows ignores the write bit for the file's owner in a way that
		// makes this case behave differently; it is covered on Unix.
		t.Skip("read-only file permissions differ on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root can write to read-only files")
	}

	dir := t.TempDir()
	dest := filepath.Join(dir, "soroforge.yaml")
	require.NoError(t, os.WriteFile(dest, []byte("original\n"), 0o444))

	require.NotPanics(t, func() {
		_, err := WriteScaffold(dir, dest, true)
		assert.Error(t, err)
	})

	body, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, "original\n", string(body), "the read-only file must be untouched")
}
