// Package keyseal reads and writes passphrase-encrypted keystore files.
//
// A plain keystore is the deployer's secret seed (S...) in a text file,
// protected only by file permissions. Anything that can read the file — a
// backup, a misconfigured volume mount, a laptop image — has the key. An
// encrypted keystore holds the same seed sealed under a key derived from a
// passphrase, so a copy of the file alone is not enough to sign.
//
// # File format (version 1)
//
// An encrypted keystore is a single JSON object:
//
//	{
//	  "soroforge_keystore": 1,
//	  "kdf": "argon2id",
//	  "kdf_params": {"time": 3, "memory_kib": 65536, "threads": 4, "salt": "<base64, 16 bytes>"},
//	  "cipher": "xchacha20-poly1305",
//	  "nonce": "<base64, 24 bytes>",
//	  "ciphertext": "<base64>"
//	}
//
// The key is argon2id(passphrase, salt, time, memory_kib, threads) with a
// 32-byte output, and the ciphertext is XChaCha20-Poly1305 over the seed.
// Every header field (version, kdf, its parameters including the salt, and
// cipher) is bound into the AEAD as additional data, so tampering with the
// parameters is detected rather than silently weakening decryption.
//
// "soroforge_keystore" is the format version. A reader rejects versions it
// does not know instead of guessing, so the format can evolve (a new KDF, a
// new cipher) by bumping it. All base64 is standard encoding with padding.
//
// Detection is unambiguous: a Stellar secret seed is base32 and starts with
// 'S', so a file whose first non-space byte is '{' is never a plain seed.
package keyseal

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

// Version is the file format version this package writes.
const Version = 1

const (
	kdfArgon2id = "argon2id"
	cipherXC20  = "xchacha20-poly1305"
	saltLen     = 16
	keyLen      = chacha20poly1305.KeySize
)

// Params are the argon2id cost parameters used when encrypting.
type Params struct {
	Time      uint32 // iterations
	MemoryKiB uint32 // memory in KiB
	Threads   uint8  // parallelism
}

// DefaultParams follow the RFC 9106 "second recommended option" with memory
// reduced to 64 MiB: about a quarter of a second on a laptop, which is paid
// once per command, and expensive enough per guess to make an offline
// dictionary attack on a stolen file slow.
var DefaultParams = Params{Time: 3, MemoryKiB: 64 * 1024, Threads: 4}

// Limits applied when decrypting, so a crafted file cannot make SoroForge
// allocate gigabytes or spin for minutes before the passphrase is checked.
const (
	maxMemoryKiB = 1024 * 1024 // 1 GiB
	maxTime      = 64
)

// ErrWrongPassphrase is returned when the passphrase does not open the
// keystore. A corrupted or tampered file produces the same error: the AEAD
// cannot tell the two apart, and saying which would help an attacker.
var ErrWrongPassphrase = errors.New("wrong passphrase, or the keystore file is corrupted")

// ErrEmptyPassphrase is returned when encrypting or decrypting with an empty
// passphrase.
var ErrEmptyPassphrase = errors.New("passphrase must not be empty")

type kdfParams struct {
	Time      uint32 `json:"time"`
	MemoryKiB uint32 `json:"memory_kib"`
	Threads   uint8  `json:"threads"`
	Salt      []byte `json:"salt"`
}

type file struct {
	Version    int       `json:"soroforge_keystore"`
	KDF        string    `json:"kdf"`
	KDFParams  kdfParams `json:"kdf_params"`
	Cipher     string    `json:"cipher"`
	Nonce      []byte    `json:"nonce"`
	Ciphertext []byte    `json:"ciphertext"`
}

// additionalData is the canonical encoding of the header that the AEAD
// authenticates. It is built by hand, not from JSON, so that key order or
// whitespace in the file never changes it.
func (f *file) additionalData() []byte {
	var b bytes.Buffer
	b.WriteString("soroforge_keystore:")
	b.WriteString(strconv.Itoa(f.Version))
	b.WriteString("|kdf:")
	b.WriteString(f.KDF)
	b.WriteString("|time:")
	b.WriteString(strconv.FormatUint(uint64(f.KDFParams.Time), 10))
	b.WriteString("|memory_kib:")
	b.WriteString(strconv.FormatUint(uint64(f.KDFParams.MemoryKiB), 10))
	b.WriteString("|threads:")
	b.WriteString(strconv.FormatUint(uint64(f.KDFParams.Threads), 10))
	b.WriteString("|salt:")
	b.WriteString(base64.StdEncoding.EncodeToString(f.KDFParams.Salt))
	b.WriteString("|cipher:")
	b.WriteString(f.Cipher)
	return b.Bytes()
}

