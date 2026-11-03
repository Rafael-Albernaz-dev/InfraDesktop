package pppoe

import (
	"context"
	"fmt"
	"infradesktop/internal/modules/inventory"
	"testing"
	"time"
)

func TestPPPoEAssociationRequiresUniqueTupleAndUptime(t *testing.T) {
	row := PPRow{
		Login: "synthetic", Caller: "AA:BB:CC:DD:EE:FF", Address: "198.51.100.1",
		Uptime: PPInterval{Known: true, Min: int64(time.Hour), Max: int64(time.Hour + time.Second)},
	}
	tunnel := PPTunnel{
		ID: "*A", Name: "untrusted-name", Login: row.Login, Caller: row.Caller,
		Address: row.Address, Uptime: row.Uptime,
		Context: PPContext{Service: "service", Interface: "vlan100"},
	}
	age := PPInterval{Known: true, Max: int64(2 * time.Second)}
	rows := []PPRow{row}
	AssociatePPTunnels(rows, []PPTunnel{tunnel}, age)
	if rows[0].Context == nil ||
		rows[0].Context.Association != "associated" ||
		rows[0].Context.Service != "service" {
		t.Fatal("unique four-part tuple did not associate", rows)
	}
	changes := []string{
		"duplicate_tunnel", "duplicate_session", "address",
		"caller", "login", "uptime", "unknown",
	}
	for _, change := range changes {
		t.Run(change, func(t *testing.T) {
			assertAssociationRejection(t, row, tunnel, age, change)
		})
	}
}

func assertAssociationRejection(
	t *testing.T, row PPRow, tunnel PPTunnel, age PPInterval, change string,
) {
	t.Helper()
	rows := []PPRow{row}
	ts := []PPTunnel{tunnel}
	switch change {
	case "duplicate_tunnel":
		ts = append(ts, tunnel)
	case "duplicate_session":
		rows = append(rows, row)
	case "address":
		ts[0].Address = "198.51.100.2"
	case "caller":
		ts[0].Caller = "00:00:00:00:00:00"
	case "login":
		ts[0].Login = "another"
	case "uptime":
		ts[0].Uptime.Min = 0
		ts[0].Uptime.Max = 10
	case "unknown":
		ts[0].Uptime.Known = false
	}
	AssociatePPTunnels(rows, ts, age)
	if rows[0].Context != nil && rows[0].Context.Association == "associated" {
		t.Fatal("invented association", change)
	}
}

func TestPPPoEBandwidthUsesRealWindowsAndRejectsResets(t *testing.T) {
	a, b := trafficPair()
	a.ID, b.ID = "*1", "*1"
	download, upload := PPBandwidth(a, b)
	if download == nil || upload == nil ||
		download.Min > 16000.0/2.3 || download.Max < 8000 ||
		upload.Min > 8000.0/2.3 || upload.Max < 4000 {
		t.Fatal("wrong direction or nonconservative interval", download, upload)
	}
	for _, why := range []string{"counter", "one_sample", "identity", "uptime", "router", "overlap"} {
		t.Run(why, func(t *testing.T) {
			assertBandwidthRejection(t, a, b, why)
		})
	}
	b.RX = a.RX
	b.TX = a.TX
	d, u := PPBandwidth(a, b)
	if d == nil || u == nil || d.Min != 0 || d.Max != 0 || u.Max != 0 {
		t.Fatal("explicit unchanged counters lost legitimate zero")
	}
}

func assertBandwidthRejection(t *testing.T, a, b PPTunnel, why string) {
	t.Helper()
	value := func(n uint64) *uint64 { return &n }
	c := b
	switch why {
	case "counter":
		c.TX = value(1)
	case "one_sample":
		c.RX = nil
	case "identity":
		c.ID = "*2"
	case "uptime":
		c.Uptime = PPInterval{Known: true, Max: int64(time.Second)}
	case "router":
		c.RouterUptime = PPInterval{Known: true, Max: int64(time.Second)}
	case "overlap":
		c.Sample = a.Sample
	}
	d, u := PPBandwidth(a, c)
	if d != nil || u != nil {
		t.Fatal("uncertain sample became known rate", why, d, u)
	}
}

func TestPPPoEReadRejectsRouterRestartBeforeFirstSample(t *testing.T) {
	up := PPInterval{Known: true, Min: int64(time.Hour), Max: int64(time.Hour + time.Second)}
	router := PPInterval{
		Known: true, Min: int64(24 * time.Hour), Max: int64(24*time.Hour + time.Second),
	}
	row := PPRow{Login: "synthetic", Caller: "caller", Address: "198.51.100.1", Uptime: up}
	target := &PPReadTarget{
		Observation: PPObservation{Provenance: PPProvenance{RouterUptime: router}},
		Rows:        []PPRow{row},
	}
	tunnel := PPTunnel{
		ID: "*1", Name: "tunnel", Login: row.Login, Caller: row.Caller,
		Address: row.Address, Uptime: up, RouterUptime: router,
	}
	age := PPInterval{Known: true, Max: int64(time.Second)}
	if MatchPPRead(target, []PPTunnel{tunnel}, age)[0].ID == "" {
		t.Fatal("legitimate first sample rejected")
	}
	tunnel.RouterUptime = PPInterval{
		Known: true, Min: int64(10 * time.Second), Max: int64(11 * time.Second),
	}
	if MatchPPRead(target, []PPTunnel{tunnel}, age)[0].ID != "" {
		t.Fatal("first sample associated across router restart")
	}
}

