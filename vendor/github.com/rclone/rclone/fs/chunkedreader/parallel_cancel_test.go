package chunkedreader

import (
	"context"
	"errors"
	"github.com/rclone/rclone/lib/multipart"
	"testing"
	"time"
)

func TestStreamReadStopsOnCancellation(t *testing.T) {
	for _, duringRead := range []bool{false, true} {
		t.Run(map[bool]string{false: "already_cancelled", true: "cancel_while_waiting"}[duringRead], func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			rw := multipart.NewRW()
			defer rw.Close()
			s := &stream{cr: &parallel{}, ctx: ctx, rw: rw, size: 1024}
			if !duringRead {
				cancel()
			}
			done := make(chan error, 1)
			go func() { _, err := s.read(make([]byte, 1024)); done <- err }()
			if duringRead {
				time.Sleep(10 * time.Millisecond)
				cancel()
			}
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("got %v, want cancellation", err)
				}
			case <-time.After(time.Second):
				t.Fatal("cancelled stream is stuck in read loop")
			}
		})
	}
}
