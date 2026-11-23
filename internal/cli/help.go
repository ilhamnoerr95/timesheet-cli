package cli

import (
	"bytes"
	"fmt"
	"strings"
	"text/tabwriter"
)

type row struct{ name, desc string }

var commands = []row{
	{"timesheet", "Fill in today's row (asks check in, check out, place, task)"},
	{"add [\"task\"]", "Same, without questions: use --in/--out/--place/--task or one task"},
	{"list", "Show the month's rows and which days are still empty"},
	{"rm [date]", "Clear check in, check out and task of a day (asks first)"},
	{"export", "Write the month's rows as CSV"},
	{"sheet", "Show linked timesheet files and switch between them"},
	{"sheet link <file.xlsx> [name]", "Link an xlsx file and make it active"},
	{"completion <shell>", "Print completion script (bash, zsh, fish)"},
	{"help", "Show this help"},
}

var options = []row{
	{"--date YYYY-MM-DD", "Day to fill in (add, rm), default today"},
	{"--in HH:MM|now", "Check in (add). 7.30 and 7:30 both work; - clears"},
	{"--out HH:MM|now", "Check out (add)"},
	{"--place TEXT", "Work place (add)"},
	{"--task TEXT", "Task (add); same as the positional task"},
	{"--month YYYY-MM", "Month to show (list, export), default this month"},
	{"--to FILE", "Write CSV to FILE instead of stdout (export)"},
	{"--sheet NAME", "Use another linked timesheet for this command"},
	{"--yes", "Do not ask before clearing (rm)"},
	{"--force", "Write even if Excel's lock file exists"},
}

// table aligns rows with tabwriter, then bolds the header line.
// Colour is applied after alignment because ANSI codes would break column widths.
func table(c painter, head [2]string, rows []row) string {
	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 0, 0, 3, ' ', 0)
	fmt.Fprintf(tw, "  %s\t%s\n", head[0], head[1])
	for _, r := range rows {
		fmt.Fprintf(tw, "  %s\t%s\n", r.name, r.desc)
	}
	tw.Flush()
	lines := strings.SplitN(buf.String(), "\n", 2)
	return c.bold(lines[0]) + "\n" + lines[1]
}

func usageText(c painter) string {
	return c.cyan("timesheet") + " - log daily work\n\n" +
		c.bold("COMMANDS") + "\n" + table(c, [2]string{"COMMAND", "DESCRIPTION"}, commands) + "\n" +
		c.bold("OPTIONS") + "\n" + table(c, [2]string{"OPTION", "DESCRIPTION"}, options) + "\n" +
		"Entries are written into the linked .xlsx. Config and daily backups: ~/.timesheet/ (override with TIMESHEET_HOME)\n"
}
