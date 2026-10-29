package routerssh

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Candidates only: not called by the application. Human outputs on both pilot
// firmwares are required before either reading enters a runtime collector.
const t3FastTrackCandidate = DeviceFastTrackCommand

const t3PortsCandidate = DevicePortsCommand

func TestL06T3DiagnosticSyntax(t *testing.T) {
	for name, body := range map[string]string{"fasttrack-readonly": t3FastTrackCandidate, "ports-readonly": t3PortsCandidate} {
		want := map[string]string{"fasttrack-readonly": "1528f8f5d4635f4401f9d21e80fee2596f68462c8d68ee3048170a6aec41ba96", "ports-readonly": "36b5341984112a15f8c50401f0925486ecb39528ab1fbe48f5a7a4a962526fab"}[name]
		if fmt.Sprintf("%x", sha256.Sum256([]byte(body+"\n"))) != want {
			t.Fatal("runtime command differs from human-executed .rsc", name)
		}
		if h, d := collectorSyntaxHazards(body); h || d {
			t.Fatalf("%s: raw health=%v raw dollar=%v", name, h, d)
		}
		if out := os.Getenv("L06_T3_DIAGNOSTIC_DIR"); out != "" {
			if e := os.MkdirAll(out, 0700); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(filepath.Join(out, name+".rsc"), []byte(body+"\n"), 0600); e != nil {
				t.Fatal(e)
			}
		}
	}
}
