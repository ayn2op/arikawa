package markdown

import (
	"testing"
	"time"

	"github.com/yuin/goldmark/v2/ast"
)

func TestTimestampParse(t *testing.T) {
	for _, test := range []struct {
		name    string
		content string
		style   byte
	}{
		{"default style", "<t:1794337200>", 'f'},
		{"styled", "starts <t:1794337200:F>.", 'F'},
		{"unknown style", "<t:1794337200:x>", 0},
		{"not a number", "<t:soon>", 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			var style byte
			_ = ast.Walk(Parse([]byte(test.content)), func(node ast.Node, entering bool) (ast.WalkStatus, error) {
				if timestamp, ok := node.(*Timestamp); ok {
					style = timestamp.Style
				}
				return ast.WalkContinue, nil
			})
			if style != test.style {
				t.Fatalf("got style %q, want %q", style, test.style)
			}
		})
	}
}

func TestTimestampFormat(t *testing.T) {
	now := time.Unix(1794337200, 0)
	for _, test := range []struct {
		name  string
		time  time.Time
		style byte
		want  string
	}{
		{"short time", now, 't', now.Format("3:04 PM")},
		{"in hours", now.Add(2 * time.Hour), 'R', "in 2 hours"},
		{"day ago", now.Add(-25 * time.Hour), 'R', "1 day ago"},
	} {
		t.Run(test.name, func(t *testing.T) {
			timestamp := &Timestamp{Time: test.time, Style: test.style}
			if got := timestamp.Format(now); got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}
