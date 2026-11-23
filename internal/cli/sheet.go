package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	ts "timesheet-cli/internal/timesheet"
	"timesheet-cli/internal/xlsx"
)

func first(s []string) string {
	if len(s) == 0 {
		return ""
	}
	return s[0]
}

// sheetCmd links xlsx files and switches the active one.
//
//	timesheet sheet                      show linked files and pick one
//	timesheet sheet <name>               switch
//	timesheet sheet link <file> [name]   link a file (name defaults to the file name) and make it active
func sheetCmd(a *app, pos []string) error {
	cfg, err := a.cfg.Load()
	if err != nil {
		return err
	}
	switch first(pos) {
	case "link":
		return linkSheet(a, cfg, pos[1:])
	case "":
		return pickSheet(a, cfg)
	}
	return switchSheet(a, cfg, pos[0])
}

func linkSheet(a *app, cfg ts.Config, args []string) error {
	if len(args) < 1 || len(args) > 2 {
		return usageError("sheet link needs <file.xlsx> [name]")
	}
	path, err := filepath.Abs(args[0])
	if err != nil {
		return err
	}
	if st, err := os.Stat(path); err != nil || st.IsDir() {
		return fmt.Errorf("%s is not a file", path)
	}
	// loading proves the file has the timesheet header before we remember it
	if _, err := (&xlsx.Repository{Path: path}).List(time.Now().Format("2006-01")); err != nil {
		return err
	}
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if len(args) == 2 {
		name = args[1]
	}
	if name == "" || name == "link" {
		return usageError("pick another name for the timesheet")
	}
	if cfg.Sheets == nil {
		cfg.Sheets = map[string]string{}
	}
	cfg.Sheets[name], cfg.ActiveSheet = path, name
	if err := a.cfg.Save(cfg); err != nil {
		return err
	}
	fmt.Fprintf(a.out, "linked %s -> %s (active)\n", a.p.c.green(name), path)
	return nil
}

func switchSheet(a *app, cfg ts.Config, name string) error {
	if _, ok := cfg.Sheets[name]; !ok {
		return usageError(fmt.Sprintf("unknown timesheet %q. Link it with: timesheet sheet link <file.xlsx> %s", name, name))
	}
	cfg.ActiveSheet = name
	if err := a.cfg.Save(cfg); err != nil {
		return err
	}
	fmt.Fprintf(a.out, "active timesheet: %s -> %s\n", a.p.c.green(name), cfg.Sheets[name])
	return nil
}

func pickSheet(a *app, cfg ts.Config) error {
	if len(cfg.Sheets) == 0 {
		fmt.Fprintln(a.out, "no timesheet linked. Run: timesheet sheet link <file.xlsx>")
		return nil
	}
	c := a.p.c
	names := make([]string, 0, len(cfg.Sheets))
	for n := range cfg.Sheets {
		names = append(names, n)
	}
	sort.Strings(names)
	for i, n := range names {
		mark := " "
		if n == cfg.ActiveSheet {
			mark = "*"
		}
		fmt.Fprintf(a.out, " %s %s %-12s %s\n", mark, c.dim(strconv.Itoa(i+1)+")"), n, c.dim(cfg.Sheets[n]))
	}
	s, err := a.p.ask("Switch to (name or number, Enter keeps)", "")
	if err != nil || s == "" || s == cfg.ActiveSheet {
		return err
	}
	if n, err := strconv.Atoi(s); err == nil && n >= 1 && n <= len(names) {
		s = names[n-1]
	}
	return switchSheet(a, cfg, s)
}
