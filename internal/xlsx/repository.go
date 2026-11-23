package xlsx

import (
	"archive/zip"
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	ts "timesheet-cli/internal/timesheet"
)

// Repository stores timesheet entries in the rows of an .xlsx file. It implements timesheet.Repository.
//
// Layout it expects (and checks for): a header row "No. | Date | Check In | Check Out | Work Place | Task List"
// followed by one pre-filled row per working day. It fills cells in the row whose Date matches.
type Repository struct {
	Path       string
	BackupDir  string // a copy of the file is kept here before the first write of each day
	IgnoreLock bool   // write even if Excel's lock file exists
}

// LockedError reports that the lock file Excel creates while a workbook is open exists.
type LockedError struct {
	Lock    string
	ModTime time.Time
}

func (e *LockedError) Error() string {
	return fmt.Sprintf("Excel may have this file open (lock file %s, %s)", filepath.Base(e.Lock), e.ModTime.Format("2006-01-02 15:04"))
}

var excelEpoch = time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)

type tab struct {
	ref    tabRef
	xml    []byte
	rows   []row
	period string // YYYY-MM from the "Periode" cell, "" when absent
}

type workbook struct {
	pkg  *pkg
	tabs []tab
}

func serialDate(c cell) (string, bool) {
	if !c.numeric {
		return "", false
	}
	f, err := strconv.ParseFloat(c.value, 64)
	if err != nil || f < 1 {
		return "", false
	}
	return excelEpoch.AddDate(0, 0, int(math.Floor(f))).Format("2006-01-02"), true
}

func clockText(c cell) string {
	if c.value == "" {
		return ""
	}
	if c.numeric {
		f, err := strconv.ParseFloat(c.value, 64)
		if err != nil {
			return c.value
		}
		m := int(math.Round(f * 1440))
		return fmt.Sprintf("%02d:%02d", m/60%24, m%60)
	}
	if t, err := ts.ParseClock(c.value, time.Time{}); err == nil {
		return t
	}
	return c.value
}

func isHeader(r row) bool {
	g := func(c string) string { return r.cells[c].value }
	return g("A") == "No." && g("B") == "Date" && g("C") == "Check In" && g("D") == "Check Out" && g("E") == "Work Place"
}

func (r *Repository) load() (*workbook, error) {
	p, err := openPkg(r.Path)
	if err != nil {
		return nil, err
	}
	refs, err := p.tabs()
	if err != nil {
		return nil, err
	}
	shared, err := p.sharedStrings()
	if err != nil {
		return nil, err
	}
	wb := &workbook{pkg: p}
	for _, ref := range refs {
		b, err := p.read(ref.part)
		if err != nil {
			return nil, err
		}
		rows := parseRows(b, shared)
		hdr := -1
		for _, rw := range rows {
			if isHeader(rw) {
				hdr = rw.num
				break
			}
		}
		if hdr < 0 {
			continue // not a timesheet tab
		}
		t := tab{ref: ref, xml: b}
		for _, rw := range rows {
			if strings.HasPrefix(rw.cells["A"].value, "Periode") {
				if d, ok := serialDate(rw.cells["C"]); ok {
					t.period = d[:7]
				}
			}
			if rw.num > hdr {
				t.rows = append(t.rows, rw)
			}
		}
		wb.tabs = append(wb.tabs, t)
	}
	if len(wb.tabs) == 0 {
		return nil, fmt.Errorf("%s has no tab with the header No. | Date | Check In | Check Out | Work Place", filepath.Base(r.Path))
	}
	return wb, nil
}

type hit struct {
	tab   *tab
	row   row
	score int // lower is better: tab whose Periode matches the month first, then workbook order
	order int
}

// find returns the best row for each date in the workbook. A copied tab (same dates, another Periode)
// loses to the tab whose Periode matches the date's month.
func (w *workbook) find() map[string]hit {
	best := map[string]hit{}
	for i := range w.tabs {
		t := &w.tabs[i]
		for _, rw := range t.rows {
			d, ok := serialDate(rw.cells["B"])
			if !ok {
				continue
			}
			h := hit{tab: t, row: rw, score: 1, order: i}
			if t.period == d[:7] {
				h.score = 0
			}
			if cur, seen := best[d]; !seen || h.score < cur.score {
				best[d] = h
			}
		}
	}
	return best
}

func toEntry(date string, rw row) ts.Entry {
	return ts.Entry{
		Date:      date,
		CheckIn:   clockText(rw.cells["C"]),
		CheckOut:  clockText(rw.cells["D"]),
		WorkPlace: rw.cells["E"].value,
		Task:      rw.cells["F"].value,
	}
}

