package deploy_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/soroworks/soroforge/internal/config"
	"github.com/soroworks/soroforge/internal/deploy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sleepRecorder replaces real waits so retry tests run instantly and can
// assert on the backoff schedule.
type sleepRecorder struct {
	mu    sync.Mutex
	waits []time.Duration
}

func (r *sleepRecorder) Sleep(_ context.Context, d time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.waits = append(r.waits, d)
	return nil
}

func (r *sleepRecorder) Waits() []time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]time.Duration(nil), r.waits...)
}

// failingServer responds with failStatus for the first failures requests and
// 200 afterwards. headers are set on failing responses.
func failingServer(t *testing.T, failures int32, failStatus int, headers map[string]string) (*httptest.Server, *atomic.Int32, *[]string) {
	t.Helper()
	var calls atomic.Int32
	var mu sync.Mutex
	var sigs []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		mu.Lock()
		sigs = append(sigs, r.Header.Get("X-SoroForge-Signature"))
		mu.Unlock()
		if calls.Add(1) <= failures {
			for k, v := range headers {
				w.Header().Set(k, v)
			}
			w.WriteHeader(failStatus)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls, &sigs
}

func noJitter(d time.Duration) time.Duration { return d }

func newTestSender(rec *sleepRecorder, mut func(*deploy.WebhookRetryPolicy)) *deploy.HTTPWebhookSender {
	p := deploy.WebhookRetryPolicy{
		BaseBackoff: 100 * time.Millisecond,
		MaxBackoff:  time.Second,
		MaxElapsed:  time.Minute,
		Sleep:       rec.Sleep,
		Jitter:      noJitter,
	}
	if mut != nil {
		mut(&p)
	}
	return deploy.NewHTTPWebhookSenderWithPolicy(nil, p)
}

func TestWebhookRetriesTransientFailures(t *testing.T) {
	for _, status := range []int{http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusTooManyRequests} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			srv, calls, sigs := failingServer(t, 2, status, nil)
			rec := &sleepRecorder{}
			sender := newTestSender(rec, nil)

			attempts, err := sender.Deliver(context.Background(),
				config.Webhook{URL: srv.URL, Secret: "s3cret"}, deploy.WebhookPayload{Alias: "counter"})
			require.NoError(t, err)
			assert.Equal(t, 3, attempts)
			assert.Equal(t, int32(3), calls.Load())
			assert.Equal(t, []time.Duration{100 * time.Millisecond, 200 * time.Millisecond}, rec.Waits(),
				"backoff doubles between attempts")

			require.Len(t, *sigs, 3)
			assert.NotEmpty(t, (*sigs)[0])
			assert.Equal(t, (*sigs)[0], (*sigs)[1], "signature is identical on every retry")
			assert.Equal(t, (*sigs)[0], (*sigs)[2])
		})
	}
}

func TestWebhookDoesNotRetryClientErrors(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusNotFound, http.StatusUnprocessableEntity} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			srv, calls, _ := failingServer(t, 10, status, nil)
			rec := &sleepRecorder{}

			attempts, err := newTestSender(rec, nil).Deliver(context.Background(),
				config.Webhook{URL: srv.URL}, deploy.WebhookPayload{})
			require.Error(t, err)
			assert.Equal(t, 1, attempts)
			assert.Equal(t, int32(1), calls.Load())
			assert.Empty(t, rec.Waits())
		})
	}
}

