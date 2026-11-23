package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/soroworks/soroforge/internal/config"
	"github.com/soroworks/soroforge/internal/keyseal"
	"github.com/soroworks/soroforge/internal/stellar"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// readPassphrase prints prompt to stderr and reads a line from the terminal
// without echoing it. It is a variable so tests can substitute input.
var readPassphrase = func(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// passphraseIsTerminal reports whether a passphrase can be prompted for. It
// asks the terminal driver rather than checking for a character device, since
// /dev/null is one too and reading a passphrase from it fails. A variable for
// the same reason as readPassphrase.
var passphraseIsTerminal = func() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

func newKeysCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keys",
		Short: "Manage keystore files",
	}
	cmd.AddCommand(newKeysEncryptCmd())
	return cmd
}

func newKeysEncryptCmd() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "encrypt <keystore-file>",
		Short: "Encrypt a plain keystore file with a passphrase",
		Long: `Encrypt a plain keystore (a file holding an S... secret seed) with a passphrase.

The key is derived with argon2id and the seed is sealed with XChaCha20-Poly1305.
By default the file is replaced in place; use --output to write elsewhere and
keep the original. The result is written with mode 0600.

The passphrase comes from SOROFORGE_KEYSTORE_PASSPHRASE or, on a terminal, is
prompted for twice without echo. Encrypted keystores are detected automatically
wherever SOROFORGE_KEYSTORE_PATH or SOROFORGE_COSIGNER_KEYSTORES is read.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return encryptKeystore(cmd.ErrOrStderr(), config.LoadEnv(), args[0], output)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "",
		"write the encrypted keystore here instead of replacing the input file")
	return cmd
}

// encryptKeystore encrypts the plain keystore at path and writes it to output,
// or back to path when output is empty. Neither the seed nor the passphrase
// appears in any message.
func encryptKeystore(w io.Writer, env config.Env, path, output string) error {
	raw, err := os.ReadFile(path) //nolint:gosec // G304: the operator names the keystore on the command line
	if err != nil {
		return fmt.Errorf("read keystore %s: %w", path, err)
	}
	if keyseal.IsEncrypted(raw) {
		return fmt.Errorf("keystore %s is already encrypted", path)
	}

	seed := strings.TrimSpace(string(raw))
	if seed == "" {
		return fmt.Errorf("keystore %s is empty", path)
	}
	// Refuse to encrypt something that is not a seed: finding out at deploy
	// time, behind a passphrase, is a worse place to discover a typo.
	signer, err := stellar.NewKeypairSigner(seed)
	if err != nil {
		return fmt.Errorf("keystore %s: %w", path, err)
	}

	passphrase, err := newPassphrase(env)
	if err != nil {
		return err
	}

	sealed, err := keyseal.Encrypt([]byte(seed), []byte(passphrase))
	if err != nil {
		return err
	}

	dest := output
	if dest == "" {
		dest = path
	}
	if err := writeFileAtomic(dest, sealed, 0o600); err != nil {
		return fmt.Errorf("write encrypted keystore %s: %w", dest, err)
	}

	fmt.Fprintf(w, "Encrypted keystore for %s written to %s\n", signer.Address(), dest)
	if output != "" {
		fmt.Fprintf(w, "The plain keystore %s is unchanged; delete it once you have checked the encrypted copy.\n", path)
	}
	return nil
}

// newPassphrase returns the passphrase to encrypt with: the environment
// variable if set, otherwise a terminal prompt entered twice.
func newPassphrase(env config.Env) (string, error) {
	if env.KeystorePassphrase != "" {
		return env.KeystorePassphrase, nil
	}
	if !passphraseIsTerminal() {
		return "", fmt.Errorf("no passphrase: set %s, or run on a terminal to be prompted", config.EnvKeystorePassphrase)
	}
	first, err := readPassphrase("New passphrase: ")
	if err != nil {
		return "", fmt.Errorf("read passphrase: %w", err)
	}
	if first == "" {
		return "", keyseal.ErrEmptyPassphrase
	}
	second, err := readPassphrase("Repeat passphrase: ")
	if err != nil {
		return "", fmt.Errorf("read passphrase: %w", err)
	}
	if first != second {
		return "", errors.New("passphrases do not match")
	}
	return first, nil
}

// writeFileAtomic writes data to a temporary file next to path and renames it
// into place, so an interrupted write never leaves a half-written keystore
// where the only copy of a key used to be.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }

	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return err
	}
	return nil
}
