package deploy

import (
	"errors"
	"testing"

	"github.com/soroworks/soroforge/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// retryable is a typed error, so the test covers errors.As as well as
// errors.Is: Unwrap must keep passing both through the stage wrapper.
type retryable struct{ msg string }

func (e *retryable) Error() string { return e.msg }

// TestStagedErrorUnwrap pins the contract of stagedError.Unwrap: recording a
// stage and a transaction hash must not hide the original failure from callers
// that match on it.
func TestStagedErrorUnwrap(t *testing.T) {
	sentinel := errors.New("rpc unavailable")
	typed := &retryable{msg: "submission timed out"}

	tests := []struct {
		name      string
		wrapped   error
		wantTyped *retryable
	}{
		{
			name:    "sentinel stays reachable with errors.Is",
			wrapped: sentinel,
		},
		{
			name:      "typed error stays reachable with errors.As",
			wrapped:   typed,
			wantTyped: typed,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := staged(store.AttemptSubmit, "abc123", tc.wrapped)
			require.Error(t, err)

			var stageErr *stagedError
			require.ErrorAs(t, err, &stageErr)
			assert.Equal(t, store.AttemptSubmit, stageErr.stage)
			assert.Equal(t, "abc123", stageErr.txHash)

			// The stage metadata is additive: the original error comes back
			// unchanged, with the same message.
			unwrapped := errors.Unwrap(err)
			require.Error(t, unwrapped)
			assert.Same(t, tc.wrapped, unwrapped)
			assert.Equal(t, tc.wrapped.Error(), err.Error())

			assert.ErrorIs(t, err, tc.wrapped, "the chain must reach the wrapped error")
			if tc.wantTyped != nil {
				var got *retryable
				require.ErrorAs(t, err, &got)
				assert.Same(t, tc.wantTyped, got)
			}
		})
	}

	// Error path: a successful run has nothing to stage, so staged passes nil
	// through rather than wrapping it — callers test the result directly and
	// never hold an error that unwraps to nil.
	assert.NoError(t, staged(store.AttemptConfirm, "", nil))
}
