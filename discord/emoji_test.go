package discord

import "testing"

func TestAPIEmojiPathString(t *testing.T) {
	for _, test := range []struct {
		emoji APIEmoji
		want  string
	}{
		{"👍", "%F0%9F%91%8D"},
		{NewAPIEmoji(1454025856958857287, "crying_skull"), "crying_skull%3A1454025856958857287"},
	} {
		if got := test.emoji.PathString(); got != test.want {
			t.Errorf("PathString(%q) = %q, want %q", test.emoji, got, test.want)
		}
	}
}
