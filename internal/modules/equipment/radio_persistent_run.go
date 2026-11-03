package equipment

import (
	"context"
	"fmt"
	"time"
)

type persistentRadioRun struct {
	service                         *Service
	ctx                             context.Context
	cancel                          context.CancelFunc
	store                           RadioStore
	j                               *runningJob
	link                            *RadioLink
	revisions                       []uint64
	key                             string
	view                            RadioView
	apOnly                          bool
	code                            Code
	reason                          string
	failure                         *RadioFailure
	sessions                        []RadioReadSession
	families                        []RadioFamily
	interval                        time.Duration
	cost                            RadioCost
	measurements                    []RadioMeasurement
	series                          []RadioTickReceipt
	previousAP, previousCPE         *RadioReading
	failures, cpuStreak, pingStreak int
	watchDone, watchExited          chan struct{}
	authFailure                     chan Code
}

func (s *Service) runRadioPersistent(parent context.Context, j *runningJob, link *RadioLink, revisions []uint64, key string) {
	defer s.wg.Done()
	defer j.cancel()
	defer func() { s.mu.Lock(); j.released = true; s.mu.Unlock() }()
	ctx, cancel := context.WithTimeout(parent, 90*time.Second)
	defer cancel()
	store, ok := s.store.(RadioStore)
	if !ok {
		s.mu.Lock()
		j.view.Code, j.view.State = Unavailable, "FAILED"
		j.released = true
		j.finished = s.now()
		s.mu.Unlock()
		return
	}
	run := &persistentRadioRun{service: s, ctx: ctx, cancel: cancel, store: store, j: j, link: link, revisions: revisions, key: key,
		view: RadioView{Link: link, APCode: OK, CPECode: OK}, code: OK,
		sessions: make([]RadioReadSession, 0, 2), families: make([]RadioFamily, 0, 2),
		watchDone: make(chan struct{}), watchExited: make(chan struct{}), authFailure: make(chan Code, 1)}
	run.loadCurrent()
	defer run.finish()
	go run.watchAuthorization()
	defer func() { close(run.watchDone); <-run.watchExited }()
	run.openSides()
	defer run.closeSessions()
	run.measureTicks()
}

// Current readings only seed presentation; openSide immediately rereads and propagates every required store error.
func (run *persistentRadioRun) loadCurrent() {
	run.view.Reading, _, _ = run.store.RadioCurrent(run.ctx, run.j.command.Scope.Profile, run.j.device.ID)
	if run.link != nil {
		run.view.CPEReading, _, _ = run.store.RadioCurrent(run.ctx, run.j.command.Scope.Profile, run.link.CPE)
	}

}

func (run *persistentRadioRun) watchAuthorization() {
	defer close(run.watchExited)
	ticker := time.NewTicker(40 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-run.ctx.Done():
			return
		case <-run.watchDone:
			return
		case <-ticker.C:
			for i, d := range run.j.radioDevices {
				copy := runningJob{command: run.j.command, device: d, credentialRevision: run.revisions[i]}
				_, _, c := run.service.check(run.ctx, &copy, false)
				if c == OK {
					_, c = run.service.radioRow(run.ctx, run.j.command, run.j.radioInput)
				}
				if c != OK {
					select {
					case run.authFailure <- c:
					default:
					}
					run.cancel()
					return
				}
			}
		}
	}

}

func (run *persistentRadioRun) openSides() {
	for i, d := range run.j.radioDevices {
		run.openSide(i, d)
		if run.code != OK {
			break
		}
	}
}

func (run *persistentRadioRun) openSide(i int, d Device) {
	observed, p, c := run.store.RadioCurrent(run.ctx, run.j.command.Scope.Profile, d.ID)
	if c != OK {
		run.code = c
		return
	}
	if i == 1 && radioUnsupportedCPE(observed, p, d, run.revisions[i]) {
		run.apOnly = true
		model, version := observed.Model, observed.Version
		run.view.CPENote = fmt.Sprintf("CPE: firmware sem driver (%s / %s), dados vistos pelo AP", model, version)
		run.view.CPEReading, run.view.CPECode = nil, RadioCodeUnsupported
		return
	}
	if p == nil || !p.Confirmed {
		run.code = RadioCodeFamilyConfirmationRequired
		return
	}
	session, c := run.service.openRadioSide(run.ctx, run.j, d, run.revisions[i], p.Family, run.link)
	if c != OK {
		if i == 0 {
			run.view.APCode = c
		} else {
			run.view.CPECode = c
		}
		run.code = c
		return
	}
	run.sessions = append(run.sessions, session)
	run.families = append(run.families, p.Family)

	reading, c := run.service.readRadioSide(run.ctx, run.j, d, run.revisions[i], p.Family, session.Initial)
	run.recordInitial(i, reading, c)
}

func (run *persistentRadioRun) recordInitial(i int, reading *RadioReading, c Code) {
	if c != OK {
		if i == 0 {
			run.view.APCode = c
		} else {
			run.view.CPECode = c
		}
		if reading != nil {
			run.failure = reading.Failure
		}
		run.code = c
		run.reason = "Leitura inicial: " + string(c)
		if run.failure != nil {
			run.reason += " (" + run.failure.Reason + ")"
		}
		return
	}
	if i == 0 {
		run.view.Reading = reading
		if run.link != nil && !RadioLinkPresent(*run.link, reading) {
			run.code = RadioCodeLinkUnconfirmed
			return
		}
	} else {
		run.view.CPEReading = reading
		if run.link != nil &&
			((reading.APBSSID != "" &&
				reading.APBSSID != run.link.BSSID) ||
				(reading.WirelessMAC != "" &&
					reading.WirelessMAC != run.link.MAC)) {
			run.code = RadioCodeLinkUnconfirmed
			return
		}
	}

}

func (run *persistentRadioRun) closeSessions() {
	for _, session := range run.sessions {
		if session != nil {
			session.Close()
		}
	}
}

func (run *persistentRadioRun) prepareCost() {
	run.interval = time.Second
	run.cost = RadioCost{Interval: 1, APOnly: run.apOnly}
	if run.j.radioInput.Mode == RadioLive {
		run.service.mu.Lock()
		run.cost = run.service.radioCosts[run.key]
		run.service.mu.Unlock()
		run.interval = time.Duration(run.cost.Interval * float64(time.Second))
		run.cost.Series = nil
	}

}

func (run *persistentRadioRun) measureTicks() {
	run.prepareCost()
	limit := 10
	if run.j.radioInput.Mode == RadioLive {
		limit = 90
	}
	for tick := 0; run.code == OK && tick < limit && run.ctx.Err() == nil; tick++ {
		observed := run.readTick()
		run.recordTick(observed)
		run.applyTickGates(observed)
		run.publishTick(observed)
		if run.code != OK {
			break
		}
		if tick+1 >= limit {
			continue
		}
		run.waitTick(observed)
		if run.ctx.Err() != nil {
			break
		}
		run.reopenFailedSides(observed)
	}
	run.cacheCost()
}
