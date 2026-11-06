package presentation

import (
	"bytes"
	"encoding/xml"
	"fmt"
)

func writeTemplateRow(out *bytes.Buffer, number int, values []string) {
	fmt.Fprintf(out, `<row r="%d">`, number)
	for i, value := range values {
		fmt.Fprintf(out, `<c r="%c%d" t="inlineStr"><is><t>`, 'A'+i, number)
		_ = xml.EscapeText(out, []byte(value))
		out.WriteString(`</t></is></c>`)
	}
	out.WriteString(`</row>`)
}

func templateXLSXParts() map[string]string {
	return map[string]string{
		"[Content_Types].xml": `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
			`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
			`<Default Extension="xml" ContentType="application/xml"/>` +
			`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>` +
			`<Override PartName="/xl/worksheets/sheet1.xml" ` +
			`ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>`,
		"_rels/.rels": `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="r1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" ` +
			`Target="xl/workbook.xml"/></Relationships>`,
		"xl/workbook.xml": `<?xml version="1.0"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" ` +
			`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
			`<sheets><sheet name="Equipamentos" sheetId="1" r:id="r1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="r1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" ` +
			`Target="worksheets/sheet1.xml"/></Relationships>`,
	}
}
