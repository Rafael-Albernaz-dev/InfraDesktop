package routerssh_test

import (
	"context"
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/modules/equipment"
	"time"
)

// Recovery scenarios control the unrelated ICMP dependency so a legitimate ping
// abort cannot prevent the SSH/pin condition under test. Actual ICMP windows are
// covered by TestRadioPersistentICMPReportsActualWindow; SSH remains real here.
type recoverySSHRemote struct{ routerssh.Adapter }
type recoveryProbeSession struct{ equipment.RadioReadSession }

func (r *recoverySSHRemote) OpenRadioSession(ctx context.Context, d equipment.Device, u, p string,
	family equipment.RadioFamily, link *equipment.RadioLink, check func() equipment.Code) (equipment.RadioReadSession, equipment.Code) {
	session, code := r.Adapter.OpenRadioSession(ctx, d, u, p, family, link, check)
	if code != equipment.OK {
		return nil, code
	}
	return &recoveryProbeSession{RadioReadSession: session}, equipment.OK
}
func (s *recoveryProbeSession) Probe() (equipment.RadioLoad, equipment.Code) {
	now := time.Now()
	return equipment.RadioLoad{Started: now.Add(-time.Millisecond), Ended: now,
		PingSent: 1, PingReceived: 1, PingRTT: []float64{1}}, equipment.OK
}
