package equipment

import (
	"context"
)

func (run *persistentRadioRun) reopenFailedSides(tick *persistentRadioTick) {
	ap := tick.ap
	apCode := tick.apCode
	cpe := tick.cpe
	cpeCode := tick.cpeCode
	for i, side := range []struct {
		reading *RadioReading
		code    Code
	}{{ap, apCode}, {cpe, cpeCode}} {
		if i >= len(run.sessions) {
			break
		}
		if side.code == OK && (side.reading == nil || side.reading.CommandSeconds <= 3) {
			continue
		}
		run.sessions[i].Close()
		run.sessions[i] = nil
		// The next normal tick performs revalidation and establishes its
		// baseline. It remains in the measured run.series and CPU/ping streaks;
		// no extra hidden sample is executed during reopening.
		next, c := run.service.openRadioSide(run.ctx, run.j, run.j.radioDevices[i], run.revisions[i], run.families[i], run.link)

		if c != OK {
			if next != nil {
				next.Close()
			}
			run.code = c
			name := "AP"
			if i == 1 {
				name = "CPE"
			}
			run.reason = name + " na reabertura: " + string(c)
			if run.failure != nil {
				run.reason += " (" + run.failure.Reason + ")"
			}
			if i == 0 {
				run.view.APCode = c
			} else {
				run.view.CPECode = c
			}
			break
		}
		run.sessions[i] = next
		if i == 0 {
			run.previousAP = nil
		} else {
			run.previousCPE = nil
		}
	}

}

func (run *persistentRadioRun) cacheCost() {
	if run.j.radioInput.Mode == RadioMeasure && len(run.measurements) == 10 && run.code == OK && run.ctx.Err() == nil {
		run.service.mu.Lock()
		if run.service.radioCosts == nil {
			run.service.radioCosts = map[string]RadioCost{}
		}
		if _, ok := run.service.radioCosts[run.key]; ok || len(run.service.radioCosts) < MaxDevices {
			run.service.radioCosts[run.key] = run.cost
		} else {
			run.cost.Enabled = false
			run.cost.Reason = "Limite de medições desta execução atingido."
		}
		run.service.mu.Unlock()
	}

}

func (run *persistentRadioRun) finish() {
	if run.ctx.Err() != nil {
		run.code = Cancelled
		if run.ctx.Err() == context.DeadlineExceeded {
			run.code = RadioCodeWindowEnded
		}
	}
	select {
	case c := <-run.authFailure:
		run.code = c
		run.reason = string(c)
	default:
	}
	if run.reason != "" {
		run.cost.Reason = run.reason
	}
	finalCost := run.cost
	run.view.Cost = &finalCost
	run.service.mu.Lock()
	defer run.service.mu.Unlock()
	if run.j.view.State == "CANCELLED" {
		run.code = Cancelled
	}
	copy := run.view
	run.j.radio = &copy
	run.j.view.Failure = run.failure
	run.j.view.Code, run.j.view.State = run.code, "SUCCEEDED"
	if run.code != OK && run.code != Cancelled && run.code != RadioCodeWindowEnded {
		run.j.view.State = "FAILED"
	}
	if run.code == Cancelled {
		run.j.view.State = "CANCELLED"
	}
	run.j.finished = run.service.now()
	run.j.released = true

}
