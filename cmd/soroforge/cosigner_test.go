package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/soroworks/soroforge/internal/config"
	"github.com/soroworks/soroforge/internal/stellar"
	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cosignerKeystore(t *testing.T, seed string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cosigner")
	require.NoError(t, os.WriteFile(path, []byte(seed+"\n"), 0o600))
	return path
}

func TestBuildSignerWrapsCosigners(t *testing.T) {
	primary, err := keypair.Random()
	require.NoError(t, err)
	co, err := keypair.Random()
	require.NoError(t, err)

	cfg := &config.Config{Env: config.Env{
		SecretKey:         primary.Seed(),
		CosignerKeystores: cosignerKeystore(t, co.Seed()),
	}}
	signer, err := buildSigner(cfg, true)
	require.NoError(t, err)

	multi, ok := signer.(*stellar.MultiSigner)
	require.True(t, ok, "co-signers wrap the primary in a MultiSigner, got %T", signer)
	assert.Equal(t, primary.Address(), multi.Address())
	assert.Equal(t, 2, multi.Signers())
}

func TestBuildSignerWithoutCosignersIsUnchanged(t *testing.T) {
	primary, err := keypair.Random()
	require.NoError(t, err)

	signer, err := buildSigner(&config.Config{Env: config.Env{SecretKey: primary.Seed()}}, true)
	require.NoError(t, err)
	_, ok := signer.(*stellar.KeypairSigner)
	assert.True(t, ok, "got %T", signer)
}

func TestBuildSignerRejectsPrimaryAsCosigner(t *testing.T) {
	primary, err := keypair.Random()
	require.NoError(t, err)

	cfg := &config.Config{Env: config.Env{
		SecretKey:         primary.Seed(),
		CosignerKeystores: cosignerKeystore(t, primary.Seed()),
	}}
	_, err = buildSigner(cfg, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "more than once")
	assert.NotContains(t, err.Error(), primary.Seed())
}

func TestBuildSignerRejectsMalformedCosignerWithoutEchoingIt(t *testing.T) {
	primary, err := keypair.Random()
	require.NoError(t, err)
	const bad = "SNOTAREALSEEDBUTSTILLSECRET"
	path := cosignerKeystore(t, bad)

	_, err = buildSigner(&config.Config{Env: config.Env{SecretKey: primary.Seed(), CosignerKeystores: path}}, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), path)
	assert.NotContains(t, err.Error(), bad)
}
