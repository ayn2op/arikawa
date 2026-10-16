package markdown

import (
	"regexp"

	"github.com/yuin/goldmark/v2/ast"
	"github.com/yuin/goldmark/v2/extension"
	"github.com/yuin/goldmark/v2/parser"
	"github.com/yuin/goldmark/v2/text"
)

// neverRegexp matches nothing.
var neverRegexp = regexp.MustCompile(`\A\z.`)

// linkify links bare http and https URLs, as Discord does, but not www. hosts or emails.
var linkify = extension.NewLinkifyParser(
	extension.WithAllowedProtocols([]string{"http:", "https:"}),
	extension.WithWWWRegexp(neverRegexp),
	extension.WithEmailRegexp(neverRegexp),
)

var autolinkRegex = regexp.MustCompile(`^https?:\/\S+(?:\.|:)[^>\s]+`)

// autolink parses <https://...>, which Discord links without an embed.
type autolink struct{}

func (autolink) Trigger() []byte {
	return []byte{'<'}
}

func (autolink) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	line, segment := block.PeekLine()
	m := autolinkRegex.FindIndex(line[1:])
	if m == nil || 1+m[1] >= len(line) || line[1+m[1]] != '>' {
		return nil
	}

	// The URL is kept as written, without decoding escapes or entities.
	url := text.NewSingleLineValueFromSegment(text.NewSegment(segment.Start+1, segment.Start+1+m[1]), text.IdentityDecoder)
	block.Advance(m[1] + 2)
	return ast.NewAutoLink(url, url, ast.WithAutoLinkText(url))
}
