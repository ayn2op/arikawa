package markdown

import (
	"bytes"

	"github.com/yuin/goldmark/v2/ast"
	"github.com/yuin/goldmark/v2/parser"
	"github.com/yuin/goldmark/v2/text"
)

// headingParser is goldmark's ATX heading parser without the levels past 3, which Discord does not support.
type headingParser struct {
	parser.BlockParser
}

func newHeadingParser() parser.BlockParser {
	return headingParser{parser.NewATXHeadingParser()}
}

func (p headingParser) Open(parent ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	line, _ := reader.PeekLine()
	if pos := pc.BlockOffset(); pos >= 0 && bytes.HasPrefix(line[pos:], []byte("####")) {
		return nil, parser.NoChildren
	}
	return p.BlockParser.Open(parent, reader, pc)
}
