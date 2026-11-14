package udp

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestDialConnection(t *testing.T) {
	t.Run("IP discovery unanswered", func(t *testing.T) {
		server, err := net.ListenPacket("udp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { server.Close() })

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		done := make(chan error, 1)
		go func() {
			_, err := DialConnection(ctx, server.LocalAddr().String(), 1)
			done <- err
		}()

		select {
		case err := <-done:
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("got %v, want %v", err, context.DeadlineExceeded)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("still waiting for IP discovery after the context expired")
		}
	})
}
