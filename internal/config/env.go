package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// Environment variables recognised by SoroForge.
//
// Secrets live here and only here. Nothing in this list is ever written to
// soroforge.yaml, echoed by --json output, or included in a log line; see
// Env.Redacted and the security notes in the README.
const (
	// EnvConfigPath overrides the location of soroforge.yaml.
	EnvConfigPath = "SOROFORGE_CONFIG"

	// EnvDatabaseURL is the Postgres connection string for deployment history.
	EnvDatabaseURL = "DATABASE_URL"

	// EnvSecretKey holds the deployer's Stellar secret seed (S...). Convenient
	// for CI, where the value comes from the runner's secret store.
	EnvSecretKey = "SOROFORGE_SECRET_KEY"

	// EnvKeystorePath points at a file containing nothing but the secret seed.
	// Preferred for workstations: a file can be mode 0600, whereas an
	// environment variable is readable by every child process and tends to end
	// up in shell history.
	EnvKeystorePath = "SOROFORGE_KEYSTORE_PATH"

	// EnvAPIToken is the original bearer token for the HTTP API. It grants
	// read and write, so existing single-token deployments keep working. The
	// server refuses to start when it and EnvAPITokens are both unset.
	EnvAPIToken = "SOROFORGE_API_TOKEN"

	// EnvAPITokens configures scoped bearer tokens as a comma-separated list
	// of name:scope:token entries, where scope is "read" or "write" (write
	// implies read). Read-only tokens can call GET routes and receive 403 on
	// the mutating ones. Example:
	//
	//	SOROFORGE_API_TOKENS="dashboard:read:$DASHBOARD_TOKEN,ci:write:$CI_TOKEN"
	EnvAPITokens = "SOROFORGE_API_TOKENS"

	// EnvAPIAddr overrides the HTTP API listen address.
	EnvAPIAddr = "SOROFORGE_API_ADDR"

	// EnvAPICORSOrigins lists the exact browser origins allowed to call the
	// HTTP API from another origin, as a comma-separated list such as
	// "https://dash.example.com,https://ops.example.com". Unset (the default)
	// means the API sends no CORS headers at all. Because the API can deploy
	// contracts with the configured signing key, wildcards are rejected — see
	// ParseCORSOrigins and ValidateCORSOrigin.
	EnvAPICORSOrigins = "SOROFORGE_API_CORS_ORIGINS"

	// EnvLogFormat selects the logging handler: "text" (default) or "json".
	// JSON is for CI and log aggregators that index structured fields.
	EnvLogFormat = "SOROFORGE_LOG_FORMAT"
)

// DefaultAPIAddr is the listen address used when EnvAPIAddr is unset.
//
// It binds loopback rather than 0.0.0.0 deliberately: this process holds a
// signing key, and exposing it to the network should be a decision someone
// makes on purpose.
const DefaultAPIAddr = "127.0.0.1:8080"

// Env holds configuration sourced from the process environment.
//
// The struct is deliberately not tagged for YAML or JSON. SecretKey must not be
// serialised anywhere, and leaving the tags off means an accidental
// json.Marshal of a Config cannot leak it.
type Env struct {
	DatabaseURL    string
	SecretKey      string
	KeystorePath   string
	APIToken       string
	APITokens      string
	APIAddr        string
	APICORSOrigins string
	LogFormat      string
}

// LoadEnv reads SoroForge's environment variables. It does not validate them:
// most commands need only a subset, so each entry point checks what it needs
// via RequireDatabaseURL or SigningSeed.
func LoadEnv() Env {
	return Env{
		DatabaseURL:    strings.TrimSpace(os.Getenv(EnvDatabaseURL)),
		SecretKey:      strings.TrimSpace(os.Getenv(EnvSecretKey)),
		KeystorePath:   strings.TrimSpace(os.Getenv(EnvKeystorePath)),
		APIToken:       os.Getenv(EnvAPIToken),
		APITokens:      strings.TrimSpace(os.Getenv(EnvAPITokens)),
		APIAddr:        strings.TrimSpace(os.Getenv(EnvAPIAddr)),
		APICORSOrigins: strings.TrimSpace(os.Getenv(EnvAPICORSOrigins)),
		LogFormat:      strings.ToLower(strings.TrimSpace(os.Getenv(EnvLogFormat))),
	}
}

// ParseCORSOrigins splits EnvAPICORSOrigins's comma-separated value into
// validated exact origins. Empty input yields no origins, which is how CORS
// stays opt-in: an unset variable must produce exactly today's behaviour of no
// CORS headers at all. Entries are trimmed and empty ones skipped, so a
// trailing comma or spaces around the list are harmless rather than a startup
// failure. An invalid entry is an error, so a typo cannot silently disable or
// silently widen the policy.
func ParseCORSOrigins(raw string) ([]string, error) {
	var origins []string
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if err := ValidateCORSOrigin(entry); err != nil {
			return nil, err
		}
		origins = append(origins, entry)
	}
	return origins, nil
}

