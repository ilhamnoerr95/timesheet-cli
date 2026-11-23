package cli

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"time"

	ts "timesheet-cli/internal/timesheet"
)

type prompter struct {
	in  *bufio.Reader
	out io.Writer
	c   painter
}

func newPrompter(in io.Reader, out io.Writer) prompter {
	return prompter{bufio.NewReader(in), out, newPainter(out)}
}

// ask shows "label [default]: " and returns the typed line, or def when empty.
func (p prompter) ask(label, def string) (string, error) {
	if def != "" {
		fmt.Fprintf(p.out, "%s %s: ", p.c.bold(label), p.c.dim("["+def+"]"))
	} else {
		fmt.Fprintf(p.out, "%s: ", p.c.bold(label))
	}
	line, err := p.in.ReadString('\n')
	if err != nil && line == "" {
		return "", err // EOF or Ctrl-D: abort without saving
	}
	if line = strings.TrimSpace(line); line != "" {
		return line, nil
	}
	return def, nil
}

// askClock re-asks until the answer is a valid time ("-" clears, "now" is the current time).
func (p prompter) askClock(label, def string, now time.Time) (string, error) {
	for {
		s, err := p.ask(label, def)
		if err != nil {
			return "", err
		}
		v, perr := ts.ParseClock(s, now)
		if perr == nil {
			return v, nil
		}
		fmt.Fprintln(p.out, p.c.red("  use HH:MM, e.g. 07:30 (7.30 works too), or 'now'"))
	}
}

func (p prompter) confirm(label string) (bool, error) {
	s, err := p.ask(label+" [y/N]", "")
	if err != nil {
		return false, err
	}
	s = strings.ToLower(s)
	return s == "y" || s == "yes", nil
}
