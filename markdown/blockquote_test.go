package markdown

import (
	"bytes"
	"testing"

	"github.com/yuin/goldmark/v2/parser"
	"github.com/yuin/goldmark/v2/renderer/html"
	"github.com/yuin/goldmark/v2/util"
)

const _blockquote = `lol message
test
asdasd
dadadasd
asdsad
> be me
> discord markdown **bad**
> suffer
discord sucks.`
const _blockquoteHTML = `<p>lol message
test
asdasd
dadadasd
asdsad</p>
<blockquote>
<p>be me
</p>
<p>discord markdown <strong>bad</strong>
</p>
<p>suffer
</p>
</blockquote>
<p>discord sucks.</p>
`

func TestBlockquote(t *testing.T) {
	// Make a fenced only parser:
	p := parser.New(
		parser.WithDefaultParsers(false),
		parser.WithBlockParsers(
			util.Prioritized[parser.BlockParser](blockquote{}, 700),
			util.Prioritized(parser.NewParagraphParser(), 500),
		),
		parser.WithInlineParsers(
			util.Prioritized(parser.NewEmphasisParser(), 200),
		),
	)

	// Results
	var buf bytes.Buffer

	// Test inline:
	source := []byte(_blockquote)
	if err := html.New().Render(&buf, source, p.Parse(source)); err != nil {
		t.Fatal("Failed to parse fenced inline:", err)
	}

	strcmp(t, "blockquote", buf.String(), _blockquoteHTML)
}
