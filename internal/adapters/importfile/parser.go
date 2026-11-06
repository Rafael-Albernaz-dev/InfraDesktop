package importfile

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"infradesktop/internal/modules/inventory"
)

const maxFile = 2 * 1024 * 1024
const maxRows = 500

var errFile = errors.New("arquivo inválido ou excede o limite")
var errPassword = errors.New("arquivo contém campo de senha")

type Parser struct{}

func (Parser) Parse(name string, data []byte) ([]inventory.ImportRow, error) {
	if len(data) > maxFile {
		return nil, errFile
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".csv":
		return parseCSV(data)
	case ".json":
		return parseJSON(data)
	case ".xlsx":
		return parseXLSX(data)
	default:
		return nil, errFile
	}
}

func passwordKey(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "senha", "password", "secret", "pass", "pwd":
		return true
	default:
		return false
	}
}

func keyName(s string) string {
	s = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(s, "\ufeff")))
	switch s {
	case "nome", "name":
		return "name"
	case "ip", "address":
		return "address"
	case "porta", "port":
		return "port"
	case "papel", "role":
		return "role"
	case "bloco", "block":
		return "block"
	case "credencial", "credential":
		return "credential"
	default:
		return ""
	}
}

func rowsFromTable(records [][]string) ([]inventory.ImportRow, error) {
	if len(records) == 0 || len(records)-1 > maxRows {
		return nil, errFile
	}
	for _, h := range records[0] {
		if passwordKey(h) {
			return nil, errPassword
		}
	}
	out := make([]inventory.ImportRow, 0, len(records)-1)
	for i, record := range records[1:] {
		m := make(map[string]string)
		for n, h := range records[0] {
			if n < len(record) {
				m[keyName(h)] = record[n]
			}
		}
		out = append(out, rowFromMap(i+2, m))
	}
	return out, nil
}

func rowFromMap(line int, m map[string]string) inventory.ImportRow {
	return inventory.ImportRow{Line: line, Name: m["name"], Address: m["address"], Port: m["port"],
		Role: m["role"], Block: m["block"], Credential: m["credential"]}
}

func parseCSV(data []byte) ([]inventory.ImportRow, error) {
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	reader := csv.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = -1
	first, _, _ := bytes.Cut(data, []byte("\n"))
	if bytes.Count(first, []byte(";")) > bytes.Count(first, []byte(",")) {
		reader.Comma = ';'
	}
	records := [][]string{}
	lines := []int{}
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil || len(records) > maxRows {
			return nil, errFile
		}
		line, _ := reader.FieldPos(0)
		lines = append(lines, line)
		records = append(records, record)
	}
	rows, err := rowsFromTable(records)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].Line = lines[i+1]
	}
	return rows, nil
}

func hasPassword(v any) bool {
	switch data := v.(type) {
	case map[string]any:
		for k, child := range data {
			if passwordKey(k) || hasPassword(child) {
				return true
			}
		}
	case []any:
		for _, child := range data {
			if hasPassword(child) {
				return true
			}
		}
	}
	return false
}

func parseJSON(data []byte) ([]inventory.ImportRow, error) {
	decoder := json.NewDecoder(bytes.NewReader(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})))
	decoder.UseNumber()
	var root any
	if decoder.Decode(&root) != nil {
		return nil, errFile
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return nil, errFile
	}
	if hasPassword(root) {
		return nil, errPassword
	}
	items, ok := root.([]any)
	if !ok || len(items) > maxRows {
		return nil, errFile
	}
	out := make([]inventory.ImportRow, 0, len(items))
	for i, item := range items {
		object, ok := item.(map[string]any)
		if !ok {
			return nil, errFile
		}
		fields := map[string]string{}
		for k, value := range object {
			switch v := value.(type) {
			case string:
				fields[keyName(k)] = v
			case json.Number:
				fields[keyName(k)] = v.String()
			case nil:
			default:
				if keyName(k) != "" {
					return nil, fmt.Errorf("%w", errFile)
				}
			}
		}
		out = append(out, rowFromMap(i+1, fields))
	}
	return out, nil
}
