package deploy

import (
	"testing"

	"github.com/soroworks/soroforge/internal/stellar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckSendStatus(t *testing.T) {
	const hash = "a1b2c3"

	t.Run("PENDING keeps waiting", func(t *testing.T) {
		assert.NoError(t, checkSendStatus(stellar.SendResult{Status: stellar.SendStatusPending, Hash: hash}))
	})

	t.Run("DUPLICATE is treated as submitted", func(t *testing.T) {
		assert.NoError(t, checkSendStatus(stellar.SendResult{Status: stellar.SendStatusDuplicate, Hash: hash}))
	})

	t.Run("TRY_AGAIN_LATER says it was not queued and names the hash", func(t *testing.T) {
		err := checkSendStatus(stellar.SendResult{Status: stellar.SendStatusTryAgainLater, Hash: hash})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "was not queued")
		assert.Contains(t, err.Error(), hash)
	})

	t.Run("ERROR with result XDR includes it", func(t *testing.T) {
		err := checkSendStatus(stellar.SendResult{
			Status:         stellar.SendStatusError,
			Hash:           hash,
			ErrorResultXDR: "AAAAAAAAAGT////7AAAAAA==",
		})
		assert.EqualError(t, err, "transaction rejected by the network (result XDR: AAAAAAAAAGT////7AAAAAA==)")
	})

	t.Run("ERROR without result XDR", func(t *testing.T) {
		err := checkSendStatus(stellar.SendResult{Status: stellar.SendStatusError, Hash: hash})
		assert.EqualError(t, err, "transaction rejected by the network")
	})

	t.Run("unknown status is named", func(t *testing.T) {
		err := checkSendStatus(stellar.SendResult{Status: "SOMETHING_NEW", Hash: hash})
		require.Error(t, err)
		assert.Contains(t, err.Error(), `"SOMETHING_NEW"`)
	})
}
