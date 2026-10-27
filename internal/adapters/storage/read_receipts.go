package storage

import (
	"database/sql"
	"fmt"
	"infradesktop/internal/modules/access"
	"strconv"
	"strings"
)

// Only byte-for-byte canonical stamps produced by the historical service are
// removable. Unknown or malformed stamps are conservatively retained.
func readReceiptClass(stamp string) string {
	for _, prefix := range []string{"ppref:", "ppdiag:"} {
		if !strings.HasPrefix(stamp, prefix+"[") {
			continue
		}
		rest := stamp[len(prefix)+1:]
		parts := []string{}
		for strings.HasPrefix(rest, "\"") {
			quoted, e := strconv.QuotedPrefix(rest)
			if e != nil {
				return ""
			}
			v, e := strconv.Unquote(quoted)
			if e != nil {
				return ""
			}
			parts = append(parts, v)
			rest = strings.TrimLeft(rest[len(quoted):], " ")
		}
		if rest != "]" || fmt.Sprintf(prefix+"%q", parts) != stamp || len(parts) < 3 || !access.ValidCompanyID(parts[1]) {
			return ""
		}
		rev, e := strconv.ParseUint(parts[2], 10, 64)
		if e != nil || rev == 0 || strconv.FormatUint(rev, 10) != parts[2] {
			return ""
		}
		if prefix == "ppref:" && len(parts) == 5 && parts[0] == "pppoe.collect-start" && parts[3] == "" && parts[4] == "" {
			return "sessions"
		}
		if prefix == "ppdiag:" && len(parts) == 6 && parts[0] == "pppoe.diagnostic-start" {
			if (parts[3] == "catalog" || parts[3] == "radius") && parts[4] == "" && parts[5] == "" {
				return parts[3]
			}
			if parts[3] == "ping" && access.ValidCompanyID(parts[4]) && access.ValidCompanyID(parts[5]) {
				return "ping"
			}
		}
		return ""
	}
	var op, id string
	var revision, credential uint64
	if n, e := fmt.Sscanf(stamp, "job:%q:%d:%q:%d", &op, &revision, &id, &credential); e == nil && n == 4 && access.ValidCompanyID(id) && revision > 0 && fmt.Sprintf("job:%q:%d:%q:%d", op, revision, id, credential) == stamp {
		if op == "device.query-start" {
			return "summary"
		}
		if op == "trust.inspect" {
			return "trust-candidate"
		}
	}
	return ""
}

func migrateReadReceipts(tx *sql.Tx) error {
	rows, e := tx.Query("SELECT id,stamp FROM equipment_operations")
	if e != nil {
		return e
	}
	ids := []string{}
	counts := map[string]int{"sessions": 0, "catalog": 0, "radius": 0, "ping": 0, "summary": 0, "trust-candidate": 0}
	for rows.Next() {
		var id, stamp string
		if e = rows.Scan(&id, &stamp); e != nil {
			rows.Close()
			return e
		}
		if class := readReceiptClass(stamp); class != "" {
			ids = append(ids, id)
			counts[class]++
		}
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, id := range ids {
		if _, e = tx.Exec("DELETE FROM equipment_operations WHERE id=?", id); e != nil {
			return e
		}
	}
	for class, n := range counts {
		if _, e = tx.Exec("INSERT INTO equipment_read_receipt_migration VALUES(?,?)", class, n); e != nil {
			return e
		}
	}
	return nil
}
