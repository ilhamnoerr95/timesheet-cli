package xlsx

import (
	"bytes"
	"encoding/xml"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ts "timesheet-cli/internal/timesheet"
	"timesheet-cli/internal/xlsx/xlsxtest"
)

func partOf(t *testing.T, path, name string) []byte {
	t.Helper()
	p, err := openPkg(path)
	if err != nil {
		t.Fatal(err)
	}
	b, err := p.read(name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestReadSaveClear(t *testing.T) {
	dir := t.TempDir()
	path, image := xlsxtest.Build(t, dir, [][2]any{{"november", xlsxtest.SerNov1}})
	r := &Repository{Path: path, BackupDir: filepath.Join(dir, "bk")}

	e, err := r.Get("2026-11-02")
	if err != nil || e.WorkPlace != "Office" || e.Filled() {
		t.Fatalf("get empty row: %+v %v", e, err)
	}
	if _, err := r.Get("2026-11-30"); !errors.Is(err, ts.ErrNoRow) {
		t.Fatalf("missing date must be ErrNoRow, got %v", err)
	}

	want := ts.Entry{Date: "2026-11-02", CheckIn: "07:31", CheckOut: "16:45", WorkPlace: "Office", Task: `a <b> & "c"`}
	if err := r.Save(want); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.Get("2026-11-02"); got != want {
		t.Fatalf("round trip: %+v", got)
	}
	// every part except the edited sheet is byte-identical, and the XML is still well-formed
	if !bytes.Equal(partOf(t, path, "xl/media/image1.png"), image) {
		t.Fatal("image part changed")
	}
	if err := xml.Unmarshal(partOf(t, path, "xl/worksheets/sheet1.xml"), new(struct{})); err != nil {
		t.Fatalf("sheet XML not well-formed: %v", err)
	}
	if s := string(partOf(t, path, "xl/worksheets/sheet1.xml")); strings.Contains(s, ">>") {
		t.Fatalf("stray '>' in sheet XML: %s", s)
	}
	// neighbouring rows are untouched
	if got, _ := r.Get("2026-11-03"); got.Filled() {
		t.Fatalf("neighbour row changed: %+v", got)
	}

	if err := r.Clear("2026-11-02"); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.Get("2026-11-02"); got.Filled() || got.WorkPlace != "Office" {
		t.Fatalf("clear must empty in/out/task and keep the work place: %+v", got)
	}
	// one backup per day, even after several writes
	if files, _ := os.ReadDir(filepath.Join(dir, "bk")); len(files) != 1 {
		t.Fatalf("want 1 backup, got %d", len(files))
	}
}

func TestListAndTabChoice(t *testing.T) {
	dir := t.TempDir()
	// the first tab is a stale copy (Periode = October) holding the same November dates
	path, _ := xlsxtest.Build(t, dir, [][2]any{{"copy", xlsxtest.SerOct1}, {"november", xlsxtest.SerNov1}})
	r := &Repository{Path: path}
	if err := r.Save(ts.Entry{Date: "2026-11-02", CheckIn: "08:00", WorkPlace: "Office"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(partOf(t, path, "xl/worksheets/sheet2.xml")), "0.3333") {
		t.Fatal("entry must go to the tab whose Periode matches the month")
	}
	if strings.Contains(string(partOf(t, path, "xl/worksheets/sheet1.xml")), "0.3333") {
		t.Fatal("the copied tab must stay untouched")
	}
	list, err := r.List("2026-11")
	if err != nil || len(list) != 3 || list[0].Date != "2026-11-02" || list[0].CheckIn != "08:00" {
		t.Fatalf("list: %+v %v", list, err)
	}
}

func TestLockAndFormula(t *testing.T) {
	dir := t.TempDir()
	path, _ := xlsxtest.Build(t, dir, [][2]any{{"november", xlsxtest.SerNov1}})
	os.WriteFile(filepath.Join(dir, "~$book.xlsx"), []byte("lock"), 0o644)
	r := &Repository{Path: path}
	var le *LockedError
	if err := r.Save(ts.Entry{Date: "2026-11-02", CheckIn: "08:00"}); !errors.As(err, &le) {
		t.Fatalf("lock file must block the write, got %v", err)
	}
	if got, _ := r.Get("2026-11-02"); got.CheckIn != "" {
		t.Fatal("nothing may be written while locked")
	}
	r.IgnoreLock = true
	if err := r.Save(ts.Entry{Date: "2026-11-02", CheckIn: "08:00"}); err != nil {
		t.Fatalf("IgnoreLock: %v", err)
	}
	if _, err := patchRow([]byte(xlsxtest.SheetXML(xlsxtest.SerNov1, xlsxtest.SerNov1+1, true)), 18, []edit{{col: "F", kind: "text", value: "x"}}); err == nil || !strings.Contains(err.Error(), "formula") {
		t.Fatalf("formula cell must be refused, got %v", err)
	}
}

func TestRejectsNonTimesheet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.xlsx")
	os.WriteFile(path, []byte("not a zip"), 0o644)
	if _, err := (&Repository{Path: path}).Get("2026-11-02"); err == nil {
		t.Fatal("garbage file must error")
	}
}