func (r *Repository) Get(date string) (ts.Entry, error) {
	w, err := r.load()
	if err != nil {
		return ts.Entry{}, err
	}
	h, ok := w.find()[date]
	if !ok {
		return ts.Entry{}, fmt.Errorf("%w: %s (weekend, or not in any tab of %s)", ts.ErrNoRow, date, filepath.Base(r.Path))
	}
	return toEntry(date, h.row), nil
}

func (r *Repository) List(month string) ([]ts.Entry, error) {
	w, err := r.load()
	if err != nil {
		return nil, err
	}
	var out []ts.Entry
	for d, h := range w.find() {
		if strings.HasPrefix(d, month) {
			out = append(out, toEntry(d, h.row))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out, nil
}

func timeEdit(col, hhmm string) edit {
	if hhmm == "" {
		return edit{col: col, kind: "empty"}
	}
	h, _ := strconv.Atoi(hhmm[:2])
	m, _ := strconv.Atoi(hhmm[3:])
	return edit{col: col, kind: "time", value: strconv.FormatFloat(float64(h*60+m)/1440, 'f', -1, 64)}
}

func textEdit(col, s string) edit {
	if s == "" {
		return edit{col: col, kind: "empty"}
	}
	return edit{col: col, kind: "text", value: s}
}

func (r *Repository) Save(e ts.Entry) error {
	edits := []edit{timeEdit("C", e.CheckIn), timeEdit("D", e.CheckOut), textEdit("F", e.Task)}
	if e.WorkPlace != "" {
		edits = append(edits, textEdit("E", e.WorkPlace))
	}
	if err := r.apply(e.Date, edits); err != nil {
		return err
	}
	// read the file back: refuse to leave a write we cannot read as intended
	got, err := r.Get(e.Date)
	if err != nil || got.CheckIn != e.CheckIn || got.CheckOut != e.CheckOut || got.Task != e.Task {
		return fmt.Errorf("verification after write failed for %s; restore from %s", e.Date, r.BackupDir)
	}
	return nil
}

func (r *Repository) Clear(date string) error {
	return r.apply(date, []edit{{col: "C", kind: "empty"}, {col: "D", kind: "empty"}, {col: "F", kind: "empty"}})
}

func (r *Repository) apply(date string, edits []edit) error {
	if !r.IgnoreLock {
		if err := r.checkLock(); err != nil {
			return err
		}
	}
	w, err := r.load()
	if err != nil {
		return err
	}
	h, ok := w.find()[date]
	if !ok {
		return fmt.Errorf("%w: %s (weekend, or not in any tab of %s)", ts.ErrNoRow, date, filepath.Base(r.Path))
	}
	patched, err := patchRow(h.tab.xml, h.row.num, edits)
	if err != nil {
		return err
	}
	if err := r.backup(w.pkg.raw); err != nil {
		return err
	}
	return r.write(w.pkg, map[string][]byte{h.tab.ref.part: patched})
}

// checkLock looks for the owner file Excel drops next to an open workbook.
func (r *Repository) checkLock() error {
	dir, base := filepath.Split(r.Path)
	for _, name := range []string{"~$" + base, "~$" + strings.TrimPrefix(base, base[:min(2, len(base))])} {
		if st, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return &LockedError{Lock: filepath.Join(dir, name), ModTime: st.ModTime()}
		}
	}
	return nil
}

// backup keeps one copy of the original per day, before the first write.
func (r *Repository) backup(orig []byte) error {
	if r.BackupDir == "" {
		return nil
	}
	if err := os.MkdirAll(r.BackupDir, 0o755); err != nil {
		return err
	}
	dst := filepath.Join(r.BackupDir, fmt.Sprintf("%s.%s.bak", filepath.Base(r.Path), time.Now().Format("2006-01-02")))
	if _, err := os.Stat(dst); err == nil {
		return nil
	}
	return os.WriteFile(dst, orig, 0o600)
}

// write copies every zip part unchanged (raw, without recompressing) except the replaced ones,
// then renames the result over the original so a crash never leaves a half-written file.
func (r *Repository) write(p *pkg, replace map[string][]byte) error {
	st, err := os.Stat(r.Path)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(r.Path), ".timesheet-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range p.zr.File {
		if data, ok := replace[f.Name]; ok {
			w, err := zw.CreateHeader(&zip.FileHeader{Name: f.Name, Method: zip.Deflate, Modified: f.Modified})
			if err != nil {
				return err
			}
			if _, err := w.Write(data); err != nil {
				return err
			}
			continue
		}
		if err := zw.Copy(f); err != nil {
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), st.Mode().Perm()); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), r.Path)
}
