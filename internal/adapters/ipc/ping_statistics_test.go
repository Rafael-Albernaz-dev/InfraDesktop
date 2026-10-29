package ipc

import (
	"github.com/google/uuid"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/foundation"
	"infradesktop/internal/testsupport/sshfixture"
	"testing"
)

func TestL06T3PingPreviousTenThroughRealIPC(t *testing.T) {
	j := newPPJourney(t, sshfixture.Config{RealTime: true, PPRows: 1})
	o := j.collect(uuid.NewString())
	row := j.call("pppoe.sessions-list", equipment.PPInput{DeviceID: j.device, ObservationID: o.ID}, foundation.OK).Data.PPPoE.Rows[0]
	for i := 0; i < 11; i++ {
		d := j.diagnostic(equipment.PPInput{DeviceID: j.device, Revision: 2, Scope: equipment.PDPing, Operation: uuid.NewString(), ObservationID: o.ID, RowID: row.ID}, equipment.OK)
		v := j.call("pppoe.diagnostic-list", equipment.PPInput{DeviceID: j.device, Scope: equipment.PDPing, DiagnosticID: d.ID}, foundation.OK).Data.PPPoE
		p := v.DiagnosticRows[0].Ping
		if p == nil || p.Jitter == nil || *p.Jitter != 59000 || p.BaselineSamples != min(i, 10) {
			t.Fatal("jitter/history did not cross job/store/IPC", i, p)
		}
		if i < 10 && p.Baseline != nil {
			t.Fatal("fewer than10 claimed baseline", i)
		}
		if i == 10 && (p.Baseline == nil || *p.Baseline != 1099000) {
			t.Fatal("median of previous10 missing", p)
		}
	}
	if j.peer.PingCalls.Load() != 11 {
		t.Fatal("hidden probe/retry", j.peer.PingCalls.Load())
	}
}
