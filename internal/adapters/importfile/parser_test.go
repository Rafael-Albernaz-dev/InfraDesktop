package importfile

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func zipFixture(t *testing.T, sheet string, extra map[string]string) []byte {
	t.Helper()
	files := map[string]string{
		"xl/workbook.xml":            `<workbook xmlns:r="rel"><sheets><sheet name="One" r:id="r1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships><Relationship Id="r1" Target="worksheets/sheet1.xml"/></Relationships>`,
		"xl/worksheets/sheet1.xml":   sheet,
	}
	for k, v := range extra {
		files[k] = v
	}
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for name, content := range files {
		w, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

const sampleSheet = `<worksheet><sheetData>` +
	`<row r="1"><c r="A1" t="inlineStr"><is><t>Nome</t></is></c>` +
	`<c r="B1" t="inlineStr"><is><t>IP</t></is></c><c r="C1" t="inlineStr"><is><t>Porta</t></is></c></row>` +
	`<row r="3"><c r="A3" t="s"><v>0</v></c><c r="B3" t="inlineStr"><is><t>192.0.2.10</t></is></c>` +
	`<c r="C3"><v>22</v></c></row></sheetData></worksheet>`

func TestFormatsAndXLSXCellTypes(t *testing.T) {
	p := Parser{}
	xlsx := zipFixture(t, sampleSheet, map[string]string{"xl/sharedStrings.xml": `<sst><si><t>Roteador</t></si></sst>`})
	cases := []struct {
		name string
		data []byte
		line int
	}{
		{"one.csv", []byte("\xef\xbb\xbfNome;IP;Porta\nRoteador;192.0.2.10;22\n"), 2},
		{"one.csv", []byte("Nome,IP,Porta\nRoteador,192.0.2.10,22\n"), 2},
		{"one.json", []byte(`[{"NAME":"Roteador","address":"192.0.2.10","port":22}]`), 1},
		{"one.xlsx", xlsx, 3},
	}
	for _, tc := range cases {
		rows, err := p.Parse(tc.name, tc.data)
		if err != nil || len(rows) != 1 {
			t.Fatalf("%s: %#v %v", tc.name, rows, err)
		}
		if rows[0].Line != tc.line || rows[0].Name != "Roteador" ||
			rows[0].Address != "192.0.2.10" || rows[0].Port != "22" {
			t.Fatalf("%s: %#v %v", tc.name, rows, err)
		}
	}
}

func TestPasswordKeysRefuseWholeFileWithoutEcho(t *testing.T) {
	secret := "synthetic-never-echo"
	sheet := strings.Replace(sampleSheet, "Nome", "SeNhA", 1)
	cases := []struct {
		name string
		data []byte
	}{
		{"one.csv", []byte("Nome,PassWord\nrouter," + secret)},
		{"one.json", []byte(`[{"name":"router","nested":{"pWd":"` + secret + `"}}]`)},
		{"one.xlsx", zipFixture(t, sheet, map[string]string{"xl/sharedStrings.xml": `<sst><si><t>` + secret + `</t></si></sst>`})},
	}
	for _, tc := range cases {
		rows, err := (Parser{}).Parse(tc.name, tc.data)
		if err == nil || len(rows) != 0 || strings.Contains(err.Error(), secret) {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
}

func TestLimitsAndHostileZip(t *testing.T) {
	p := Parser{}
	csv := "Nome,IP\n" + strings.Repeat("router,192.0.2.1\n", 500)
	if rows, err := p.Parse("one.csv", []byte(csv)); err != nil || len(rows) != 500 {
		t.Fatal("500 rows", len(rows), err)
	}
	if _, err := p.Parse("one.csv", []byte(csv+"extra,192.0.2.2\n")); err == nil {
		t.Fatal("501 accepted")
	}
	if _, err := p.Parse("one.csv", bytes.Repeat([]byte{'x'}, maxFile+1)); err == nil {
		t.Fatal("oversized file accepted")
	}
	for _, extra := range []map[string]string{
		{"../escape": "x"}, {"xl/bad.xml": `<!DOCTYPE x [<!ENTITY e "bad">]><x>&e;</x>`},
		{"xl/huge.txt": strings.Repeat("0", maxExpanded+1)},
	} {
		if _, err := p.Parse("one.xlsx", zipFixture(t, sampleSheet, extra)); err == nil {
			t.Fatal("hostile zip accepted")
		}
	}
	extra := map[string]string{}
	for i := 0; i < maxZipEntries; i++ {
		extra[fmt.Sprintf("f%d", i)] = ""
	}
	if _, err := p.Parse("one.xlsx", zipFixture(t, sampleSheet, extra)); err == nil {
		t.Fatal("too many entries accepted")
	}
	formula := strings.Replace(sampleSheet, `<v>22</v>`, `<f>1+1</f><v>22</v>`, 1)
	if _, err := p.Parse("one.xlsx", zipFixture(t, formula, nil)); err == nil {
		t.Fatal("formula accepted")
	}
}

func TestCSVPhysicalLineAfterQuotedMultiline(t *testing.T) {
	data := []byte("Nome,IP\n\"roteador\nexemplo\",192.0.2.1\nsegundo,192.0.2.2\n")
	rows, err := (Parser{}).Parse("one.csv", data)
	if err != nil || len(rows) != 2 || rows[0].Line != 2 || rows[1].Line != 4 {
		t.Fatalf("physical line numbers: %#v %v", rows, err)
	}
}
