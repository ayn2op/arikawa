package markdown

import (
	"github.com/yuin/goldmark/v2/parser"
	"github.com/yuin/goldmark/v2/text"
	"github.com/yuin/goldmark/v2/util"
)

func blockParsers() []util.PrioritizedValue[parser.BlockParser] {
	return []util.PrioritizedValue[parser.BlockParser]{
		util.Prioritized[parser.BlockParser](defaultFencedCodeBlockParser, 10),
		util.Prioritized(newHeadingParser(), 100),
		util.Prioritized(newListParser(), 300),
		util.Prioritized(parser.NewListItemParser(), 400),
		util.Prioritized[parser.BlockParser](blockquote{}, 500),
		util.Prioritized[parser.BlockParser](paragraph{}, 1000),
	}
}

func inlineParsers(emojiParser *emoji) []util.PrioritizedValue[parser.InlineParser] {
	return []util.PrioritizedValue[parser.InlineParser]{
		util.Prioritized[parser.InlineParser](emojiParser, 200),
		util.Prioritized[parser.InlineParser](inlineCodeSpan{}, 300),
		util.Prioritized[parser.InlineParser](inline{}, 350),
		util.Prioritized[parser.InlineParser](mention{}, 400),
		util.Prioritized[parser.InlineParser](autolink{}, 500),
		util.Prioritized(parser.NewLinkParser(), 600),
	}
}

// matchInline function to parse a pair of bytes (chars)
func matchInline(r text.Reader, open, close byte) []byte {
	line, _ := r.PeekLine()

	start := 0
	for ; start < len(line) && line[start] != open; start++ {
	}

	stop := start
	for ; stop < len(line) && line[stop] != close; stop++ {
	}

	// This would be true if there's no closure.
	if stop >= len(line) || line[stop] != close {
		return nil
	}

	stop++ // add the '>'

	// Advance total distance:
	r.Advance(stop)

	return line[start:stop]
}
