package arikawa_test

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/diamondburned/arikawa/v3/utils/handler"
)

func Example() {
	s := state.New("Bot " + os.Getenv("DISCORD_TOKEN"))
	s.AddIntents(gateway.IntentGuilds | gateway.IntentGuildMessages)
	handler.Add(s, func(m *gateway.MessageCreateEvent) {
		log.Printf("%s: %s", m.Author.Username, m.Content)
	})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if err := s.Connect(ctx); err != nil {
		log.Println("cannot open:", err)
	}
}
