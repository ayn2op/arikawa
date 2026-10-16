package markdown

import (
	"bytes"
	"regexp"

	"github.com/yuin/goldmark/v2/ast"
	"github.com/yuin/goldmark/v2/parser"
	"github.com/yuin/goldmark/v2/text"
)

const (
	InlineEmojiSize = 22
	LargeEmojiSize  = 48
)

func EmojiURL(emojiID string, animated bool) string {
	const EmojiBaseURL = "https://cdn.discordapp.com/emojis/"

	if animated {
		return EmojiBaseURL + emojiID + ".gif?v=1"
	}

	return EmojiBaseURL + emojiID + ".png?v=1"
}

type Emoji struct {
	ast.BaseInline

	ID   string
	Name string
	GIF  bool

	Large bool // TODO
}

var KindEmoji = ast.NewNodeKind("Emoji")

// Kind implements Node.Kind.
func (e *Emoji) Kind() ast.NodeKind {
	return KindEmoji
}

// Dump implements Node.Dump
func (e *Emoji) Dump(source []byte) *ast.NodeDump {
	return ast.NewNodeDump(e, map[string]any{
		"ID":    e.ID,
		"Name":  e.Name,
		"GIF":   e.GIF,
		"Large": e.Large,
	})
}

func (e Emoji) EmojiURL() string {
	return EmojiURL(e.ID, e.GIF)
}

type emoji struct{}

// emojiSize is whether the emojis of a parse are large, kept in its context.
type emojiSize struct {
	searched bool // if a small/large check was done
	large    bool
}

var emojiRegex = regexp.MustCompile(`<(a?):(.+?):(\d+)>`)

func (emoji) Trigger() []byte {
	// return []byte("http")
	return []byte{'<'}
}

func (*emoji) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	state := pc.ComputeIfAbsent(emojiCtx, func() any { return &emojiSize{} }).(*emojiSize)

	match := matchInline(block, '<', '>')
	if match == nil {
		return nil
	}

	var matches = emojiRegex.FindSubmatch(match)
	if len(matches) != 4 {
		return nil
	}

	var emoji = &Emoji{
		GIF:   string(matches[1]) == "a",
		Name:  string(matches[2]),
		ID:    string(matches[3]),
		Large: state.large,
	}
	emoji.Init(emoji)

	// Check if emojis should be small:
	if !state.searched {
		state.searched = true

		// Get the entire text:
		source := block.Source()

		// Try and delete all emoji matches:
		source = emojiRegex.ReplaceAll(source, nil)

		// Trim spaces:
		source = bytes.TrimSpace(source)

		// Check if there are letters:
		if len(source) == 0 {
			// No, make our emojis big:
			state.large = true
			emoji.Large = true
		}
	}

	return emoji
}
