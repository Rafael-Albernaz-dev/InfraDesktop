package radio

import (
	"fmt"
	"time"
)

type persistentRadioTick struct {
	started, ended             time.Time
	ap, cpe                    *RadioReading
	apCode, cpeCode, probeCode Code
	load                       RadioLoad
	entry                      RadioTickReceipt
	measurement                RadioMeasurement
}

func (run *persistentRadioRun) readTick() *persistentRadioTick {
	tick := &persistentRadioTick{started: time.Now()}
	tick.ap, tick.apCode = run.readAPTick()
	tick.cpe, tick.cpeCode = run.readCPETick()
	run.validateTickLink(tick)
	tick.load, tick.probeCode = run.sessions[0].Probe()
	tick.ended = time.Now()
	tick.entry = run.tickReceipt(tick)
	tick.measurement = run.tickMeasurement(tick)
	return tick
}
func (run *persistentRadioRun) readAPTick() (*RadioReading, Code) {
	ap,
		apCode := run.service.readRadioSide(run.ctx,
		run.j,
		run.j.RadioDevices[0],
		run.revisions[0],
		run.families[0],
		func() (*RadioReading,
			Code) {
			r, c := run.sessions[0].Tick()
			if c == OK {
				r.At = PPTime(run.service.Now())
				if r.Version == "" && run.view.Reading != nil {
					r.Version = run.view.Reading.Version
				}
				RadioCounterMetrics(run.previousAP, r)
			}
			return r, c
		})
	run.view.APCode = apCode
	if apCode != OK && !radioTransientTick(apCode) {
		run.code, run.reason = apCode, "AP: "+string(apCode)
	}

	return ap, apCode
}

func (run *persistentRadioRun) readCPETick() (*RadioReading, Code) {
	cpeCode := OK
	var cpe *RadioReading
	if len(run.sessions) == 2 && run.code == OK {
		cpe,
			cpeCode = run.service.readRadioSide(run.ctx,
			run.j,
			run.j.RadioDevices[1],
			run.revisions[1],
			run.families[1],
			func() (*RadioReading,
				Code) {
				r, c := run.sessions[1].Tick()
				if c == OK {
					r.At = PPTime(run.service.Now())
					RadioCounterMetrics(run.previousCPE, r)
				}
				return r, c
			})
	}
	if !run.apOnly {
		run.view.CPECode = cpeCode
	}
	if cpeCode != OK && !radioTransientTick(cpeCode) {
		run.code, run.reason = cpeCode, "CPE: "+string(cpeCode)
	}

	return cpe, cpeCode
}

func (run *persistentRadioRun) validateTickLink(tick *persistentRadioTick) {
	ap := tick.ap
	apCode := tick.apCode
	cpe := tick.cpe
	cpeCode := tick.cpeCode
	if run.link != nil && apCode == OK && !RadioLinkPresent(*run.link, ap) {
		apCode = RadioCodeLinkUnconfirmed
		run.view.APCode = apCode
	}
	if run.link != nil &&
		cpeCode == OK &&
		cpe != nil &&
		((cpe.APBSSID != "" &&
			cpe.APBSSID != run.link.BSSID) ||
			(cpe.WirelessMAC != "" &&
				cpe.WirelessMAC != run.link.MAC)) {
		cpeCode = RadioCodeLinkUnconfirmed
		run.view.CPECode = cpeCode
	}
	if apCode != OK && !radioTransientTick(apCode) {
		run.code, run.reason = apCode, "AP: "+string(apCode)
	}
	if cpeCode != OK && !radioTransientTick(cpeCode) {
		run.code, run.reason = cpeCode, "CPE: "+string(cpeCode)
	}

	tick.apCode, tick.cpeCode = apCode, cpeCode
}

func (run *persistentRadioRun) tickReceipt(tick *persistentRadioTick) RadioTickReceipt {
	ap := tick.ap
	apCode := tick.apCode
	cpe := tick.cpe
	cpeCode := tick.cpeCode
	load := tick.load
	probeCode := tick.probeCode
	ended := tick.ended
	entry := RadioTickReceipt{At: PPTime(ended), CPU: []int{}, ICMP: append([]float64{}, load.PingRTT...), Reason: "OK"}
	if ap != nil {
		entry.AP = ap.CommandSeconds
		if apCode != OK {
			entry.Failure = ap.Failure
		}
		entry.CPU = append([]int{}, ap.CPU...)
		load.CPU = entry.CPU
		if len(load.PingRTT) > 0 {
			v := load.PingRTT[len(load.PingRTT)-1]
			ap.PingRTT = &v
		}
	}
	if cpe != nil {
		entry.CPE = cpe.CommandSeconds
		if cpeCode != OK && entry.Failure == nil {
			entry.Failure = cpe.Failure
		}
	}
	if apCode != OK {
		entry.Reason = "AP: " + string(apCode)
	} else if cpeCode != OK {
		entry.Reason = "CPE: " + string(cpeCode)
	} else if probeCode != OK {
		entry.Reason = "ICMP indisponível: " + string(probeCode)
	} else if load.PingReceived < load.PingSent {
		entry.Reason = fmt.Sprintf("Ping perdido: %d/%d", load.PingReceived, load.PingSent)
	}
	if entry.Failure != nil {
		run.failure = entry.Failure
		entry.Reason += " (" + entry.Failure.Reason + ")"
	}

	tick.load = load
	return entry
}

