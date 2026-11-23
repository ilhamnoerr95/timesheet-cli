package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	ts "timesheet-cli/internal/timesheet"
	"timesheet-cli/internal/xlsx"
)

// root is where the config and daily backups live.
func root() string {
	if p := os.Getenv("TIMESHEET_HOME"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".timesheet")
}

// parse accepts flags before or after positional args (std flag stops at the first positional).
func parse(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			return pos, nil
		}
		pos = append(pos, fs.Arg(0))
		args = fs.Args()[1:]
	}
}

type usageError string

func (e usageError) Error() string { return string(e) }

// Run returns the process exit code: 0 ok, 1 runtime error, 2 usage error.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	cmd := "add" // bare `timesheet` logs today
	if len(args) > 0 {
		cmd, args = args[0], args[1:]
	}
	if cmd == "help" || cmd == "-h" || cmd == "--help" {
		fmt.Fprint(stdout, usageText(newPainter(stdout)))
		return 0
	}
	a := app{
		cfg: ts.JSONConfigStore{Path: filepath.Join(root(), "config.json")},
		p:   newPrompter(stdin, stdout),
		out: stdout,
	}
	err := dispatch(&a, cmd, args)
	var ue usageError
	switch {
	case err == nil:
		return 0
	case errors.Is(err, flag.ErrHelp):
		fmt.Fprint(stdout, usageText(newPainter(stdout)))
		return 0
	case errors.Is(err, io.EOF):
		fmt.Fprintln(stderr, "\ncancelled, nothing saved")
		return 1
	case errors.As(err, &ue):
		fmt.Fprintf(stderr, "%s %v\n\n%s", newPainter(stderr).red("error:"), err, usageText(newPainter(stderr)))
		return 2
	}
	fmt.Fprintln(stderr, newPainter(stderr).red("error:"), err)
	return 1
}

// app bundles what every command needs.
type app struct {
	svc   *ts.Service // bound to the active timesheet by useSheet
	repo  *xlsx.Repository
	name  string // name of the active timesheet
	cfg   ts.JSONConfigStore
	p     prompter
	out   io.Writer
	force bool
}

// useSheet binds the service to the named timesheet, falling back to the saved active one.
func (a *app) useSheet(flagValue string) error {
	cfg, err := a.cfg.Load()
	if err != nil {
		return err
	}
	name := flagValue
	if name == "" {
		name = cfg.ActiveSheet
	}
	if name == "" && len(cfg.Sheets) == 1 {
		for n := range cfg.Sheets {
			name = n
		}
	}
	if name == "" {
		return fmt.Errorf("no timesheet linked yet. Run: timesheet sheet link <file.xlsx>")
	}
	path, ok := cfg.Sheets[name]
	if !ok {
		return usageError(fmt.Sprintf("unknown timesheet %q. Link it with: timesheet sheet link <file.xlsx> %s", name, name))
	}
	a.name = name
	a.repo = &xlsx.Repository{Path: path, BackupDir: filepath.Join(root(), "backups"), IgnoreLock: a.force}
	a.svc = ts.NewService(a.repo)
	return nil
}

func dispatch(a *app, cmd string, args []string) error {
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	f := flags{
		date:  fs.String("date", time.Now().Format("2006-01-02"), ""),
		month: fs.String("month", time.Now().Format("2006-01"), ""),
		to:    fs.String("to", "", ""),
		in:    fs.String("in", "", ""),
		out:   fs.String("out", "", ""),
		place: fs.String("place", "", ""),
		task:  fs.String("task", "", ""),
		sheet: fs.String("sheet", "", ""),
		force: fs.Bool("force", false, ""),
		yes:   fs.Bool("yes", false, ""),
	}
	pos, err := parse(fs, args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return err
		}
		return usageError(err.Error())
	}
	f.set = map[string]bool{}
	fs.Visit(func(x *flag.Flag) { f.set[x.Name] = true })
	a.force = *f.force

	switch cmd {
	case "completion":
		return completion(pos, a.out)
	case "sheet":
		return sheetCmd(a, pos)
	case "add", "list", "rm", "export":
		if err := a.useSheet(*f.sheet); err != nil {
			return err
		}
	default:
		return usageError(fmt.Sprintf("unknown command %q", cmd))
	}
	switch cmd {
	case "add":
		return add(a, f, pos)
	case "list":
		return list(a, *f.month)
	case "rm":
		return rm(a, f, pos)
	default:
		return export(a, *f.month, *f.to)
	}
}

type flags struct {
	date, month, to, in, out, place, task, sheet *string
	force, yes                                   *bool
	set                                          map[string]bool // flags the user actually passed
}
