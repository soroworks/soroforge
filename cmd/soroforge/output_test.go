package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDescribeLedgers(t *testing.T) {
	// TTLs are counted in ledgers; the output translates them to wall-clock
	// time at Stellar's ~5-second close time so the number means something.
	tests := map[uint32]string{
		0:      "about 0 seconds",
		24:     "about 2 minutes",
		1200:   "about 100 minutes",
		17280:  "about 24 hours",
		200000: "about 12 days",
	}
	for ledgers, want := range tests {
		assert.Equal(t, want, describeLedgers(ledgers), "ledgers: %d", ledgers)
	}
}
