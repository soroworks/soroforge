package deploy

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHTTPWebhookSender(t *testing.T) {
	tests := []struct {
		name         string
		client       *http.Client
		wantTimeout  time.Duration
		wantProvided bool
	}{
		{
			name:        "nil client gets timeout",
			wantTimeout: DefaultWebhookTimeout,
		},
		{
			name:         "provided client used as-is",
			client:       &http.Client{},
			wantProvided: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sender := NewHTTPWebhookSender(tc.client)
			require.NotNil(t, sender)
			if tc.wantProvided {
				assert.Same(t, tc.client, sender.client)
			} else {
				require.NotNil(t, sender.client)
				assert.Equal(t, tc.wantTimeout, sender.client.Timeout)
			}
		})
	}
}