func TestWebhookGivesUpAfterMaxAttempts(t *testing.T) {
	srv, calls, _ := failingServer(t, 100, http.StatusBadGateway, nil)
	rec := &sleepRecorder{}

	attempts, err := newTestSender(rec, func(p *deploy.WebhookRetryPolicy) { p.MaxAttempts = 3 }).
		Deliver(context.Background(), config.Webhook{URL: srv.URL}, deploy.WebhookPayload{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "502")
	assert.Contains(t, err.Error(), "after 3 attempts")
	assert.Equal(t, 3, attempts)
	assert.Equal(t, int32(3), calls.Load())
}

func TestWebhookBackoffIsCapped(t *testing.T) {
	srv, _, _ := failingServer(t, 100, http.StatusInternalServerError, nil)
	rec := &sleepRecorder{}

	_, err := newTestSender(rec, func(p *deploy.WebhookRetryPolicy) {
		p.MaxAttempts = 6
		p.BaseBackoff = 300 * time.Millisecond
		p.MaxBackoff = time.Second
	}).Deliver(context.Background(), config.Webhook{URL: srv.URL}, deploy.WebhookPayload{})
	require.Error(t, err)
	assert.Equal(t, []time.Duration{
		300 * time.Millisecond, 600 * time.Millisecond, time.Second, time.Second, time.Second,
	}, rec.Waits())
}

func TestWebhookHonoursRetryAfter(t *testing.T) {
	t.Run("seconds on 429", func(t *testing.T) {
		srv, _, _ := failingServer(t, 1, http.StatusTooManyRequests, map[string]string{"Retry-After": "3"})
		rec := &sleepRecorder{}
		attempts, err := newTestSender(rec, func(p *deploy.WebhookRetryPolicy) { p.MaxBackoff = 10 * time.Second }).
			Deliver(context.Background(), config.Webhook{URL: srv.URL}, deploy.WebhookPayload{})
		require.NoError(t, err)
		assert.Equal(t, 2, attempts)
		assert.Equal(t, []time.Duration{3 * time.Second}, rec.Waits())
	})

	t.Run("capped on 503", func(t *testing.T) {
		srv, _, _ := failingServer(t, 1, http.StatusServiceUnavailable, map[string]string{"Retry-After": "3600"})
		rec := &sleepRecorder{}
		_, err := newTestSender(rec, func(p *deploy.WebhookRetryPolicy) { p.MaxBackoff = 2 * time.Second }).
			Deliver(context.Background(), config.Webhook{URL: srv.URL}, deploy.WebhookPayload{})
		require.NoError(t, err)
		assert.Equal(t, []time.Duration{2 * time.Second}, rec.Waits())
	})

	t.Run("HTTP-date", func(t *testing.T) {
		when := time.Now().Add(5 * time.Second).UTC().Format(http.TimeFormat)
		srv, _, _ := failingServer(t, 1, http.StatusServiceUnavailable, map[string]string{"Retry-After": when})
		rec := &sleepRecorder{}
		_, err := newTestSender(rec, func(p *deploy.WebhookRetryPolicy) { p.MaxBackoff = 10 * time.Second }).
			Deliver(context.Background(), config.Webhook{URL: srv.URL}, deploy.WebhookPayload{})
		require.NoError(t, err)
		require.Len(t, rec.Waits(), 1)
		assert.InDelta(t, float64(5*time.Second), float64(rec.Waits()[0]), float64(1500*time.Millisecond))
	})

	t.Run("ignored on 500", func(t *testing.T) {
		srv, _, _ := failingServer(t, 1, http.StatusInternalServerError, map[string]string{"Retry-After": "7"})
		rec := &sleepRecorder{}
		_, err := newTestSender(rec, nil).
			Deliver(context.Background(), config.Webhook{URL: srv.URL}, deploy.WebhookPayload{})
		require.NoError(t, err)
		assert.Equal(t, []time.Duration{100 * time.Millisecond}, rec.Waits())
	})
}

func TestWebhookTotalTimeIsBounded(t *testing.T) {
	srv, calls, _ := failingServer(t, 100, http.StatusBadGateway, nil)

	// A fake clock that advances by each recorded wait, so the elapsed
	// budget is exercised without sleeping.
	var mu sync.Mutex
	now := time.Unix(0, 0)
	rec := &sleepRecorder{}
	sender := newTestSender(rec, func(p *deploy.WebhookRetryPolicy) {
		p.MaxAttempts = 100
		p.BaseBackoff = time.Second
		p.MaxBackoff = 4 * time.Second
		p.MaxElapsed = 10 * time.Second
		p.Now = func() time.Time { mu.Lock(); defer mu.Unlock(); return now }
		p.Sleep = func(ctx context.Context, d time.Duration) error {
			mu.Lock()
			now = now.Add(d)
			mu.Unlock()
			return rec.Sleep(ctx, d)
		}
	})

	attempts, err := sender.Deliver(context.Background(), config.Webhook{URL: srv.URL}, deploy.WebhookPayload{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "retry budget")
	// Waits 1s + 2s + 4s = 7s; the next 4s wait would cross 10s.
	assert.Equal(t, 4, attempts)
	assert.Equal(t, int32(4), calls.Load())
	var total time.Duration
	for _, w := range rec.Waits() {
		total += w
	}
	assert.Less(t, total, 10*time.Second)
}

func TestWebhookRetriesNetworkErrors(t *testing.T) {
	// A server that is closed before use refuses connections.
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	rec := &sleepRecorder{}
	attempts, err := newTestSender(rec, func(p *deploy.WebhookRetryPolicy) { p.MaxAttempts = 3 }).
		Deliver(context.Background(), config.Webhook{URL: url}, deploy.WebhookPayload{})
	require.Error(t, err)
	assert.Equal(t, 3, attempts)
	assert.Len(t, rec.Waits(), 2)
}

func TestWebhookStopsWhenContextCancelled(t *testing.T) {
	srv, calls, _ := failingServer(t, 100, http.StatusBadGateway, nil)
	ctx, cancel := context.WithCancel(context.Background())

	sender := newTestSender(&sleepRecorder{}, func(p *deploy.WebhookRetryPolicy) {
		p.Sleep = func(ctx context.Context, d time.Duration) error {
			cancel()
			return ctx.Err()
		}
	})
	attempts, err := sender.Deliver(ctx, config.Webhook{URL: srv.URL}, deploy.WebhookPayload{})
	require.Error(t, err)
	assert.Equal(t, 1, attempts)
	assert.Equal(t, int32(1), calls.Load())
}

func TestWebhookDefaultJitterStaysInRange(t *testing.T) {
	// The default jitter must never wait longer than the computed backoff
	// nor less than half of it.
	srv, _, _ := failingServer(t, 100, http.StatusBadGateway, nil)
	rec := &sleepRecorder{}
	sender := deploy.NewHTTPWebhookSenderWithPolicy(nil, deploy.WebhookRetryPolicy{
		MaxAttempts: 5,
		BaseBackoff: 100 * time.Millisecond,
		Sleep:       rec.Sleep,
	})
	_, err := sender.Deliver(context.Background(), config.Webhook{URL: srv.URL}, deploy.WebhookPayload{})
	require.Error(t, err)
	base := 100 * time.Millisecond
	for i, w := range rec.Waits() {
		assert.GreaterOrEqual(t, w, base/2, "wait %d", i)
		assert.LessOrEqual(t, w, base, "wait %d", i)
		base *= 2
	}
}
