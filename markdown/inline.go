package markdown

import (
	"slices"

	"github.com/yuin/goldmark/v2/ast"
	"github.com/yuin/goldmark/v2/parser"
	"github.com/yuin/goldmark/v2/text"
	"github.com/yuin/goldmark/v2/util"
)

type Inline struct {
	ast.BaseInline
	Attr Attribute
}

var KindInline = ast.NewNodeKind("Inline")

// Kind implements Node.Kind.
func (e *Inline) Kind() ast.NodeKind {
	return KindInline
}

// Dump implements Node.Dump
func (e *Inline) Dump(source []byte) *ast.NodeDump {
	return ast.NewNodeDump(e, map[string]any{
		"Attributes": e.Attr.String(),
	})
}

func newInline(attr Attribute) *Inline {
	n := &Inline{Attr: attr}
	n.Init(n)
	return n
}

var inlineTriggers = []byte{'*', '_', '|', '~'}

type inlineDelimiterProcessor struct {
	char byte
	attr Attribute
}

func (p *inlineDelimiterProcessor) IsDelimiter(b byte) bool {
	if slices.Contains(inlineTriggers, b) {
		p.char = b
		return true
	}

	return false
}

func (p *inlineDelimiterProcessor) CanOpenCloser(opener, closer *parser.Delimiter) bool {
	var can = opener.Char == closer.Char
	if !can {
		return false
	}

	switch consumes := opener.Length; {
	case p.char == '_' && consumes == 2: // __
		p.attr = AttrUnderline
	case p.char == '_' && consumes == 1: // _
		fallthrough
	case p.char == '*' && consumes == 1: // *
		p.attr = AttrItalics
	case p.char == '*' && consumes == 2: // **
		p.attr = AttrBold
	case p.char == '*' && consumes == 3: // ***
		p.attr = AttrBold | AttrItalics
	case p.char == '|' && consumes == 2: // ||
		p.attr = AttrSpoiler
	case p.char == '~' && consumes == 2: // ~~
		p.attr = AttrStrikethrough
	default:
		return false
	}

	return true
}

func (p *inlineDelimiterProcessor) OnMatch(consumes int) ast.Node {
	return newInline(p.attr)
}

type inline struct{}

func (inline) Trigger() []byte {
	return inlineTriggers
}

func (inline) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	before := block.PrecedingCharacter()
	line, segment := block.PeekLine()

	processor := inlineDelimiterProcessor{}

	// parser.ParseDelimiter would match simple runs like CommonMark emphasis, without asking processor.
	node := scanDelimiter(line, before, 1, &processor)
	if node == nil {
		return nil
	}
	node.Value = text.NewSingleLineValueFromSegment(segment.WithStop(segment.Start+node.OriginalLength), block.Decoder())

	block.Advance(node.OriginalLength)
	pc.PushDelimiter(node)

	return node
}

type inlineCodeSpan struct{}

var _ parser.InlineParser = inlineCodeSpan{}

func (p inlineCodeSpan) Trigger() []byte {
	return []byte{'`'}
}

func (p inlineCodeSpan) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	line, startSegment := block.PeekLine()
	opener := 0
	for ; opener < len(line) && line[opener] == '`'; opener++ {
	}
	block.Advance(opener)
	l, pos := block.Position()
	node := newInline(AttrMonospace)
	for {
		line, segment := block.PeekLine()
		if line == nil {
			block.SetPosition(l, pos)
			return ast.NewText(text.NewSingleLineValueFromSegment(startSegment.WithStop(startSegment.Start+opener), block.Decoder()))
		}
		for i := 0; i < len(line); i++ {
			c := line[i]
			if c == '`' {
				oldi := i
				for ; i < len(line) && line[i] == '`'; i++ {
				}
				closure := i - oldi
				if closure == opener && (i >= len(line) || line[i] != '`') {
					segment = segment.WithStop(segment.Start + i - closure)
					if !segment.IsEmpty() {
						node.AppendChild(ast.NewText(text.NewSingleLineValueFromSegment(segment, text.IdentityDecoder)))
					}
					block.Advance(i)
					goto end
				}
			}
		}
		if !util.IsBlank(line) {
			node.AppendChild(ast.NewText(text.NewSingleLineValueFromSegment(segment, text.IdentityDecoder)))
		}
		block.AdvanceLine()
	}
end:
	return node
}

// scanDelimiter returns the run of delimiter characters that line starts with, if it is at least minimum long.
func scanDelimiter(line []byte, before rune, minimum int, processor parser.DelimiterProcessor) *parser.Delimiter {
	if len(line) == 0 || !processor.IsDelimiter(line[0]) {
		return nil
	}
	c := line[0]
	j := 0
	for j < len(line) && line[j] == c {
		j++
	}
	if j < minimum {
		return nil
	}
	after := rune(' ')
	if j != len(line) {
		after = util.ToRune(line, j)
	}

	isLeft := parser.IsLeftFlankingDelimiterRun(before, after)
	isRight := parser.IsRightFlankingDelimiterRun(before, after)
	canOpen, canClose := isLeft, isRight
	if c == '_' {
		canOpen = isLeft && (!isRight || util.IsPunctRune(before))
		canClose = isRight && (!isLeft || util.IsPunctRune(after))
	}
	return parser.NewDelimiter(canOpen, canClose, j, c, processor)
}
