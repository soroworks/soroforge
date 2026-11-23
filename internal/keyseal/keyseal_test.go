package keyseal

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cheap keeps tests fast; production uses DefaultParams.
var cheap = Params{Time: 1, MemoryKiB: 64, Threads: 1}

const testSeed = "SBFGFF27Y64ZUGFAIG5AMJGQODZZKV2YQKAVUUN4HNE24XZXD2OEUVUP"

func TestRoundTrip(t *testing.T) {
	data, err := EncryptWithParams([]byte(testSeed), []byte("correct horse"), cheap)
	require.NoError(t, err)
	assert.True(t, IsEncrypted(data))
	assert.NotContains(t, string(data), testSeed)
	assert.NotContains(t, string(data), "correct horse")

	seed, err := Decrypt(data, []byte("correct horse"))
	require.NoError(t, err)
	assert.Equal(t, testSeed, string(seed))
}

func TestEncryptIsRandomised(t *testing.T) {
	a, err := EncryptWithParams([]byte(testSeed), []byte("pw"), cheap)
	require.NoError(t, err)
	b, err := EncryptWithParams([]byte(testSeed), []byte("pw"), cheap)
	require.NoError(t, err)
	assert.NotEqual(t, a, b, "salt and nonce must differ between encryptions")
}

func TestWrongPassphrase(t *testing.T) {
	data, err := EncryptWithParams([]byte(testSeed), []byte("right"), cheap)
	require.NoError(t, err)

	_, err = Decrypt(data, []byte("wrong"))
	require.ErrorIs(t, err, ErrWrongPassphrase)
	assert.NotContains(t, err.Error(), "wrong\"")
	assert.NotContains(t, err.Error(), testSeed)
}

func TestEmptyPassphraseRefused(t *testing.T) {
	_, err := EncryptWithParams([]byte(testSeed), nil, cheap)
	require.ErrorIs(t, err, ErrEmptyPassphrase)

	data, err := EncryptWithParams([]byte(testSeed), []byte("pw"), cheap)
	require.NoError(t, err)
	_, err = Decrypt(data, []byte(""))
	require.ErrorIs(t, err, ErrEmptyPassphrase)
}

func TestTamperedHeaderIsDetected(t *testing.T) {
	data, err := EncryptWithParams([]byte(testSeed), []byte("pw"), cheap)
	require.NoError(t, err)

	var f map[string]any
	require.NoError(t, json.Unmarshal(data, &f))
	params := f["kdf_params"].(map[string]any)
	params["time"] = 2 // still valid, but not what the ciphertext was sealed with
	tampered, err := json.Marshal(f)
	require.NoError(t, err)

	_, err = Decrypt(tampered, []byte("pw"))
	require.ErrorIs(t, err, ErrWrongPassphrase, "changing the KDF parameters must fail authentication")
}

func TestDecryptRejectsBadFiles(t *testing.T) {
	good, err := EncryptWithParams([]byte(testSeed), []byte("pw"), cheap)
	require.NoError(t, err)

	mutate := func(fn func(m map[string]any)) []byte {
		var m map[string]any
		require.NoError(t, json.Unmarshal(good, &m))
		fn(m)
		out, err := json.Marshal(m)
		require.NoError(t, err)
		return out
	}

	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"not json", []byte("{nope"), "not a valid encrypted keystore"},
		{"future version", mutate(func(m map[string]any) { m["soroforge_keystore"] = 2 }), "unsupported keystore format version 2"},
		{"unknown kdf", mutate(func(m map[string]any) { m["kdf"] = "pbkdf2" }), "unsupported keystore kdf"},
		{"unknown cipher", mutate(func(m map[string]any) { m["cipher"] = "aes-128-ecb" }), "unsupported keystore cipher"},
		{"huge memory", mutate(func(m map[string]any) {
			m["kdf_params"].(map[string]any)["memory_kib"] = 1 << 30
		}), "exceed limits"},
		{"zero time", mutate(func(m map[string]any) {
			m["kdf_params"].(map[string]any)["time"] = 0
		}), "must be non-zero"},
		{"short nonce", mutate(func(m map[string]any) { m["nonce"] = "AAAA" }), "nonce must be"},
		{"unknown field", mutate(func(m map[string]any) { m["extra"] = true }), "not a valid encrypted keystore"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Decrypt(tc.data, []byte("pw"))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestIsEncrypted(t *testing.T) {
	assert.False(t, IsEncrypted([]byte(testSeed)))
	assert.False(t, IsEncrypted([]byte(testSeed+"\n")))
	assert.False(t, IsEncrypted(nil))
	assert.False(t, IsEncrypted([]byte("   \n")))
	assert.True(t, IsEncrypted([]byte("\n  {\"soroforge_keystore\": 1}")))
}

func TestFileFormatIsDocumented(t *testing.T) {
	// The package doc promises these field names; a rename is a format
	// change and must bump Version.
	data, err := EncryptWithParams([]byte(testSeed), []byte("pw"), cheap)
	require.NoError(t, err)
	for _, field := range []string{`"soroforge_keystore": 1`, `"kdf": "argon2id"`, `"memory_kib"`, `"salt"`, `"cipher": "xchacha20-poly1305"`, `"nonce"`, `"ciphertext"`} {
		assert.True(t, strings.Contains(string(data), field), "missing %s", field)
	}
}
