package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"timesheet-cli/internal/xlsx/xlsxtest"
)

func run(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := Run(args, strings.NewReader(stdin), &out, &errb)
	return code, out.String(), errb.String()
}

// setup links a fresh workbook (rows for 2026-11-02..04, work place "Office") and returns its path.
func setup(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("TIMESHEET_HOME", filepath.Join(dir, "home"))
	p, _ := xlsxtest.Build(t, dir, [][2]any{{"november", xlsxtest.SerNov1}})
	if code, _, errs := run(t, "", "sheet", "link", p); code != 0 {
		t.Fatalf("link: %s", errs)
	}
	return p
}

func contains(t *testing.T, got string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Fatalf("missing %q in:\n%s", w, got)
		}
	}
}

func TestNoSheetLinked(t *testing.T) {
	t.Setenv("TIMESHEET_HOME", t.TempDir())
	code, _, errs := run(t, "", "add", "--in", "8:00")
	if code != 1 {
		t.Fatalf("code=%d", code)
	}
	contains(t, errs, "no timesheet linked")
}

func TestFlagsAndPositional(t *testing.T) {
	setup(t)
	code, out, errs := run(t, "", "add", "--date", "2026-11-02", "--in", "7.30", "--out", "16:45", "--task", "fix login")
	if code != 0 {
		t.Fatalf("%d %s", code, errs)
	}
	contains(t, out, "saved 2026-11-02 in book", "07:30 - 16:45", "Office", "fix login")
	if _, out, _ := run(t, "", "add", "--date", "2026-11-03", "only a task"); !strings.Contains(out, "only a task") {
		t.Fatalf("positional task: %s", out)
	}
	if _, out, _ := run(t, "", "add", "--date", "2026-11-02", "--in", "07:30"); !strings.Contains(out, "no changes") {
		t.Fatalf("same value: %s", out)
	}
	_, out, _ = run(t, "", "list", "--month", "2026-11")
	contains(t, out, "2026-11-02 Mon", "07:30", "16:45", "Filled: 2 of 3 days")
}

func TestInteractive(t *testing.T) {
	setup(t)
	// check in with a dot, check out, Enter keeps the work place, a task
	code, out, _ := run(t, "7.30\n16:45\n\nfix bug\n", "add", "--date", "2026-11-02")
	if code != 0 {
		t.Fatalf("code=%d %s", code, out)
	}
	contains(t, out, "Check in", "Check out", "Work place", "Task", "saved 2026-11-02", "07:30 - 16:45 | Office | fix bug")
	// again: existing values are the defaults, so four Enters change nothing
	_, out, _ = run(t, "\n\n\n\n", "add", "--date", "2026-11-02")
	contains(t, out, "Already logged", "[07:30]", "no changes")
	// "-" clears a value
	_, out, _ = run(t, "\n\n\n-\n", "add", "--date", "2026-11-02")
	contains(t, out, "07:30 - 16:45 | Office | -")
	// a bad time is asked again
	_, out, _ = run(t, "abc\n8:00\n\n\n\n", "add", "--date", "2026-11-03")
	contains(t, out, "use HH:MM", "saved 2026-11-03", "08:00")
	// EOF in the middle saves nothing
	if code, _, errs := run(t, "9:00\n", "add", "--date", "2026-11-04"); code != 1 || !strings.Contains(errs, "nothing saved") {
		t.Fatalf("code=%d %s", code, errs)
	}
	if _, out, _ := run(t, "", "list", "--month", "2026-11"); strings.Contains(out, "09:00") {
		t.Fatalf("cancelled run was saved: %s", out)
	}
}

func TestRejects(t *testing.T) {
	setup(t)
	cases := []struct {
		args []string
		code int
		msg  string
	}{
		{[]string{"add", "--date", "2026-11-30", "--in", "8:00"}, 1, "no row"},
		{[]string{"add", "--date", "2026-11-02", "--in", "25:00"}, 1, "out of range"},
		{[]string{"add", "--date", "2026-11-02", "--in", "16:00", "--out", "08:00"}, 1, "before check in"},
		{[]string{"add", "--date", "nope", "--in", "8:00"}, 1, "YYYY-MM-DD"},
		{[]string{"add", "a", "b"}, 2, "at most one"},
		{[]string{"bogus"}, 2, "unknown command"},
		{[]string{"sheet", "nope"}, 2, "unknown timesheet"},
		{[]string{"list", "--month", "2026/11"}, 1, "YYYY-MM"},
	}
	for _, c := range cases {
		code, _, errs := run(t, "", c.args...)
		if code != c.code || !strings.Contains(errs, c.msg) {
			t.Errorf("%v: code=%d err=%q (want %d, %q)", c.args, code, errs, c.code, c.msg)
		}
	}
	if code, _, _ := run(t, "", "help"); code != 0 {
		t.Error("help should exit 0")
	}
}

