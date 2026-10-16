package markdown

import "github.com/yuin/goldmark/v2/parser"

// listParser is goldmark's list parser without + bullets, which Discord does not support.
type listParser struct {
	parser.BlockParser
}

func newListParser() parser.BlockParser {
	return listParser{parser.NewListParser()}
}

func (listParser) Trigger() []byte {
	return []byte{'-', '*', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9'}
}
