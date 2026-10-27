package equipment

import (
	"testing"
	"time"
)

func TestPPDiagnosticRelationsAreConfigurationOnly(t *testing.T) {
	rows := []PDRow{{ID: "i1", Kind: "interface", Name: "ether"}, {ID: "i2", Kind: "interface", Name: "outer"}, {ID: "i3", Kind: "interface", Name: "inner"}, {ID: "v1", Kind: "vlan", Name: "outer", Parent: "ether", VLANID: 100}, {ID: "v2", Kind: "vlan", Name: "inner", Parent: "outer", VLANID: 200}, {ID: "s1", Kind: "service", Name: "service-a", Parent: "inner"}, {ID: "s2", Kind: "service", Name: "service-b", Parent: "inner"}, {ID: "missing", Kind: "service", Name: "missing", Parent: "absent"}}
	ResolvePDRelations(rows)
	if rows[4].Relation != "resolved" || rows[4].ParentID != "i2" || rows[5].Relation != "resolved" || rows[6].ParentID != rows[5].ParentID || rows[7].Relation != "missing" {
		t.Fatal(rows)
	}
	duplicate := append([]PDRow{}, rows...)
	duplicate = append(duplicate, PDRow{ID: "homonym", Kind: "interface", Name: "inner"})
	ResolvePDRelations(duplicate)
	if duplicate[5].Relation != "ambiguous" || duplicate[5].ParentID != "" {
		t.Fatal("homonym bound", duplicate[5])
	}
	rows[3].Parent = "inner"
	ResolvePDRelations(rows)
	if rows[4].Relation != "cycle" || rows[5].Relation != "cycle" {
		t.Fatal("cycle bound", rows)
	}
}

func TestPPDiagnosticPingIdentityIsOpaqueAndConservative(t *testing.T) {
	interval := func(seconds int64) PPInterval {
		return PPInterval{Known: true, Min: seconds * int64(time.Second), Max: (seconds+1)*int64(time.Second) - 1}
	}
	target := &PPPingTarget{Observation: PPObservation{Provenance: PPProvenance{Version: "7.26", Family: "v7", RouterUptime: interval(10000)}}, Row: PPRow{RemoteID: "opaque protocol identifier", SessionID: "opaque session token", Address: "198.51.100.1", Uptime: interval(500)}, Age: PPInterval{Known: true, Min: 0, Max: int64(time.Millisecond)}}
	fresh := &PPObservation{Quality: PPQuality{Status: "complete"}, Provenance: target.Observation.Provenance, Rows: []PPRow{target.Row}}
	if !PPPingStable(target, fresh, time.Millisecond) {
		t.Fatal("domain required a RouterOS grammar instead of opaque identity")
	}
	changed := *fresh
	changed.Rows = append([]PPRow{}, fresh.Rows...)
	changed.Rows[0].SessionID = "different"
	if PPPingStable(target, &changed, time.Millisecond) {
		t.Fatal("session reuse accepted")
	}
	changed.Rows = append([]PPRow{}, fresh.Rows...)
	changed.Rows = append(changed.Rows, PPRow{RemoteID: "other", SessionID: "other", Address: target.Row.Address, Uptime: interval(500)})
	if PPPingStable(target, &changed, time.Millisecond) {
		t.Fatal("nonunique address accepted")
	}
	target.Age = PPInterval{Known: true, Min: int64(120 * time.Second), Max: int64(120*time.Second + time.Millisecond)}
	fresh.Provenance.RouterUptime = interval(10120)
	fresh.Rows[0].Uptime = interval(620)
	if !PPPingStable(target, fresh, 0) {
		t.Fatal("valid long interval refused by former60s window")
	}
	for _, address := range []string{"127.0.0.1", "0.0.0.0", "224.0.0.1", "169.254.1.1", "fe80::1", "example.test", "198.51.100.1;ping"} {
		if _, ok := PPPingAddress(PPRow{Address: address}); ok {
			t.Fatal("noncanonical probe target", address)
		}
	}
	if address, ok := PPPingAddress(PPRow{Address: "::ffff:198.51.100.1"}); !ok || address != "198.51.100.1" {
		t.Fatal("mapped IPv4 not canonical", address, ok)
	}
}