func (run *persistentRadioRun) tickMeasurement(tick *persistentRadioTick) RadioMeasurement {
	ap := tick.ap
	cpe := tick.cpe
	apCode := tick.apCode
	cpeCode := tick.cpeCode
	probeCode := tick.probeCode
	started := tick.started
	ended := tick.ended
	load := tick.load
	measurement := RadioMeasurement{APOnly: run.apOnly,
		Seconds: ended.Sub(started).Seconds(),
		Started: started,
		Ended:   ended,
		Complete: apCode == OK &&
			cpeCode == OK &&
			probeCode == OK,
		Load: load}
	if ap != nil {
		measurement.AP = RadioTiming{CommandSeconds: ap.CommandSeconds, PathSeconds: ap.Seconds}
	}
	if cpe != nil {
		measurement.CPE = RadioTiming{CommandSeconds: cpe.CommandSeconds, PathSeconds: cpe.Seconds}
	}

	return measurement
}

func (run *persistentRadioRun) recordTick(tick *persistentRadioTick) {
	ap := tick.ap
	apCode := tick.apCode
	cpe := tick.cpe
	cpeCode := tick.cpeCode
	entry := tick.entry
	measurement := tick.measurement
	if apCode == OK {
		AccumulateRadioChains(run.previousAP, ap)
		run.view.Reading = ap
		run.previousAP = ap
	}
	if cpeCode == OK && cpe != nil {
		AccumulateRadioChains(run.previousCPE, cpe)
		run.view.CPEReading = cpe
		run.previousCPE = cpe
	}
	if run.link != nil {
		copy := *run.link
		copy.ConfirmedNow = apCode == OK
		run.view.Link = &copy
	}
	run.series = append(run.series, entry)
	if run.j.RadioInput.Mode == RadioMeasure {
		run.measurements = append(run.measurements, measurement)
		run.cost = AssessRadioMeasurements(run.measurements)
	}

}

func (run *persistentRadioRun) applyTickGates(tick *persistentRadioTick) {
	ap := tick.ap
	cpe := tick.cpe
	load := tick.load
	entry := tick.entry
	violation := entry.Reason != "OK"
	if run.j.RadioInput.Mode == RadioLive && !radioTickComplete(ap, cpe, load, run.apOnly) {
		violation = true
		if entry.Reason == "OK" {
			entry.Reason = radioTickReason(ap, cpe, run.cost, run.apOnly)
			run.series[len(run.series)-1] = entry
		}
	}

	tick.entry = entry
	run.liveLoadGates(tick)
	if violation {
		run.failures++
	} else {
		run.failures = 0
	}
}

func (run *persistentRadioRun) liveLoadGates(tick *persistentRadioTick) {
	ap := tick.ap
	load := tick.load
	entry := tick.entry
	if run.j.RadioInput.Mode == RadioLive && run.code == OK {
		run.recordCPUStreak(ap)
		run.recordPingStreak(load)
		if run.cpuStreak >= 2 {
			run.reason = fmt.Sprintf("CPU do AP chegou a %d%% em duas leituras consecutivas (limite 90%%)", ap.CPU[len(ap.CPU)-1])
		}
		if run.cpuStreak == 1 && run.reason == "" {
			entry.Reason = fmt.Sprintf("CPU %d%%: pico isolado; próxima leitura decide", ap.CPU[len(ap.CPU)-1])
			run.series[len(run.series)-1] = entry
		}
		if run.reason != "" {
			entry.Reason = run.reason
			run.series[len(run.series)-1] = entry
			run.code = RadioCodeLiveAborted
		}
	}

	tick.entry = entry
}

func (run *persistentRadioRun) publishTick(tick *persistentRadioTick) {
	run.cost.Series = append([]RadioTickReceipt{}, run.series...)
	costSnapshot := run.cost
	run.view.Cost = &costSnapshot
	run.service.Mu.Lock()
	if run.j.View.State == "RUNNING" && run.ctx.Err() == nil {
		copy := run.view
		run.j.Radio = &copy
	}
	run.service.Mu.Unlock()
	if run.failures >= 2 && run.code == OK {
		run.code = RadioCodeLiveAborted
		run.reason = tick.entry.Reason
	}
}

func (run *persistentRadioRun) waitTick(tick *persistentRadioTick) {
	started := tick.started
	apCode := tick.apCode
	cpeCode := tick.cpeCode
	delay := run.interval - time.Since(started)
	if apCode != OK || cpeCode != OK {
		delay = run.interval
	}
	if delay < 0 {
		delay = 0
	}
	timer := time.NewTimer(delay)
	select {
	case <-run.ctx.Done():
		timer.Stop()
	case <-timer.C:
	}
	if run.ctx.Err() != nil {
		return
	}

}

func (run *persistentRadioRun) recordCPUStreak(ap *RadioReading) {
	if ap != nil && len(ap.CPU) > 0 {
		for _, cpu := range ap.CPU {
			if cpu > RadioMaxCPU && cpu <= 100 {
				run.cpuStreak++
			} else {
				run.cpuStreak = 0
			}
		}
	} else {
		run.cpuStreak = 0
	}

}

func (run *persistentRadioRun) recordPingStreak(load RadioLoad) {
	for _, rtt := range load.PingRTT {
		if radioFinite(rtt) && rtt > run.cost.PingLimit {
			run.pingStreak++
		} else {
			run.pingStreak = 0
		}
		if run.pingStreak >= 2 {
			run.reason = fmt.Sprintf("Ping %.2f ms em duas leituras consecutivas acima de %.2f ms (2× a mediana)", rtt, run.cost.PingLimit)
		}
	}

}
