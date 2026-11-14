package voice

import (
	"context"
	"testing"
	"time"

	"github.com/ayn2op/arikawa/v3/utils/handler"
	"github.com/ayn2op/arikawa/v3/voice/voicegateway"
)

func TestSessionSpeaking(t *testing.T) {
	server := newFakeVoiceServer(t)
	events := handler.New()
	mainSession := &fakeDiscord{events: events, server: server}
	session := NewSessionCustom(mainSession, events, fakeUserID)
	t.Cleanup(func() { session.Leave(context.Background()) })

	// expectSpeaking fails t unless the voice server is next told that the
	// newest connection speaks into the microphone.
	expectSpeaking := func(t *testing.T) {
		t.Helper()

		got, err := server.nextSpeaking(5 * time.Second)
		if err != nil {
			t.Fatal(err)
		}
		if want := (voicegateway.SpeakingEvent{Speaking: voicegateway.Microphone, SSRC: server.ssrc.Load()}); got != want {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	}

	t.Run("join", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := session.JoinChannelAndSpeak(ctx, 1, false, false); err != nil {
			t.Fatal(err)
		}
		expectSpeaking(t)
	})

	t.Run("voice server moved", func(t *testing.T) {
		mainSession.moveVoiceServer()
		expectSpeaking(t)
	})
}
