package markdown

import (
	"github.com/yuin/goldmark/v2/ast"
	"github.com/yuin/goldmark/v2/parser"
	"github.com/yuin/goldmark/v2/text"
)

type paragraph struct{}

var _paragraph = paragraph{}

func (b paragraph) Trigger() []byte {
	return nil
}

func (b paragraph) Open(p ast.Node, r text.Reader, pc parser.Context) (ast.Node, parser.State) {
	_, segment := r.PeekLine()
	node := ast.NewParagraph()
	node.AppendSource(segment)
	r.Advance(segment.Len() - 1)
	return node, parser.NoChildren
}

func (b paragraph) Continue(node ast.Node, r text.Reader, pc parser.Context) parser.State {
	_, segment := r.PeekLine()

	// Dirty dirty little hack, dirty dirty source code..
	if p := node.Parent(); p != nil && p.Kind() == ast.KindListItem {
		trimmed := segment.TrimLeftSpace(r.Source())
		if trimmed.IsEmpty() {
			return parser.Close
		}
	}

	node.(*ast.Paragraph).AppendSource(segment)
	r.Advance(segment.Len() - 1)

	return parser.Continue | parser.NoChildren
}

func (b paragraph) Close(node ast.Node, r text.Reader, pc parser.Context) {
	p := node.Parent()
	if p == nil {
		// paragraph has been transformed
		return
	}

	lines := node.(*ast.Paragraph).Source()
	if len(lines) == 0 {
		p.RemoveChild(node)
		return
	}
	lines[len(lines)-1] = lines[len(lines)-1].TrimRightSpace(r.Source())
}

func (b paragraph) CanInterruptParagraph() bool {
	return false
}

func (b paragraph) CanAcceptIndentedLine() bool {
	return false
}