// IsEncrypted reports whether data looks like an encrypted keystore rather
// than a plain seed. It does not validate the file; Decrypt does that.
func IsEncrypted(data []byte) bool {
	trimmed := bytes.TrimSpace(data)
	return len(trimmed) > 0 && trimmed[0] == '{'
}

// Encrypt seals seed under passphrase with DefaultParams and returns the file
// contents, ending in a newline.
func Encrypt(seed, passphrase []byte) ([]byte, error) {
	return EncryptWithParams(seed, passphrase, DefaultParams)
}

// EncryptWithParams is Encrypt with explicit argon2id cost parameters. Tests
// use cheap parameters; production callers should use Encrypt.
func EncryptWithParams(seed, passphrase []byte, p Params) ([]byte, error) {
	if len(passphrase) == 0 {
		return nil, ErrEmptyPassphrase
	}
	if len(seed) == 0 {
		return nil, errors.New("keystore: nothing to encrypt")
	}
	if p.Time == 0 || p.MemoryKiB == 0 || p.Threads == 0 {
		return nil, errors.New("keystore: argon2id parameters must be non-zero")
	}

	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("keystore: generate salt: %w", err)
	}
	nonce := make([]byte, chacha20poly1305.NonceSizeX)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("keystore: generate nonce: %w", err)
	}

	f := &file{
		Version: Version,
		KDF:     kdfArgon2id,
		KDFParams: kdfParams{
			Time:      p.Time,
			MemoryKiB: p.MemoryKiB,
			Threads:   p.Threads,
			Salt:      salt,
		},
		Cipher: cipherXC20,
		Nonce:  nonce,
	}

	key := argon2.IDKey(passphrase, salt, p.Time, p.MemoryKiB, p.Threads, keyLen)
	defer wipe(key)
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, fmt.Errorf("keystore: %w", err)
	}
	f.Ciphertext = aead.Seal(nil, nonce, seed, f.additionalData())

	out, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("keystore: encode: %w", err)
	}
	return append(out, '\n'), nil
}

// Decrypt opens an encrypted keystore and returns the seed it holds. The
// error never includes the passphrase or any part of the plaintext.
func Decrypt(data, passphrase []byte) ([]byte, error) {
	if len(passphrase) == 0 {
		return nil, ErrEmptyPassphrase
	}

	var f file
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("not a valid encrypted keystore: %w", err)
	}
	if f.Version != Version {
		return nil, fmt.Errorf("unsupported keystore format version %d (this build reads version %d)", f.Version, Version)
	}
	if f.KDF != kdfArgon2id {
		return nil, fmt.Errorf("unsupported keystore kdf %q", f.KDF)
	}
	if f.Cipher != cipherXC20 {
		return nil, fmt.Errorf("unsupported keystore cipher %q", f.Cipher)
	}
	kp := f.KDFParams
	switch {
	case kp.Time == 0 || kp.MemoryKiB == 0 || kp.Threads == 0:
		return nil, errors.New("invalid keystore: argon2id parameters must be non-zero")
	case kp.Time > maxTime || kp.MemoryKiB > maxMemoryKiB:
		return nil, fmt.Errorf("invalid keystore: argon2id parameters exceed limits (time <= %d, memory_kib <= %d)", maxTime, maxMemoryKiB)
	case len(kp.Salt) < saltLen:
		return nil, fmt.Errorf("invalid keystore: salt must be at least %d bytes", saltLen)
	case len(f.Nonce) != chacha20poly1305.NonceSizeX:
		return nil, fmt.Errorf("invalid keystore: nonce must be %d bytes", chacha20poly1305.NonceSizeX)
	case len(f.Ciphertext) <= chacha20poly1305.Overhead:
		return nil, errors.New("invalid keystore: ciphertext is too short")
	}

	key := argon2.IDKey(passphrase, kp.Salt, kp.Time, kp.MemoryKiB, kp.Threads, keyLen)
	defer wipe(key)
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, fmt.Errorf("keystore: %w", err)
	}
	seed, err := aead.Open(nil, f.Nonce, f.Ciphertext, f.additionalData())
	if err != nil {
		return nil, ErrWrongPassphrase
	}
	return seed, nil
}

// wipe zeroes a derived key once it is no longer needed. Go gives no
// guarantee that no copy survives elsewhere, but there is no reason to leave
// this one around.
func wipe(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
