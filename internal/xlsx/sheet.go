package xlsx

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	rowRe  = regexp.MustCompile(`(?s)<row\b[^>]*?(?:/>|>.*?</row>)`)
	cellRe = regexp.MustCompile(`(?s)<c\b([^>]*?)(?:/>|>(.*?)</c>)`)
	attrRe = regexp.MustCompile(`\b([A-Za-z:]+)="([^"]*)"`)
	textRe = regexp.MustCompile(`(?s)<t\b[^>]*>(.*?)</t>`)
	vRe    = regexp.MustCompile(`(?s)<v>(.*?)</v>`)
)

// cell is the decoded content of one cell: text for strings, the number as written otherwise.
type cell struct {
	value   string
	numeric bool
}

type row struct {
	num   int
	cells map[string]cell // keyed by column letters
}

func attrs(s string) map[string]string {
	m := map[string]string{}
	for _, a := range attrRe.FindAllStringSubmatch(s, -1) {
		m[a[1]] = a[2]
	}
	return m
}

func splitRef(ref string) (col string, n int) {
	i := strings.IndexAny(ref, "0123456789")
	if i < 0 {
		return ref, 0
	}
	n, _ = strconv.Atoi(ref[i:])
	return ref[:i], n
}

func decodeCell(attr, body string, shared []string) cell {
	typ := attrs(attr)["t"]
	switch typ {
	case "s":
		if m := vRe.FindStringSubmatch(body); m != nil {
			if i, err := strconv.Atoi(m[1]); err == nil && i >= 0 && i < len(shared) {
				return cell{value: strings.TrimSpace(shared[i])}
			}
		}
	case "inlineStr":
		var sb strings.Builder
		for _, m := range textRe.FindAllStringSubmatch(body, -1) {
			sb.WriteString(unescape(m[1]))
		}
		return cell{value: strings.TrimSpace(sb.String())}
	case "str":
		if m := vRe.FindStringSubmatch(body); m != nil {
			return cell{value: strings.TrimSpace(unescape(m[1]))}
		}
	default:
		if m := vRe.FindStringSubmatch(body); m != nil {
			return cell{value: m[1], numeric: true}
		}
	}
	return cell{}
}

// parseRows decodes every row of a worksheet part.
func parseRows(sheetXML []byte, shared []string) []row {
	var rows []row
	for _, r := range rowRe.FindAll(sheetXML, -1) {
		s := string(r)
		open := s[:strings.Index(s, ">")+1]
		num, _ := strconv.Atoi(attrs(open)["r"])
		rw := row{num: num, cells: map[string]cell{}}
		for _, m := range cellRe.FindAllStringSubmatch(s, -1) {
			col, _ := splitRef(attrs(m[1])["r"])
			if c := decodeCell(m[1], m[2], shared); c.value != "" {
				rw.cells[col] = c
			}
		}
		rows = append(rows, rw)
	}
	return rows
}

type edit struct {
	col   string
	kind  string // "time" (fraction of a day), "text" or "empty"
	value string
}

func colIndex(col string) int {
	n := 0
	for _, r := range col {
		n = n*26 + int(r-'A'+1)
	}
	return n
}

func xmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}

func buildCell(col string, rowNum int, style string, e edit) string {
	ref := fmt.Sprintf(`r="%s%d"`, col, rowNum)
	if style != "" {
		ref += fmt.Sprintf(` s="%s"`, style)
	}
	switch e.kind {
	case "time":
		return fmt.Sprintf(`<c %s><v>%s</v></c>`, ref, e.value)
	case "text":
		return fmt.Sprintf(`<c %s t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>`, ref, xmlEscape(e.value))
	}
	return fmt.Sprintf(`<c %s/>`, ref)
}

// patchRow rewrites the cells in edits inside row rowNum and returns the new worksheet XML.
// Everything outside that row is left untouched. It refuses to overwrite formula cells.
func patchRow(sheetXML []byte, rowNum int, edits []edit) ([]byte, error) {
	for _, loc := range rowRe.FindAllIndex(sheetXML, -1) {
		s := string(sheetXML[loc[0]:loc[1]])
		open := s[:strings.Index(s, ">")+1]
		if n, _ := strconv.Atoi(attrs(open)["r"]); n != rowNum {
			continue
		}
		newRow, err := patchRowXML(s, open, rowNum, edits)
		if err != nil {
			return nil, err
		}
		out := append([]byte{}, sheetXML[:loc[0]]...)
		out = append(out, newRow...)
		return append(out, sheetXML[loc[1]:]...), nil
	}
	return nil, fmt.Errorf("row %d not found", rowNum)
}

type seg struct {
	col string
	xml string
}

func patchRowXML(s, open string, rowNum int, edits []edit) (string, error) {
	var segs []seg
	for _, m := range cellRe.FindAllStringSubmatch(s, -1) {
		col, _ := splitRef(attrs(m[1])["r"])
		segs = append(segs, seg{col, m[0]})
	}
	for _, e := range edits {
		idx := -1
		for i, sg := range segs {
			if sg.col == e.col {
				idx = i
			}
		}
		if idx < 0 {
			segs = append(segs, seg{e.col, buildCell(e.col, rowNum, "", e)})
			continue
		}
		if strings.Contains(segs[idx].xml, "<f>") || strings.Contains(segs[idx].xml, "<f ") {
			return "", fmt.Errorf("cell %s%d contains a formula; refusing to overwrite it", e.col, rowNum)
		}
		style := attrs(cellRe.FindStringSubmatch(segs[idx].xml)[1])["s"]
		segs[idx].xml = buildCell(e.col, rowNum, style, e)
	}
	sort.SliceStable(segs, func(i, j int) bool { return colIndex(segs[i].col) < colIndex(segs[j].col) })
	open = strings.TrimSpace(open)
	if strings.HasSuffix(open, "/>") { // a self-closing <row/> gets a body
		open = strings.TrimSuffix(open, "/>") + ">"
	}
	var sb strings.Builder
	sb.WriteString(open)
	for _, sg := range segs {
		sb.WriteString(sg.xml)
	}
	sb.WriteString("</row>")
	return sb.String(), nil
}
