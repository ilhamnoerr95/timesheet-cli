package cli

import (
	"io"
	"os"
)

// painter wraps text in ANSI codes only when the target is a terminal.
// NO_COLOR (https://no-color.org) and TERM=dumb turn it off; pipes and test buffers never get colour.
type painter struct{ on bool }

func newPainter(w io.Writer) painter {
	f, ok := w.(*os.File)
	if !ok || os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return painter{}
	}
	st, err := f.Stat()
	return painter{err == nil && st.Mode()&os.ModeCharDevice != 0}
}

func (p painter) wrap(code, s string) string {
	if !p.on {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func (p painter) bold(s string) string  { return p.wrap("1", s) }
func (p painter) dim(s string) string   { return p.wrap("2", s) }
func (p painter) green(s string) string { return p.wrap("32", s) }
func (p painter) red(s string) string   { return p.wrap("31", s) }
func (p painter) cyan(s string) string  { return p.wrap("36", s) }
