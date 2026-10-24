package ipc

import (
	"context"
	"encoding/json"
	"infradesktop/internal/modules/foundation"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Exercise the same AccountClient -> transport -> framed Unix reply as bindings.
func TestFocalAccountClientEquipmentCodes(t *testing.T) {
	b, e := os.ReadFile("schema-v1.json")
	if e != nil {
		t.Fatal(e)
	}
	var schema struct {
		Equipment struct {
			Codes []foundation.Code `json:"error_codes"`
		} `json:"equipment"`
	}
	if e = json.Unmarshal(b, &schema); e != nil || len(schema.Equipment.Codes) != 23 {
		t.Fatal("equipment error schema drift", e)
	}
	for _, code := range append(schema.Equipment.Codes, foundation.Code("UNKNOWN_FUTURE_CODE")) {
		t.Run(string(code), func(t *testing.T) {
			dir := t.TempDir()
			if e := os.Chmod(dir, 0700); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(filepath.Join(dir, "capability"), []byte(strings.Repeat("a", 64)), 0600); e != nil {
				t.Fatal(e)
			}
			l, e := net.ListenUnix("unix", &net.UnixAddr{Name: filepath.Join(dir, "service.sock"), Net: "unix"})
			if e != nil {
				t.Fatal(e)
			}
			defer l.Close()
			if e = os.Chmod(filepath.Join(dir, "service.sock"), 0600); e != nil {
				t.Fatal(e)
			}
			done := make(chan error, 1)
			go func() {
				c, e := l.Accept()
				if e != nil {
					done <- e
					return
				}
				defer c.Close()
				b, e := readFrame(c)
				if e != nil {
					done <- e
					return
				}
				var q Request
				if e = json.Unmarshal(b, &q); e == nil {
					e = writeFrame(c, Response{Version: 1, RequestID: q.RequestID, Code: code})
				}
				done <- e
			}()
			client := NewAccountClient(dir, t.TempDir(), uint32(os.Getuid()))
			r := client.Call(context.Background(), Request{Op: "device.query-start", Equipment: &EquipmentInput{}})
			want := code
			if code == "UNKNOWN_FUTURE_CODE" {
				want = foundation.Unavailable
			}
			if r.Code != want {
				t.Fatalf("real AccountClient lost code: got %s want %s", r.Code, want)
			}
			if e := <-done; e != nil {
				t.Fatal(e)
			}
		})
	}
}
