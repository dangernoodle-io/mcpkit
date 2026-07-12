package testkit

import (
	"context"
	"sync"
	"testing"
)

// TestRecordToolListChanged_BufferFull proves that recordToolListChanged
// handles a full buffer correctly by dropping signals in the default case
// (non-blocking send). This tests the drop branch at line ~98 of harness.go.
//
// The test constructs a Harness with the buffered channel, calls
// recordToolListChanged bufferSize+1 times in rapid succession (no draining),
// and asserts it neither blocks nor panics. The first bufferSize sends succeed
// (channel fills), and the (+1)th send hits the non-blocking default clause,
// proving the drop branch is reachable and safe.
func TestRecordToolListChanged_BufferFull(t *testing.T) {
	const bufferSize = 8

	// Construct a minimal Harness with the toolListChanged channel at the
	// same buffer size as the real constructor uses (line 62 of harness.go).
	h := &Harness{
		t:               t,
		mu:              sync.Mutex{},
		progress:        make(map[any][]ProgressEvent),
		toolListChanged: make(chan struct{}, bufferSize),
	}

	// Send bufferSize + 1 signals. The first bufferSize hit the
	// successful send case (case h.toolListChanged <- struct{}{}:).
	// The (+1)th hits the default: case, exercising the drop branch.
	//
	// If recordToolListChanged blocks on any send, this test will hang
	// (test timeout). If it panics, the panic surfaces immediately.
	for i := 0; i < bufferSize+1; i++ {
		h.recordToolListChanged(context.Background())
	}

	// Verify the channel is exactly full (bufferSize sends succeeded,
	// the (+1)th was dropped).
	if len(h.toolListChanged) != bufferSize {
		t.Fatalf("expected channel to contain %d signals (all successful sends before drop), got %d",
			bufferSize, len(h.toolListChanged))
	}
}
