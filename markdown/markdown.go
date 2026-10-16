package markdown

import (
	"sync"

	"github.com/ayn2op/arikawa/v3/discord"
	"github.com/ayn2op/arikawa/v3/state/store"
	"github.com/yuin/goldmark/v2/ast"
	"github.com/yuin/goldmark/v2/parser"
)

var (
	messageCtx = parser.NewContextKey()
	sessionCtx = parser.NewContextKey()
	emojiCtx   = parser.NewContextKey()

	// markdownParser is safe for concurrent use, since each parse keeps its state in its own context.
	markdownParser = sync.OnceValue(func() parser.Parser {
		return parser.New(
			// Discord supports only some of CommonMark, so its parsers replace the default ones.
			parser.WithDefaultParsers(false),
			parser.WithBlockParsers(blockParsers()...),
			parser.WithInlineParsers(inlineParsers(&emoji{})...),
			parser.WithExtensions(linkify),
		)
	})
)

func parse(content []byte, opts ...parser.ParseOption) ast.Node {
	return markdownParser().Parse(content, opts...)
}

// ParseWithMessage parses the given byte slice with the Discord state and the
// Message as source for the ast nodes.
func ParseWithMessage(b []byte, s store.Cabinet, m *discord.Message) ast.Node {
	// Context to pass down messages:
	ctx := parser.NewContext()
	ctx.Set(messageCtx, m)
	ctx.Set(sessionCtx, &s)

	return parse(b, parser.WithContext(ctx))
}

// Parse parses the given byte slice with extra options.
func Parse(content []byte, opts ...parser.ParseOption) ast.Node {
	return parse(content, opts...)
}

func getMessage(pc parser.Context) *discord.Message {
	if v := pc.Get(messageCtx); v != nil {
		return v.(*discord.Message)
	}
	return nil
}
func getSession(pc parser.Context) *store.Cabinet {
	if v := pc.Get(sessionCtx); v != nil {
		return v.(*store.Cabinet)
	}
	return nil
}
