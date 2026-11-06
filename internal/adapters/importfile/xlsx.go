package importfile

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"path"
	"strconv"
	"strings"

	"infradesktop/internal/modules/inventory"
)

const maxExpanded = 32 * 1024 * 1024
const maxZipEntries = 1000

type workbook struct {
	Sheets []struct {
		ID string `xml:"id,attr"`
	} `xml:"sheets>sheet"`
}
type relationships struct {
	Items []struct {
		ID     string `xml:"Id,attr"`
		Target string `xml:"Target,attr"`
	} `xml:"Relationship"`
}
type textItem struct {
	Text string `xml:"t"`
	Runs []struct {
		Text string `xml:"t"`
	} `xml:"r"`
}
type sharedStrings struct {
	Items []textItem `xml:"si"`
}
type sheetCell struct {
	Ref     string   `xml:"r,attr"`
	Type    string   `xml:"t,attr"`
	Value   string   `xml:"v"`
	Formula *string  `xml:"f"`
	Inline  textItem `xml:"is"`
}
type worksheet struct {
	Rows []struct {
		Number int         `xml:"r,attr"`
		Cells  []sheetCell `xml:"c"`
	} `xml:"sheetData>row"`
}

func safeZipPath(name string) bool {
	if name == "" || strings.ContainsAny(name, "\\:\x00") || strings.HasPrefix(name, "/") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == ".." || part == "." {
			return false
		}
	}
	return true
}

func validateXML(data []byte) error {
	d := xml.NewDecoder(bytes.NewReader(data))
	for {
		t, err := d.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return errFile
		}
		if _, ok := t.(xml.Directive); ok {
			return errFile
		}
	}
}

func unpack(data []byte) (map[string][]byte, error) {
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil || len(z.File) > maxZipEntries {
		return nil, errFile
	}
	files := map[string][]byte{}
	var expanded int64
	for _, f := range z.File {
		if !safeZipPath(f.Name) || f.UncompressedSize64 > maxExpanded || expanded+int64(f.UncompressedSize64) > maxExpanded {
			return nil, errFile
		}
		if f.FileInfo().IsDir() {
			continue
		}
		if _, ok := files[f.Name]; ok {
			return nil, errFile
		}
		body, err := readZipEntry(f, maxExpanded-expanded)
		if err != nil {
			return nil, err
		}
		expanded += int64(len(body))
		files[f.Name] = body
	}
	return files, nil
}

func readZipEntry(f *zip.File, remaining int64) ([]byte, error) {
	r, err := f.Open()
	if err != nil {
		return nil, errFile
	}
	body, readErr := io.ReadAll(io.LimitReader(r, remaining+1))
	closeErr := r.Close()
	if readErr != nil || closeErr != nil || int64(len(body)) > remaining {
		return nil, errFile
	}
	name := strings.ToLower(f.Name)
	if strings.HasSuffix(name, ".xml") || strings.HasSuffix(name, ".rels") {
		if validateXML(body) != nil {
			return nil, errFile
		}
	}
	return body, nil
}

func firstSheet(files map[string][]byte) ([]byte, error) {
	var book workbook
	var rel relationships
	if xml.Unmarshal(files["xl/workbook.xml"], &book) != nil || len(book.Sheets) == 0 ||
		xml.Unmarshal(files["xl/_rels/workbook.xml.rels"], &rel) != nil {
		return nil, errFile
	}
	for _, item := range rel.Items {
		if item.ID != book.Sheets[0].ID {
			continue
		}
		target := strings.TrimPrefix(item.Target, "/")
		if !safeZipPath(target) {
			return nil, errFile
		}
		if !strings.HasPrefix(target, "xl/") {
			target = path.Join("xl", target)
		}
		body, ok := files[target]
		if !ok {
			return nil, errFile
		}
		return body, nil
	}
	return nil, errFile
}

func textValue(t textItem) string {
	var b strings.Builder
	b.WriteString(t.Text)
	for _, r := range t.Runs {
		b.WriteString(r.Text)
	}
	return b.String()
}

func cellColumn(ref string) int {
	n := 0
	for _, c := range ref {
		if c >= 'A' && c <= 'Z' {
			n = n*26 + int(c-'A'+1)
			if n > 16384 {
				return -1
			}
			continue
		}
		break
	}
	return n - 1
}

func parseXLSX(data []byte) ([]inventory.ImportRow, error) {
	files, err := unpack(data)
	if err != nil {
		return nil, err
	}
	body, err := firstSheet(files)
	if err != nil {
		return nil, err
	}
	var shared sharedStrings
	if data, ok := files["xl/sharedStrings.xml"]; ok && xml.Unmarshal(data, &shared) != nil {
		return nil, errFile
	}
	var sheet worksheet
	if xml.Unmarshal(body, &sheet) != nil || len(sheet.Rows) == 0 || len(sheet.Rows)-1 > maxRows {
		return nil, errFile
	}
	records, err := sheetRecords(sheet, shared)
	if err != nil {
		return nil, err
	}
	rows, err := rowsFromTable(records)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		if sheet.Rows[i+1].Number > 0 {
			rows[i].Line = sheet.Rows[i+1].Number
		}
	}
	return rows, nil
}

func sheetRecords(sheet worksheet, shared sharedStrings) ([][]string, error) {
	records := make([][]string, 0, len(sheet.Rows))
	for _, row := range sheet.Rows {
		record := []string{}
		for _, c := range row.Cells {
			col := cellColumn(c.Ref)
			if col < 0 || col >= 256 {
				return nil, errFile
			}
			value, err := decodeCell(c, shared)
			if err != nil {
				return nil, err
			}
			for len(record) <= col {
				record = append(record, "")
			}
			record[col] = value
		}
		records = append(records, record)
	}
	return records, nil
}

func decodeCell(c sheetCell, shared sharedStrings) (string, error) {
	if c.Formula != nil {
		return "", errFile
	}
	switch c.Type {
	case "s":
		n, err := strconv.Atoi(c.Value)
		if err != nil || n < 0 || n >= len(shared.Items) {
			return "", errFile
		}
		return textValue(shared.Items[n]), nil
	case "inlineStr":
		return textValue(c.Inline), nil
	case "", "n", "str":
		return c.Value, nil
	default:
		return "", errFile
	}
}
