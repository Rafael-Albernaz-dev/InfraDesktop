package pulse

import "testing"

func TestOnlineRecoveryStartsANewPeriodAfterAnObservedOutage(t *testing.T) {
	tr := NewTracker()
	tr.Sync([]Target{a, b})
	tr.Record(a, Reply, at(0))
	for i := 1; i <= 3; i++ {
		tr.Record(a, NoReply, at(i*10))
		tr.Record(b, Reply, at(i*10+1))
	}
	if d := tr.Snapshot(at(31)).Devices[a.ID]; d.State != Offline || !d.Since.Equal(at(30)) {
		t.Fatal("outage not observed", d)
	}
	tr.Record(a, Reply, at(40))
	if d := tr.Snapshot(at(40)).Devices[a.ID]; d.State != Online || !d.Since.Equal(at(40)) {
		t.Fatal("new online period must start at recovery", d)
	}
}