// ValidateCORSOrigin reports whether origin is an exact origin — scheme, host,
// optional port, nothing else — that a dashboard could be served from.
//
// The rules are deliberately strict because this API can deploy contracts with
// the configured signing key. "*" (or a wildcard anywhere in an entry) would
// let any page a user visits drive the API from the browser, so cross-origin
// access must be a narrow, explicit opt-in; the error says so rather than
// quietly accepting it. Paths, queries, and trailing slashes are refused
// because browsers compare origins exactly — accepting them would configure a
// list that never matches, with CORS silently off.
//
// It is exported because api.NewRouter holds Options.CORSOrigins to the same
// rules: a server must refuse a wildcard from either entry point, not only
// from the environment variable.
func ValidateCORSOrigin(origin string) error {
	if strings.Contains(origin, "*") {
		return fmt.Errorf(
			"%s must not contain \"*\": list the exact origins the dashboard is served from",
			EnvAPICORSOrigins)
	}

	u, err := url.Parse(origin)
	switch {
	case err != nil || u.Opaque != "" || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "":
		return fmt.Errorf(
			"%s: %q is not a valid origin: expected scheme://host[:port] (e.g. https://dash.example.com)",
			EnvAPICORSOrigins, origin)
	case u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil:
		return fmt.Errorf(
			"%s: %q is not a valid origin: an origin has no path, query, or trailing slash",
			EnvAPICORSOrigins, origin)
	}
	return nil
}

// RequireDatabaseURL returns the configured Postgres URL, or an error naming
// the variable to set.
func (e Env) RequireDatabaseURL() (string, error) {
	if e.DatabaseURL == "" {
		return "", fmt.Errorf("%s is not set; SoroForge needs Postgres to record deployment history", EnvDatabaseURL)
	}
	return e.DatabaseURL, nil
}

// ListenAddr returns the HTTP API listen address, defaulting to DefaultAPIAddr.
func (e Env) ListenAddr() string {
	if e.APIAddr == "" {
		return DefaultAPIAddr
	}
	return e.APIAddr
}

// SigningSeed returns the deployer's secret seed from whichever source is
// configured, along with a label describing that source for diagnostics.
//
// The returned seed is a secret: callers must hand it straight to a signer and
// must not log it, store it, or include it in command output. The label exists
// so operators can confirm *which* key source was used without the value ever
// being printed.
//
// EnvSecretKey wins over EnvKeystorePath when both are set, so that CI can
// override a developer's on-disk key without editing files.
func (e Env) SigningSeed() (seed string, source string, err error) {
	if e.SecretKey != "" {
		return e.SecretKey, "env:" + EnvSecretKey, nil
	}

	if e.KeystorePath != "" {
		raw, err := os.ReadFile(e.KeystorePath)
		if err != nil {
			return "", "", fmt.Errorf("read keystore %s: %w", e.KeystorePath, err)
		}
		// Trim so a trailing newline from `echo S... > keystore` is tolerated;
		// a stray newline in a seed is a confusing failure to debug.
		seed := strings.TrimSpace(string(raw))
		if seed == "" {
			return "", "", fmt.Errorf("keystore %s is empty", e.KeystorePath)
		}
		return seed, "keystore:" + e.KeystorePath, nil
	}

	return "", "", fmt.Errorf(
		"no signing key configured: set %s or %s (see README security notes)",
		EnvSecretKey, EnvKeystorePath,
	)
}

// HasSigningKey reports whether a signing key source is configured. Read-only
// commands use it to give a clear error instead of failing deeper in the stack.
func (e Env) HasSigningKey() bool {
	return e.SecretKey != "" || e.KeystorePath != ""
}

// Redacted returns a copy safe to print or log: every secret is replaced with a
// fixed placeholder, and the database URL keeps only its shape.
func (e Env) Redacted() map[string]string {
	return map[string]string{
		EnvDatabaseURL:    redact(e.DatabaseURL),
		EnvSecretKey:      redact(e.SecretKey),
		EnvKeystorePath:   e.KeystorePath, // a path is not itself a secret
		EnvAPIToken:       redact(e.APIToken),
		EnvAPITokens:      redact(e.APITokens),
		EnvAPIAddr:        e.ListenAddr(),
		EnvAPICORSOrigins: e.APICORSOrigins, // origins are public, not secrets
		EnvLogFormat:      e.LogFormat,
	}
}

func redact(v string) string {
	if v == "" {
		return "(unset)"
	}
	return "(set)"
}