func TestRm(t *testing.T) {
	setup(t)
	run(t, "", "add", "--date", "2026-11-02", "--in", "8:00", "--task", "x")
	if _, out, _ := run(t, "n\n", "rm", "2026-11-02"); !strings.Contains(out, "kept") {
		t.Fatalf("n keeps: %s", out)
	}
	if _, out, _ := run(t, "", "list", "--month", "2026-11"); !strings.Contains(out, "08:00") {
		t.Fatal("row must survive a 'n'")
	}
	if _, out, _ := run(t, "y\n", "rm", "2026-11-02"); !strings.Contains(out, "cleared 2026-11-02") {
		t.Fatalf("y clears: %s", out)
	}
	if _, out, _ := run(t, "", "rm", "2026-11-02", "--yes"); !strings.Contains(out, "nothing to clear") {
		t.Fatalf("empty: %s", out)
	}
	if _, out, _ := run(t, "", "list", "--month", "2026-11"); !strings.Contains(out, "Office") || strings.Contains(out, "08:00") {
		t.Fatalf("clear keeps the work place only: %s", out)
	}
}

func TestLockFile(t *testing.T) {
	p := setup(t)
	lock := filepath.Join(filepath.Dir(p), "~$book.xlsx")
	os.WriteFile(lock, []byte("x"), 0o644)
	// refuse
	code, out, errs := run(t, "n\n", "add", "--date", "2026-11-02", "--in", "8:00")
	if code != 1 || !strings.Contains(out, "Excel may have this file open") || !strings.Contains(errs, "not written") {
		t.Fatalf("refuse: %d %s %s", code, out, errs)
	}
	if _, out, _ := run(t, "", "list", "--month", "2026-11"); strings.Contains(out, "08:00") {
		t.Fatal("nothing may be written when refused")
	}
	// accept at the prompt
	if code, out, _ := run(t, "y\n", "add", "--date", "2026-11-02", "--in", "8:00"); code != 0 || !strings.Contains(out, "saved") {
		t.Fatalf("accept: %d %s", code, out)
	}
	// --force skips the question
	if code, out, _ := run(t, "", "add", "--date", "2026-11-03", "--in", "9:00", "--force"); code != 0 || !strings.Contains(out, "saved") {
		t.Fatalf("force: %d %s", code, out)
	}
}

func TestSheetsExportReminder(t *testing.T) {
	p := setup(t)
	// a second file, picked by number, then used for one command with --sheet
	other, _ := xlsxtest.Build(t, t.TempDir(), [][2]any{{"november", xlsxtest.SerNov1}})
	if code, _, errs := run(t, "", "sheet", "link", other, "second"); code != 0 {
		t.Fatal(errs)
	}
	run(t, "", "add", "--date", "2026-11-02", "--in", "8:00", "--sheet", "book")
	if _, out, _ := run(t, "", "list", "--month", "2026-11"); strings.Contains(out, "08:00") {
		t.Fatalf("active sheet is 'second', which must be empty: %s", out)
	}
	if _, out, _ := run(t, "1\n", "sheet"); !strings.Contains(out, "active timesheet: book") || !strings.Contains(out, p) {
		t.Fatalf("pick by number: %s", out)
	}
	if _, out, _ := run(t, "", "export", "--month", "2026-11"); !strings.HasPrefix(out, "date,check_in,check_out,work_place,task") || !strings.Contains(out, "2026-11-02,08:00") {
		t.Fatalf("export: %s", out)
	}
	if time.Now().After(time.Date(2026, 11, 5, 0, 0, 0, 0, time.Local)) {
		_, out, _ := run(t, "", "add", "--date", "2026-11-04", "--in", "8:00")
		contains(t, out, "Still empty: 2026-11-03")
	}
}

func TestCompletion(t *testing.T) {
	for _, sh := range []string{"bash", "zsh", "fish"} {
		if code, out, _ := run(t, "", "completion", sh); code != 0 || !strings.Contains(out, "timesheet") {
			t.Errorf("%s: code=%d", sh, code)
		}
	}
	if code, _, _ := run(t, "", "completion", "powershell"); code != 2 {
		t.Error("unsupported shell should exit 2")
	}
}
