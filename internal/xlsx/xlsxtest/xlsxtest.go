// Package xlsxtest builds tiny timesheet workbooks for tests.
package xlsxtest

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Excel serials: 46327 = 2026-11-01, 46296 = 2026-10-01.
const (
	SerNov1 = 46327
	SerOct1 = 46296
)

var sharedStrings0 = []string{"Periode :", "No.", "Date", "Check In", "Check Out", "Work Place", "Task List", "Office"}

func SheetXML(period int, firstDay int, formulaRow bool) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	fmt.Fprintf(&sb, `<row r="11"><c r="A11" t="s"><v>0</v></c><c r="C11" s="40"><v>%d</v></c></row>`, period)
	sb.WriteString(`<row r="14"><c r="A14" t="s"><v>1</v></c><c r="B14" t="s"><v>2</v></c><c r="C14" t="s"><v>3</v></c><c r="D14" t="s"><v>4</v></c><c r="E14" t="s"><v>5</v></c><c r="F14" t="s"><v>6</v></c></row>`)
	for i := 0; i < 3; i++ {
		r := 15 + i
		fmt.Fprintf(&sb, `<row r="%d" ht="49"><c r="A%d"><v>%d</v></c><c r="B%d" s="18"><v>%d</v></c><c r="C%d" s="19"/><c r="D%d" s="19"></c><c r="E%d" s="14" t="s"><v>7</v></c><c r="F%d" s="15"/></row>`,
			r, r, i+1, r, firstDay+i, r, r, r, r)
	}
	if formulaRow {
		sb.WriteString(`<row r="18"><c r="B18"><v>46400</v></c><c r="F18"><f>A1&amp;"x"</f><v>x</v></c></row>`)
	}
	sb.WriteString(`</sheetData></worksheet>`)
	return sb.String()
}

// Build writes a small workbook with the given tabs (name, periodSerial) and a fake image part.
func Build(t testing.TB, dir string, tabs [][2]any) (path string, image []byte) {
	t.Helper()
	image = []byte("\x89PNG-not-really-an-image")
	var wb, rels, sst strings.Builder
	wb.WriteString(`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets>`)
	rels.WriteString(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	for i, tb := range tabs {
		fmt.Fprintf(&wb, `<sheet name="%s" sheetId="%d" r:id="rId%d"/>`, tb[0], i+1, i+1)
		target := fmt.Sprintf("worksheets/sheet%d.xml", i+1)
		if i == 0 {
			target = "/xl/" + target // both absolute and relative targets occur in real files
		}
		fmt.Fprintf(&rels, `<Relationship Id="rId%d" Target="%s"/>`, i+1, target)
	}
	wb.WriteString(`</sheets></workbook>`)
	rels.WriteString(`</Relationships>`)
	sst.WriteString(`<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`)
	for _, s := range sharedStrings0 {
		fmt.Fprintf(&sst, `<si><t>%s</t></si>`, s)
	}
	sst.WriteString(`</sst>`)

	path = filepath.Join(dir, "book.xlsx")
	f, _ := os.Create(path)
	zw := zip.NewWriter(f)
	add := func(name string, data []byte) {
		w, _ := zw.Create(name)
		w.Write(data)
	}
	add("[Content_Types].xml", []byte(`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>`))
	add("xl/workbook.xml", []byte(wb.String()))
	add("xl/_rels/workbook.xml.rels", []byte(rels.String()))
	add("xl/sharedStrings.xml", []byte(sst.String()))
	add("xl/media/image1.png", image)
	for i, tb := range tabs {
		add(fmt.Sprintf("xl/worksheets/sheet%d.xml", i+1), []byte(SheetXML(tb[1].(int), SerNov1+1, i == len(tabs)-1 && tb[0] == "formula")))
	}
	zw.Close()
	f.Close()
	return path, image
}