type readClockStore struct {
	Store
	PPStore
	observation *PPObservation
	row         PPRow
}

func (s readClockStore) PPCurrent(context.Context, string, string) (PPView, Code) {
	return PPView{Current: s.observation}, OK
}
func (s readClockStore) PPSession(context.Context, string, PPInput) (PPView, Code) {
	return PPView{Rows: []PPRow{s.row}}, OK
}

func TestPPPoEPreparedAgeHasOneClockOrigin(t *testing.T) {
	device := Device{ID: "00000000-0000-4000-8000-000000000001", Revision: 2}
	id := "00000000-0000-4000-8000-000000000002"
	observation := &PPObservation{
		ID: "observation",
		Provenance: PPProvenance{
			Epoch: "epoch", Connection: PPConnection(device), Revision: 2, CredentialRevision: 1,
			Sample: PPInterval{Known: true, Min: int64(time.Second), Max: int64(2 * time.Second)},
		},
	}
	row := PPRow{
		ID: id, RemoteID: "remote", SessionID: "session",
		Uptime: PPInterval{Known: true, Min: int64(time.Hour), Max: int64(time.Hour + time.Second)},
	}
	inv := &inventory.Service{
		Store:  readClockStore{observation: observation, row: row},
		Origin: time.Now().Add(-time.Hour),
		Epoch:  "epoch",
	}
	s := New(inv, nil)
	target := PPTarget{Device: device}
	target.Credential.Metadata.Revision = 1
	in := PPInput{DeviceID: device.ID, ObservationID: observation.ID, RowIDs: []string{id}}
	reading, c := s.preparePPRead(context.Background(), Command{}, in, target)
	if c != OK {
		t.Fatal(c)
	}
	anchor := reading.Prepared.Sub(inv.Origin).Nanoseconds()
	if reading.Age.Min != anchor-observation.Provenance.Sample.Max ||
		reading.Age.Max != anchor-observation.Provenance.Sample.Min {
		t.Fatal("age upper/lower bounds do not cover the prepared clock origin")
	}
}

func TestPPPoELiveAdmissionHasFiniteSixtySecondBudget(t *testing.T) {
	if !PPReadScope("live") {
		t.Fatal("live read class unavailable")
	}
	if budget := PPReadBudget("live"); budget != 60*time.Second {
		t.Fatal("live class has no bounded60s contract", budget)
	}
	if PPReadBudget("bandwidth") != QueryBudget {
		t.Fatal("short finite reads expanded")
	}
}

func TestPPPoEOperationCancelCannotStopAtCompletedCache(t *testing.T) {
	device := "00000000-0000-4000-8000-000000000001"
	operation := "00000000-0000-4000-8000-000000000002"
	command := Command{Op: "pppoe.read-start", Authority: 1, Operation: operation}
	cancel := command
	cancel.Op = "pppoe.read-cancel"
	for cached := 1; cached < MaxJobs; cached++ {
		calls := 0
		inv := &inventory.Service{Jobs: map[string]*RunningJob{}}
		s := New(inv, nil)
		for i := 0; i < cached; i++ {
			inv.Jobs[fmt.Sprint(i)] = &RunningJob{
				Command: command, Device: Device{ID: device},
				View: Job{State: "SUCCEEDED"}, Finished: time.Now(), Read: &PPReadResult{},
			}
		}
		active := &RunningJob{
			Command: command, Device: Device{ID: device},
			View: Job{State: "RUNNING"}, Cancel: func() { calls++ },
		}
		inv.Jobs["active"] = active
		in := PPInput{DeviceID: device, Operation: operation}
		if c := s.handlePPRead(context.Background(), cancel, in, &PPView{}); c != OK {
			t.Fatal(c)
		}
		if calls != 1 || active.View.State != "CANCELLED" {
			t.Fatal("completed cache hid active operation", cached, calls)
		}
	}
}

func TestPPPoETopConsumptionOrdersKnownLowerBounds(t *testing.T) {
	if !PPReadScope("top") {
		t.Fatal("top consumption unavailable")
	}
	rows := []PPReadRow{
		{Login: "unknown"},
		{Login: "zero", Download: &PPRate{}, Upload: &PPRate{}},
		{Login: "larger", Download: &PPRate{Min: 20, Max: 30}, Upload: &PPRate{Min: 1, Max: 2}},
		{Login: "smaller", Download: &PPRate{Min: 10, Max: 15}, Upload: &PPRate{Min: 1, Max: 2}},
	}
	SortPPConsumption(rows)
	if rows[0].Login != "larger" || rows[1].Login != "smaller" ||
		rows[2].Login != "zero" || rows[3].Login != "unknown" {
		t.Fatal("unknown was ordered as zero or wrong lower bound", rows)
	}
}
