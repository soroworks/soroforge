package deploy

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"

	"github.com/soroworks/soroforge/internal/config"
	"github.com/soroworks/soroforge/internal/store"
)

// DefaultWebhookTimeout bounds a single webhook delivery attempt.
const DefaultWebhookTimeout = 10 * time.Second

// Retry defaults. A receiver that is restarting or sitting behind a load
// balancer returning 502 usually recovers within seconds; four attempts over
// roughly 0.5s, 1s and 2s of backoff cover that without letting a dead
// receiver hold a deploy result for long.
const (
	// DefaultWebhookMaxAttempts is the total number of delivery attempts,
	// including the first.
	DefaultWebhookMaxAttempts = 4
	// DefaultWebhookBaseBackoff is the backoff before the second attempt; it
	// doubles for each attempt after that.
	DefaultWebhookBaseBackoff = 500 * time.Millisecond
	// DefaultWebhookMaxBackoff caps a single wait, including one requested
	// by a Retry-After header.
	DefaultWebhookMaxBackoff = 10 * time.Second
	// DefaultWebhookMaxElapsed bounds the whole delivery, attempts and waits
	// together, so a dead receiver cannot stall a deploy result.
	DefaultWebhookMaxElapsed = 30 * time.Second
)

// WebhookStatus reports the outcome of delivering a webhook to one URL.
type WebhookStatus struct {
	URL   string `json:"url"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	// Attempts is how many HTTP requests were made, including retries.
	Attempts int `json:"attempts,omitempty"`
}

// WebhookPayload is the JSON payload sent to configured webhook endpoints after
// a confirmed deploy or upgrade.
type WebhookPayload struct {
	Alias          string       `json:"alias"`
	Network        string       `json:"network"`
	ContractID     string       `json:"contract_id"`
	Contract       string       `json:"contract,omitempty"`
	WasmHash       string       `json:"wasm_hash"`
	Action         store.Action `json:"action"`
	DeployerPubkey string       `json:"deployer_pubkey"`
	TxHash         string       `json:"tx_hash,omitempty"`
	Tx             string       `json:"tx,omitempty"`
	Ledger         uint32       `json:"ledger,omitempty"`
	Notes          string       `json:"notes,omitempty"`
}

// WebhookSender delivers webhooks.
type WebhookSender interface {
	Send(ctx context.Context, webhook config.Webhook, payload WebhookPayload) error
}

// AttemptCountingSender is implemented by senders that retry, so a delivery
// status can report how many attempts were made. Senders that do not
// implement it are counted as one attempt.
type AttemptCountingSender interface {
	Deliver(ctx context.Context, webhook config.Webhook, payload WebhookPayload) (attempts int, err error)
}

// WebhookRetryPolicy controls how HTTPWebhookSender retries a failed delivery.
// Zero fields take the package defaults.
type WebhookRetryPolicy struct {
	// MaxAttempts is the total number of attempts, including the first.
	MaxAttempts int
	// BaseBackoff is the wait before the second attempt; it doubles after.
	BaseBackoff time.Duration
	// MaxBackoff caps any single wait, including a Retry-After request.
	MaxBackoff time.Duration
	// MaxElapsed bounds the whole delivery. A retry whose wait would cross
	// it is not attempted.
	MaxElapsed time.Duration

	// Sleep waits for d or until ctx is done. Tests inject a recorder so
	// they never actually sleep. Defaults to a timer-based wait.
	Sleep func(ctx context.Context, d time.Duration) error
	// Jitter maps a computed backoff to the duration actually waited.
	// Defaults to "full jitter": a uniform random duration in [d/2, d].
	Jitter func(d time.Duration) time.Duration
	// Now returns the current time; it lets tests drive the elapsed budget.
	Now func() time.Time
}

func (p WebhookRetryPolicy) withDefaults() WebhookRetryPolicy {
	if p.MaxAttempts <= 0 {
		p.MaxAttempts = DefaultWebhookMaxAttempts
	}
	if p.BaseBackoff <= 0 {
		p.BaseBackoff = DefaultWebhookBaseBackoff
	}
	if p.MaxBackoff <= 0 {
		p.MaxBackoff = DefaultWebhookMaxBackoff
	}
	if p.MaxElapsed <= 0 {
		p.MaxElapsed = DefaultWebhookMaxElapsed
	}
	if p.Sleep == nil {
		p.Sleep = sleepContext
	}
	if p.Jitter == nil {
		p.Jitter = halfJitter
	}
	if p.Now == nil {
		p.Now = time.Now
	}
	return p
}

func sleepContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// halfJitter spreads retries from many deployers so they do not hit a
// recovering receiver in lockstep, while keeping at least half the backoff.
func halfJitter(d time.Duration) time.Duration {
	if d <= 1 {
		return d
	}
	half := d / 2
	return half + rand.N(d-half+1) //nolint:gosec // jitter timing, not security-sensitive
}

// HTTPWebhookSender delivers webhooks via HTTP POST, retrying transient
// failures according to its WebhookRetryPolicy.
type HTTPWebhookSender struct {
	client *http.Client
	policy WebhookRetryPolicy
}

var (
	_ WebhookSender         = (*HTTPWebhookSender)(nil)
	_ AttemptCountingSender = (*HTTPWebhookSender)(nil)
)

// NewHTTPWebhookSender returns a WebhookSender using client, or an http.Client with
// DefaultWebhookTimeout when client is nil. It retries with the default policy.
func NewHTTPWebhookSender(client *http.Client) *HTTPWebhookSender {
	return NewHTTPWebhookSenderWithPolicy(client, WebhookRetryPolicy{})
}

// NewHTTPWebhookSenderWithPolicy is NewHTTPWebhookSender with an explicit
// retry policy; zero fields of policy take the defaults.
func NewHTTPWebhookSenderWithPolicy(client *http.Client, policy WebhookRetryPolicy) *HTTPWebhookSender {
	if client == nil {
		client = &http.Client{Timeout: DefaultWebhookTimeout}
	}
	return &HTTPWebhookSender{client: client, policy: policy.withDefaults()}
}

// Send delivers a JSON payload to a webhook URL, optionally signing with HMAC-SHA256.
func (s *HTTPWebhookSender) Send(ctx context.Context, wh config.Webhook, payload WebhookPayload) error {
	_, err := s.Deliver(ctx, wh, payload)
	return err
}

// Deliver is Send that also reports the number of attempts made.
//
// 5xx responses, 429 and network errors are retried with exponential backoff
// and jitter; any other non-2xx response is final. A Retry-After header on a
// 429 or 503 replaces the computed backoff, capped at MaxBackoff. The body
// and signature are computed once, so every attempt carries identical bytes
// and an identical X-SoroForge-Signature header.
func (s *HTTPWebhookSender) Deliver(ctx context.Context, wh config.Webhook, payload WebhookPayload) (int, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("marshal webhook payload: %w", err)
	}

	var signature string
	if wh.Secret != "" {
		mac := hmac.New(sha256.New, []byte(wh.Secret))
		mac.Write(raw)
		signature = "sha256=" + hex.EncodeToString(mac.Sum(nil))
	}

	p := s.policy
	start := p.Now()
	backoff := p.BaseBackoff
	attempts := 0
	for {
		attempts++
		retryAfter, retryable, err := s.attempt(ctx, wh.URL, raw, signature)
		if err == nil {
			return attempts, nil
		}
		if !retryable || attempts >= p.MaxAttempts || ctx.Err() != nil {
			return attempts, attemptsError(attempts, err)
		}

		wait := p.Jitter(min(backoff, p.MaxBackoff))
		if retryAfter > 0 {
			wait = min(retryAfter, p.MaxBackoff)
		}
		if p.Now().Sub(start)+wait >= p.MaxElapsed {
			return attempts, attemptsError(attempts, fmt.Errorf("%w (giving up: retry budget of %s exhausted)", err, p.MaxElapsed))
		}
		if serr := p.Sleep(ctx, wait); serr != nil {
			return attempts, attemptsError(attempts, err)
		}
		backoff *= 2
	}
}

func attemptsError(attempts int, err error) error {
	if attempts <= 1 {
		return err
	}
	return fmt.Errorf("%w (after %d attempts)", err, attempts)
}

// attempt makes one POST. It reports whether a failure is worth retrying and,
// when the receiver asked for one, the Retry-After delay.
func (s *HTTPWebhookSender) attempt(ctx context.Context, url string, raw []byte, signature string) (time.Duration, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return 0, false, fmt.Errorf("create webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "SoroForge-Webhook")
	if signature != "" {
		req.Header.Set("X-SoroForge-Signature", signature)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		// A cancelled or expired caller context is final; anything else at
		// the transport level (refused, reset, timeout) is transient.
		retryable := ctx.Err() == nil && !errors.Is(err, context.Canceled)
		return 0, retryable, fmt.Errorf("deliver webhook: %w", err)
	}
	defer resp.Body.Close()
	// Drain a bounded amount so the connection can be reused for the retry.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return 0, false, nil
	}

	statusErr := fmt.Errorf("webhook returned status %s", resp.Status)
	switch {
	case resp.StatusCode == http.StatusTooManyRequests, resp.StatusCode == http.StatusServiceUnavailable:
		return parseRetryAfter(resp.Header.Get("Retry-After"), time.Now()), true, statusErr
	case resp.StatusCode >= 500:
		return 0, true, statusErr
	default:
		return 0, false, statusErr
	}
}

// parseRetryAfter reads a Retry-After value in either delay-seconds or
// HTTP-date form. It returns 0 when the header is absent or unusable.
func parseRetryAfter(v string, now time.Time) time.Duration {
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs <= 0 {
			return 0
		}
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := t.Sub(now); d > 0 {
			return d
		}
	}
	return 0
}
