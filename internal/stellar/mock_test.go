package stellar

import (
	"context"
	"testing"

	"github.com/stellar/go-stellar-sdk/txnbuild"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockClientSendCount(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		setup     func(*MockClient)
		sendCount int
	}{
		{
			name:      "happy path - no sends",
			setup:     func(c *MockClient) {},
			sendCount: 0,
		},
		{
			name: "happy path - single send",
			setup: func(c *MockClient) {
				_, _ = c.Send(ctx, "envelope-1")
			},
			sendCount: 1,
		},
		{
			name: "happy path - multiple sends",
			setup: func(c *MockClient) {
				_, _ = c.Send(ctx, "envelope-1")
				_, _ = c.Send(ctx, "envelope-2")
				_, _ = c.Send(ctx, "envelope-3")
			},
			sendCount: 3,
		},
		{
			name: "error path - send fails but still counted",
			setup: func(c *MockClient) {
				c.SendFunc = func(_ context.Context, _ string) (SendResult, error) {
					return SendResult{}, assert.AnError
				}
				_, _ = c.Send(ctx, "envelope-1")
			},
			sendCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewMockClient()
			tt.setup(c)

			count := c.SendCount()
			assert.Equal(t, tt.sendCount, count, "SendCount should match expected count")
		})
	}
}

func TestMockSignerSignCount(t *testing.T) {
	tx, err := NewTestTransaction()
	require.NoError(t, err)

	tests := []struct {
		name      string
		setup     func(*MockSigner)
		signCount int
	}{
		{
			name:      "happy path - no signs",
			setup:     func(s *MockSigner) {},
			signCount: 0,
		},
		{
			name: "happy path - single sign",
			setup: func(s *MockSigner) {
				_, _ = s.Sign("Test SDF Network ; September 2015", tx)
			},
			signCount: 1,
		},
		{
			name: "happy path - multiple signs",
			setup: func(s *MockSigner) {
				_, _ = s.Sign("Test SDF Network ; September 2015", tx)
				_, _ = s.Sign("Test SDF Network ; September 2015", tx)
				_, _ = s.Sign("Test SDF Network ; September 2015", tx)
			},
			signCount: 3,
		},
		{
			name: "error path - sign fails but still counted",
			setup: func(s *MockSigner) {
				s.SignErr = assert.AnError
				_, _ = s.Sign("Test SDF Network ; September 2015", tx)
			},
			signCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewMockSigner()
			tt.setup(s)

			count := s.SignCount()
			assert.Equal(t, tt.signCount, count, "SignCount should match expected count")
		})
	}
}

func NewTestTransaction() (*txnbuild.Transaction, error) {
	return txnbuild.NewTransaction(txnbuild.TransactionParams{
		SourceAccount:        &txnbuild.SimpleAccount{AccountID: "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHF", Sequence: 1},
		IncrementSequenceNum: true,
		Operations:           []txnbuild.Operation{&txnbuild.BumpSequence{BumpTo: 2}},
		BaseFee:              txnbuild.MinBaseFee,
		Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewInfiniteTimeout()},
	})
}
