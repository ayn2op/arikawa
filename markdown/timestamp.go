package markdown

import (
	"regexp"
	"strconv"
	"time"

	"github.com/yuin/goldmark/v2/ast"
	"github.com/yuin/goldmark/v2/parser"
	"github.com/yuin/goldmark/v2/text"
)

// Timestamp is <t:unix> or <t:unix:style>.
type Timestamp struct {
	ast.BaseInline

	Time  time.Time
	Style byte
}

var KindTimestamp = ast.NewNodeKind("Timestamp")

// Kind implements Node.Kind.
func (t *Timestamp) Kind() ast.NodeKind {
	return KindTimestamp
}

// Dump implements Node.Dump
func (t *Timestamp) Dump(source []byte) *ast.NodeDump {
	return ast.NewNodeDump(t, map[string]any{"Time": t.Time.String(), "Style": string(t.Style)})
}

// Format returns the timestamp in local time, or relative to now for the R style.
func (t *Timestamp) Format(now time.Time) string {
	var layout string
	switch t.Style {
	case 'R':
		return relativeTime(t.Time.Sub(now))
	case 't':
		layout = "3:04 PM"
	case 'T':
		layout = "3:04:05 PM"
	case 'd':
		layout = "01/02/2006"
	case 'D':
		layout = "January 2, 2006"
	case 'F':
		layout = "Monday, January 2, 2006 at 3:04 PM"
	case 's':
		layout = "01/02/2006, 3:04 PM"
	case 'S':
		layout = "01/02/2006, 3:04:05 PM"
	default:
		layout = "January 2, 2006 at 3:04 PM"
	}
	return t.Time.Local().Format(layout)
}

var relativeUnits = []struct {
	name   string
	length time.Duration
}{
	{"year", 365 * 24 * time.Hour},
	{"month", 30 * 24 * time.Hour},
	{"day", 24 * time.Hour},
	{"hour", time.Hour},
	{"minute", time.Minute},
	{"second", time.Second},
}

func relativeTime(duration time.Duration) string {
	magnitude := duration.Abs()
	unit := relativeUnits[0]
	for _, unit = range relativeUnits {
		if magnitude >= unit.length {
			break
		}
	}

	count := int64(magnitude / unit.length)
	amount := strconv.FormatInt(count, 10) + " " + unit.name
	if count != 1 {
		amount += "s"
	}

	if duration < 0 {
		return amount + " ago"
	}
	return "in " + amount
}

type timestamp struct{}

var timestampRegex = regexp.MustCompile(`^<t:(-?\d{1,13})(?::([tTdDfFsSR]))?>$`)

func (timestamp) Trigger() []byte {
	return []byte{'<'}
}

func (timestamp) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	matches := timestampRegex.FindSubmatch(matchInline(block, '<', '>'))
	if matches == nil {
		return nil
	}

	seconds, _ := strconv.ParseInt(string(matches[1]), 10, 64)
	node := &Timestamp{Time: time.Unix(seconds, 0), Style: 'f'}
	if len(matches[2]) > 0 {
		node.Style = matches[2][0]
	}
	node.Init(node)
	return node
}
