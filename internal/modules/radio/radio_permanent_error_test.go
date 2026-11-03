package radio

import (
	"context"
	"infradesktop/internal/modules/access"
	"strings"
	"testing"
)

type radioFatalRemote struct {
	*radioLiveRemote
	code Code
}
type radioFatalSession struct {
	RadioReadSession
	code Code
}

func (r radioFatalRemote) OpenRadioSession(ctx context.Context,
	d Device,
	u,
	p string,
	f RadioFamily,
	l *RadioLink,
	check func() Code) (RadioReadSession,
	Code) {
	s, c := r.radioLiveRemote.OpenRadioSession(ctx, d, u, p, f, l, check)
	code := OK
	if d.Role == RoleAP {
		code = r.code
	}
	return radioFatalSession{s, code}, c
}
func (s radioFatalSession) Tick() (*RadioReading, Code) {
	if s.code != OK {
		return nil, s.code
	}
	return s.RadioReadSession.Tick()
}
func TestRadioPermanentErrorsNeverReopen(t *testing.T) {
	for _, code := range []Code{TrustChanged,
		access.Unauthorized,
		RadioCodeRemoteAuthFailed,
		CollectorDefect,
		RadioCodeLinkUnconfirmed,
		"REMOTE_PERMISSION_DENIED"} {
		t.Run(string(code), func(t *testing.T) {
			remote := &radioLiveRemote{}
			s, q, in, _ := radioHarness(t, remote)
			s.Remote = radioFatalRemote{remote, code}
			v, c := s.HandleRadio(context.Background(), q, in)
			if c != OK {
				t.Fatal(c)
			}
			s.WG.Wait()
			s.Mu.Lock()
			j := s.Jobs[v.Job.ID]
			actual := j.View.Code
			reason := j.Radio.Cost.Reason
			s.Mu.Unlock()
			if actual != code || remote.opens.Load() != 2 || remote.closes.Load() != 2 || !strings.Contains(reason, string(code)) {
				t.Fatalf(("permanent failure was retried/reclassified: code=%s want=%s opens=%d closes=%d" +
					" reason=%s"),
					actual,
					code,
					remote.opens.Load(),
					remote.closes.Load(),
					reason)
			}
		})
	}
}
