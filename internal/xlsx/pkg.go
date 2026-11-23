// Package xlsx reads and edits timesheet workbooks without a third-party library.
// Edits touch only the cells of one row inside one worksheet part; every other
// part of the zip (images, styles, other tabs) is copied byte-for-byte.
package xlsx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"os"
	"path"
	"strings"
)

type pkg struct {
	raw []byte
	zr  *zip.Reader
}

func openPkg(file string) (*pkg, error) {
	raw, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, fmt.Errorf("%s is not a valid xlsx file: %w", file, err)
	}
	return &pkg{raw, zr}, nil
}

func (p *pkg) read(name string) ([]byte, error) {
	for _, f := range p.zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("part %s not found in workbook", name)
}

type tabRef struct{ name, part string }

const relNS = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"

// tabs lists the worksheets in workbook order with the zip path of each one.
func (p *pkg) tabs() ([]tabRef, error) {
	wbXML, err := p.read("xl/workbook.xml")
	if err != nil {
		return nil, err
	}
	relsXML, err := p.read("xl/_rels/workbook.xml.rels")
	if err != nil {
		return nil, err
	}
	var wb struct {
		Sheets []struct {
			Name string `xml:"name,attr"`
			RID  string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
		} `xml:"sheets>sheet"`
	}
	var rels struct {
		Rel []struct {
			ID     string `xml:"Id,attr"`
			Target string `xml:"Target,attr"`
		} `xml:"Relationship"`
	}
	if err := xml.Unmarshal(wbXML, &wb); err != nil {
		return nil, err
	}
	if err := xml.Unmarshal(relsXML, &rels); err != nil {
		return nil, err
	}
	target := map[string]string{}
	for _, r := range rels.Rel {
		target[r.ID] = r.Target
	}
	var out []tabRef
	for _, s := range wb.Sheets {
		t, ok := target[s.RID]
		if !ok {
			continue
		}
		if strings.HasPrefix(t, "/") {
			t = strings.TrimPrefix(t, "/")
		} else {
			t = path.Join("xl", t)
		}
		out = append(out, tabRef{s.Name, t})
	}
	return out, nil
}

// sharedStrings returns the workbook string table (empty when the workbook has none).
func (p *pkg) sharedStrings() ([]string, error) {
	b, err := p.read("xl/sharedStrings.xml")
	if err != nil {
		return nil, nil
	}
	var sst struct {
		SI []struct {
			T string `xml:"t"`
			R []struct {
				T string `xml:"t"`
			} `xml:"r"`
		} `xml:"si"`
	}
	if err := xml.Unmarshal(b, &sst); err != nil {
		return nil, err
	}
	out := make([]string, len(sst.SI))
	for i, si := range sst.SI {
		out[i] = si.T
		for _, r := range si.R {
			out[i] += r.T
		}
	}
	return out, nil
}

func unescape(s string) string { return html.UnescapeString(s) }
