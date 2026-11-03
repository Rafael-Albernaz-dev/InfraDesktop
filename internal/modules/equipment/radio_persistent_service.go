package equipment

import (
	"context"
	"fmt"
)

func (s *Service) openRadioSide(ctx context.Context,
	j *runningJob,
	d Device,
	credential uint64,
	family RadioFamily,
	link *RadioLink) (RadioReadSession,
	Code) {
	remote, ok := s.remote.(RadioSessionRemote)
	if !ok {
		return nil, RadioCodeLoadUnavailable
	}
	copy := runningJob{command: j.command, device: d, credentialRevision: credential}
	check := func() Code {
		row, c := s.radioRow(ctx, j.command, j.radioInput)
		if c != OK {
			return c
		}
		if d.Role == RoleCPE && (row == nil || d.Address != row.Address) {
			return RadioCodeCPEUnreachable
		}
		_, _, c = s.check(ctx, &copy, false)
		return c
	}
	if c := check(); c != OK {
		return nil, c
	}
	user, secret, c := s.check(ctx, &copy, true)
	if c != OK {
		return nil, c
	}
	session, c := remote.OpenRadioSession(ctx, d, user, secret, family, link, check)
	secret = ""
	return session, c
}

func radioTickReason(ap, cpe *RadioReading, cost RadioCost, apOnly bool) string {
	if ap == nil {
		return "Leitura do AP indisponível"
	}
	if ap.CommandSeconds > 3 {
		return fmt.Sprintf("Comando do AP levou %.2f s (limite 3 s)", ap.CommandSeconds)
	}
	if !apOnly && cpe == nil {
		return "Leitura da CPE indisponível"
	}
	if cpe != nil && cpe.CommandSeconds > 3 {
		return fmt.Sprintf("Comando da CPE levou %.2f s (limite 3 s)", cpe.CommandSeconds)
	}
	for _, cpu := range ap.CPU {
		if cpu > RadioMaxCPU {
			return fmt.Sprintf("CPU do AP chegou a %d%% (limite 90%%)", cpu)
		}
	}
	if ap.PingRTT == nil {
		return "ICMP indisponível"
	}
	if *ap.PingRTT > cost.PingLimit {
		return fmt.Sprintf("Ping %.2f ms acima de 2× a mediana (%.2f ms)", *ap.PingRTT, cost.PingLimit)
	}
	return "Carga incompleta"
}

func radioTickComplete(ap, cpe *RadioReading, load RadioLoad, apOnly bool) bool {
	if ap == nil || (!apOnly && cpe == nil) || len(ap.CPU) == 0 {
		return false
	}
	if !completeRadioPing(load) {
		return false
	}
	readings := []*RadioReading{ap}
	if !apOnly {
		readings = append(readings, cpe)
	}
	for _, r := range readings {
		if !radioFinite(r.CommandSeconds) || r.CommandSeconds <= 0 || r.CommandSeconds > 3 {
			return false
		}
	}
	for _, cpu := range ap.CPU {
		if cpu < 0 || cpu > 100 {
			return false
		}
	}

	return true
}

// Only transport/read loss is transient. Authorization, pin, permission,
// parser defects, link identity, limits and cancellation keep their real code.
func radioTransientTick(c Code) bool {
	switch c {
	case RadioCodeDeadlineExceeded, RadioCodeSSHConnectionLost, RadioCodeConnectFailed, Partial:
		return true
	}
	return false
}

func completeRadioPing(load RadioLoad) bool {
	if load.PingSent < 1 || load.PingReceived != load.PingSent || len(load.PingRTT) != load.PingReceived {
		return false
	}
	for _, rtt := range load.PingRTT {
		if !radioFinite(rtt) {
			return false
		}
	}
	return true
}
