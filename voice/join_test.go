package voice

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ayn2op/arikawa/v3/utils/handler"
	"github.com/ayn2op/arikawa/v3/voice/udp"
)

func TestSessionJoinChannel(t *testing.T) {
	t.Run("UDP dial fails", func(t *testing.T) {
		server := newFakeVoiceServer(t)
		events := handler.New()
		session := NewSessionCustom(&fakeDiscord{events: events, server: server}, events, fakeUserID)
		t.Cleanup(func() { session.Leave(context.Background()) })

		dialErr := errors.New("unreachable")
		session.SetUDPDialer(func(context.Context, string, uint32) (*udp.Connection, error) { return nil, dialErr })

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := session.JoinChannel(ctx, 1, false, false); !errors.Is(err, dialErr) {
			t.Fatalf("got %v, want %v", err, dialErr)
		}
	})
}
