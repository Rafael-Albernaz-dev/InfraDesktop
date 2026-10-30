package equipment

import (
	"context"
	"fmt"
	"infradesktop/internal/modules/access"
	"strconv"
	"strings"
	"time"
)

func jobUsesEndpoint(j *runningJob, d Device) bool {
	if Endpoint(j.device) == Endpoint(d) {
		return true
	}
	for _, target := range j.radioDevices {
		if Endpoint(target) == Endpoint(d) {
			return true
		}
	}
	return false
}
func (s *Service) radioRow(ctx context.Context, q Command, in RadioInput) (*PPRow, Code) {
	if in.RowID == "" {
		return nil, OK
	}
	store, ok := s.store.(PPStore)
	if !ok {
		return nil, Unavailable
	}
	if !access.ValidCompanyID(in.DeviceID) || !access.ValidCompanyID(in.ObservationID) || !access.ValidCompanyID(in.RowID) {
		return nil, Invalid
	}
	current, c := store.PPCurrent(ctx, q.Scope.Profile, in.DeviceID)
	if c != OK {
		return nil, c
	}
	if current.Current == nil || current.Current.ID != in.ObservationID {
		return nil, ObservationMissing
	}
	target, c := store.ReadPPTarget(ctx, q.Scope.Profile, in.DeviceID, false)
	if c != OK {
		return nil, c
	}
	if target.Device.Role != "concentrator" {
		return nil, Invalid
	}
	if in.Revision != 0 && target.Device.Revision != in.Revision {
		return nil, Revision
	}
	p := current.Current.Provenance
	if p.Revision != target.Device.Revision || p.CredentialRevision != target.Credential.Metadata.Revision || p.Connection != PPConnection(target.Device) {
		return nil, ObservationIncompatible
	}
	detail, c := store.PPSession(ctx, q.Scope.Profile, PPInput{DeviceID: in.DeviceID, ObservationID: in.ObservationID, RowID: in.RowID})
	if c != OK {
		return nil, c
	}
	if len(detail.Rows) != 1 {
		return nil, ObservationMissing
	}
	r := detail.Rows[0]
	if r.State == "OFFLINE" {
		return nil, "RADIO_SESSION_OFFLINE"
	}
	return &r, OK
}
func radioKey(q Command, link *RadioLink, devices []Device, revisions []uint64) string {
	key := q.Scope.Profile + "|" + q.Scope.Activation
	if link != nil {
		key += fmt.Sprintf("|%q|%q|%q|%q|%q", link.Concentrator, link.Login, link.MAC, link.Interface, link.BSSID)
	}
	for i, d := range devices {
		key += fmt.Sprintf("|%q|%d|%d|%q", d.ID, d.Revision, revisions[i], PPConnection(d))
	}
	return key
}
func radioPage(v RadioView, in RadioInput) (RadioView, Code) {
	if v.Reading == nil {
		return v, OK
	}
	if in.ReadingID != "" && v.Reading.ID != in.ReadingID {
		return RadioView{}, ObservationMissing
	}
	r := *v.Reading
	if v.Link != nil {
		clients := []RadioClient{}
		for _, client := range r.Clients {
			if client.MAC == v.Link.MAC && client.Interface == v.Link.Interface && client.BSSID == v.Link.BSSID {
				clients = append(clients, client)
			}
		}
		r.Clients = clients
	}
	v.Total = len(r.Clients)
	if in.Offset < 0 || in.Offset > v.Total {
		return RadioView{}, CursorInvalid
	}
	end := min(in.Offset+RadioPageSize, v.Total)
	r.Clients = append([]RadioClient{}, r.Clients[in.Offset:end]...)
	v.Reading, v.More, v.NextOffset = &r, end < v.Total, end
	return v, OK
}
func (s *Service) HandleRadio(ctx context.Context, q Command, in RadioInput) (RadioView, Code) {
	v := RadioView{}
	c := s.authorized(ctx, q, func(authority uint64) Code {
		q.Authority = authority
		store, ok := s.store.(RadioStore)
		if !ok {
			return Unavailable
		}
		switch q.Op {
		case "radio.status", "radio.cancel":
			if q.Op == "radio.cancel" && in.JobID == "" {
				if !access.ValidCompanyID(in.DeviceID) || !access.ValidCompanyID(in.Operation) {
					return Invalid
				}
				s.mu.Lock()
				defer s.mu.Unlock()
				for _, j := range s.jobs {
					if j.command.Op == "radio.start" && sameScope(q, j.command) && j.radioInput.DeviceID == in.DeviceID && j.command.Operation == in.Operation {
						j.cancel()
						j.view.State, j.view.Code = "CANCELLED", Cancelled
						v.Job = copyJob(j)
					}
				}
				return OK
			}
			if !access.ValidCompanyID(in.JobID) {
				return Invalid
			}
			check := q
			check.JobID = in.JobID
			check.Op = "device.query-status"
			if q.Op == "radio.cancel" {
				check.Op = "device.query-cancel"
			}
			job, c := s.statusKind(ctx, check, "radio.start")
			if c != OK {
				return c
			}
			s.mu.Lock()
			j := s.jobs[in.JobID]
			if j != nil && j.radio != nil {
				v = *j.radio
			}
			s.mu.Unlock()
			v.Job = job
			var code Code
			v, code = radioPage(v, in)
			return code
		case "radio.current", "radio.confirm-family":
			if !access.ValidCompanyID(in.DeviceID) {
				return Invalid
			}
			var c Code
			v.Reading, v.Profile, c = store.RadioCurrent(ctx, q.Scope.Profile, in.DeviceID)
			if c != OK {
				return c
			}
			if q.Op == "radio.confirm-family" {
				t, c := s.store.(PPStore).ReadPPTarget(ctx, q.Scope.Profile, in.DeviceID, false)
				if c != OK {
					return c
				}
				if t.Device.Revision != in.Revision {
					return Revision
				}
				c = store.ConfirmRadioFamily(ctx, q.Scope.Profile, t.Device, t.Credential.Metadata.Revision, in.Family, in.Warning, in.Critical)
				if c != OK {
					return c
				}
				v.Reading, v.Profile, c = store.RadioCurrent(ctx, q.Scope.Profile, in.DeviceID)
				if c != OK {
					return c
				}
			}
			v, c = radioPage(v, in)
			return c
		case "radio.link-current", "radio.link-confirm":
			row, c := s.radioRow(ctx, q, in)
			if c != OK {
				return c
			}
			if row == nil {
				return Invalid
			}
			links, c := store.RadioLinks(ctx, q.Scope.Profile, in.DeviceID, row.Login)
			if c != OK {
				return c
			}
			if q.Op == "radio.link-confirm" {
				if in.Link == nil || !access.ValidCompanyID(in.Operation) {
					return Invalid
				}
				choice := in.Link
				link := RadioLink{CPE: choice.CPE, AP: choice.AP, MAC: choice.MAC, Interface: choice.Interface, BSSID: choice.BSSID, Evidence: choice.Evidence, Origin: choice.Origin}
				if !access.ValidCompanyID(link.AP) || !access.ValidCompanyID(link.CPE) || !ValidText(link.Interface, 80) || !ValidText(link.Evidence, 256) || (link.Origin != "manual" && link.Origin != "sugerido-confirmado") {
					return Invalid
				}
				var valid bool
				link.MAC, valid = RadioMAC(link.MAC)
				if !valid {
					return Invalid
				}
				link.BSSID, valid = RadioMAC(link.BSSID)
				if !valid {
					return Invalid
				}
				ap, c := s.store.(PPStore).ReadPPTarget(ctx, q.Scope.Profile, link.AP, false)
				if c != OK {
					return c
				}
				cpe, c := s.store.(PPStore).ReadPPTarget(ctx, q.Scope.Profile, link.CPE, false)
				if c != OK {
					return c
				}
				if ap.Device.Role != "ap" || cpe.Device.Role != "cpe" || !ap.Device.Active || !cpe.Device.Active || cpe.Device.Address != row.Address {
					return "RADIO_CPE_UNREACHABLE"
				}
				reading, _, c := store.RadioCurrent(ctx, q.Scope.Profile, link.AP)
				if c != OK {
					return c
				}
				if link.Origin == "sugerido-confirmado" && !RadioLinkPresent(link, reading) {
					return "RADIO_LINK_UNCONFIRMED"
				}
				link.Concentrator, link.Scope, link.Login, link.At, link.ConfirmedNow = in.DeviceID, PPScope, row.Login, PPTime(s.now()), false
				if c := store.SaveRadioLink(ctx, q.Scope.Profile, link, in.Operation, PPInput{DeviceID: in.DeviceID, ObservationID: in.ObservationID, RowID: in.RowID}); c != OK {
					return c
				}
				links = []RadioLink{link}
			}
			if len(links) == 1 {
				link := links[0]
				reading, _, c := store.RadioCurrent(ctx, q.Scope.Profile, link.AP)
				link.ConfirmedNow = c == OK && RadioLinkPresent(link, reading)
				v.Link = &link
			}
			readings, c := store.RadioAPReadings(ctx, q.Scope.Profile)
			if c != OK {
				return c
			}
			st, c := s.store.ReadEquipment(ctx, q.Scope.Profile)
			if c != OK {
				return c
			}
			for _, d := range st.Devices {
				if !d.Active || d.Role != "cpe" || d.Address != row.Address {
					continue
				}
				cpe, _, _ := store.RadioCurrent(ctx, q.Scope.Profile, d.ID)
				for _, ap := range readings {
					for _, client := range ap.Clients {
						clue := cpe != nil && cpe.WirelessMAC == client.MAC
						caller, valid := RadioMAC(row.Caller)
						if valid {
							a, e1 := strconv.ParseUint(strings.ReplaceAll(caller, ":", ""), 16, 64)
							b, e2 := strconv.ParseUint(strings.ReplaceAll(client.MAC, ":", ""), 16, 64)
							clue = clue || (e1 == nil && e2 == nil && (a == b || a+1 == b || b+1 == a))
						}
						clue = clue || strings.EqualFold(client.Name, row.Login)
						if clue && len(v.Suggestions) < RadioPageSize {
							v.Suggestions = append(v.Suggestions, RadioLink{CPE: d.ID, AP: ap.DeviceID, MAC: client.MAC, Interface: client.Interface, BSSID: client.BSSID, Evidence: "MAC na registration table; identidade da CPE/caller-id/nome é pista, confirme", Origin: "sugerido-confirmado"})
						}
					}
				}
			}
			return OK
		case "radio.start":
			var c Code
			v.Job, c = s.startRadio(ctx, q, in)
			return c
		}
		return Invalid
	})
	return v, c
}

func (s *Service) startRadio(ctx context.Context, q Command, in RadioInput) (*Job, Code) {
	if !access.ValidCompanyID(in.Operation) || !access.ValidCompanyID(in.DeviceID) || (in.Mode != "once" && in.Mode != "measure" && in.Mode != "live") {
		return nil, Invalid
	}
	if _, ok := s.remote.(RadioRemote); !ok {
		return nil, Unavailable
	}
	row, c := s.radioRow(ctx, q, in)
	if c != OK {
		return nil, c
	}
	var link *RadioLink
	id := in.DeviceID
	if row != nil {
		links, c := s.store.(RadioStore).RadioLinks(ctx, q.Scope.Profile, in.DeviceID, row.Login)
		if c != OK {
			return nil, c
		}
		if len(links) != 1 {
			return nil, "RADIO_LINK_REQUIRED"
		}
		link, id = &links[0], links[0].AP
	} else if in.Mode != "once" {
		return nil, Invalid
	}
	t, c := s.store.(PPStore).ReadPPTarget(ctx, q.Scope.Profile, id, false)
	if c != OK && c != MissingCredential {
		return nil, c
	}
	if in.Revision == 0 || (row == nil && in.Revision != t.Device.Revision) {
		return nil, Revision
	}
	devices, revisions := []Device{t.Device}, []uint64{t.Credential.Metadata.Revision}
	if link != nil {
		cpe, c := s.store.(PPStore).ReadPPTarget(ctx, q.Scope.Profile, link.CPE, false)
		if c != OK && c != MissingCredential {
			return nil, c
		}
		devices, revisions = append(devices, cpe.Device), append(revisions, cpe.Credential.Metadata.Revision)
	}
	if row == nil {
		d := devices[0]
		if !d.Active {
			return nil, Inactive
		}
		if d.Fingerprint == "" {
			return nil, TrustRequired
		}
		if d.Role != "ap" && d.Role != "cpe" {
			return nil, "UNSUPPORTED"
		}
	}
	if row == nil && !RadioFamily(in.Family) {
		return nil, Invalid
	}
	q.Operation = in.Operation
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune()
	if s.closed {
		return nil, Unavailable
	}
	for _, j := range s.jobs {
		if sameScope(q, j.command) && j.command.Operation == in.Operation {
			if j.command.Op != q.Op || j.radioInput.DeviceID != in.DeviceID || j.radioInput.Mode != in.Mode || j.radioInput.RowID != in.RowID || j.radioInput.ObservationID != in.ObservationID || j.radioInput.Family != in.Family {
				return nil, Revision
			}
			return copyJob(j), OK
		}
		if !j.released {
			if in.Mode == "live" && j.command.Op == "radio.start" && j.radioInput.Mode == "live" {
				return nil, access.Busy
			}
			for _, d := range devices {
				if jobUsesEndpoint(j, d) {
					return nil, access.Busy
				}
			}
		}
	}
	active := 0
	for _, j := range s.jobs {
		if !j.released {
			active++
		}
	}
	if active >= MaxConcurrent {
		return nil, access.Busy
	}
	if len(s.jobs) >= MaxJobs {
		return nil, Limit
	}
	key := radioKey(q, link, devices, revisions)
	if in.Mode == "live" {
		cost, ok := s.radioCosts[key]
		if !ok || !cost.Enabled {
			return nil, "RADIO_LIVE_COST_REQUIRED"
		}
	}
	jobID, c := s.ids.ID()
	if c != OK {
		return nil, c
	}
	budget := 45 * time.Second
	if in.Mode == "live" {
		budget = 90 * time.Second
	}
	if in.Mode == "measure" {
		budget = 180 * time.Second
	}
	jobctx, cancel := context.WithTimeout(context.Background(), budget)
	j := &runningJob{command: q, device: t.Device, credentialRevision: t.Credential.Metadata.Revision, radioInput: in, radioDevices: devices, cancel: cancel, view: Job{ID: jobID, DeviceID: id, Kind: "radio.start", State: "RUNNING", Code: OK}}
	s.jobs[jobID] = j
	s.wg.Add(1)
	go s.runRadio(jobctx, j, link, revisions, key)
	return copyJob(j), OK
}

func (s *Service) readRadioSide(ctx context.Context, j *runningJob, d Device, credential uint64, family string) (*RadioReading, Code) {
	pathStarted := time.Now()
	ctx, cancel := context.WithTimeout(ctx, QueryBudget)
	defer cancel()
	copy := runningJob{command: j.command, device: d, credentialRevision: credential}
	copy.cancel = cancel
	defer s.watchPP(ctx, &copy)()
	check := func() Code {
		if _, c := s.radioRow(ctx, j.command, j.radioInput); c != OK {
			return c
		}
		_, _, c := s.check(ctx, &copy, false)
		return c
	}
	if c := check(); c != OK {
		return nil, c
	}
	if !d.Active {
		return nil, Inactive
	}
	if d.Fingerprint == "" {
		return nil, TrustRequired
	}
	user, secret, c := s.check(ctx, &copy, true)
	if c != OK {
		return nil, c
	}
	r, c := s.remote.(RadioRemote).ReadRadio(ctx, d, user, secret, family, check)
	secret = ""
	if c != OK || r == nil {
		if c == OK {
			c = Partial
		}
		return nil, c
	}
	if c := check(); c != OK {
		return nil, c
	}
	r.ID, c = s.ids.ID()
	if c != OK {
		return nil, c
	}
	StampRadio(r, d, credential, s.now())

	r.Seconds = time.Since(pathStarted).Seconds()
	c = s.authorized(ctx, j.command, func(uint64) Code {
		if _, c := s.radioRow(ctx, j.command, j.radioInput); c != OK {
			return c
		}
		return s.store.(RadioStore).SaveRadio(ctx, j.command.Scope.Profile, d, credential, r)
	})
	if c != OK {
		return nil, c
	}
	r.Seconds = time.Since(pathStarted).Seconds()
	return r, OK
}
func (s *Service) runRadio(ctx context.Context, j *runningJob, link *RadioLink, revisions []uint64, key string) {
	defer s.wg.Done()
	defer j.cancel()
	defer func() { s.mu.Lock(); j.released = true; s.mu.Unlock() }()
	watchDone, watchExited := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(watchExited)
		ticker := time.NewTicker(40 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-watchDone:
				return
			case <-ticker.C:
				if c := s.authorized(ctx, j.command, func(uint64) Code { _, c := s.radioRow(ctx, j.command, j.radioInput); return c }); c != OK {
					j.cancel()
					return
				}
			}
		}
	}()
	defer func() { close(watchDone); <-watchExited }()
	store := s.store.(RadioStore)
	view := RadioView{Link: link, APCode: OK, CPECode: OK}
	view.Reading, _, _ = store.RadioCurrent(ctx, j.command.Scope.Profile, j.device.ID)
	if link != nil {
		view.CPEReading, _, _ = store.RadioCurrent(ctx, j.command.Scope.Profile, link.CPE)
	}
	var measurements []RadioMeasurement
	interval := 5 * time.Second
	var liveCost RadioCost
	if j.radioInput.Mode == "live" {
		s.mu.Lock()
		liveCost = s.radioCosts[key]
		interval = time.Duration(liveCost.Interval * float64(time.Second))
		s.mu.Unlock()
	}
	limit := 1
	if j.radioInput.Mode == "measure" {
		limit = 10
	}
	if j.radioInput.Mode == "live" {
		limit = 512
	}
	failures := 0
	code := OK
	for sample := 0; sample < limit && ctx.Err() == nil; sample++ {
		type loadResult struct {
			value RadioLoad
			code  Code
		}
		var loadDone chan loadResult
		var sampleDone chan struct{}
		if j.radioInput.Mode == "measure" {
			loadDone = make(chan loadResult, 1)
			sampleDone = make(chan struct{})
			ready := make(chan struct{})
			probe, ok := s.remote.(RadioLoadRemote)
			if !ok {
				code = "RADIO_LOAD_UNAVAILABLE"
				break
			}
			go func() {
				user, secret, c := s.check(ctx, j, true)
				if c != OK {
					loadDone <- loadResult{code: c}
					return
				}
				copy := runningJob{command: j.command, device: j.device, credentialRevision: j.credentialRevision}
				value, c := probe.ObserveRadioLoad(ctx, j.device, user, secret, func() Code { _, _, c := s.check(ctx, &copy, false); return c }, func() { close(ready) }, sampleDone)
				secret = ""
				loadDone <- loadResult{value, c}
			}()
			select {
			case <-ctx.Done():
				code = Cancelled
			case result := <-loadDone:
				code = result.code
				if code == OK {
					code = Partial
				}
			case <-ready:
			}
			if code != OK {
				break
			}
		}
		started := time.Now()
		_, profile, c := store.RadioCurrent(ctx, j.command.Scope.Profile, j.device.ID)
		family := j.radioInput.Family
		var r *RadioReading
		if link != nil && (c != OK || !profile.Confirmed) {
			c = "RADIO_FAMILY_CONFIRMATION_REQUIRED"
		} else {
			if link != nil {
				family = profile.Family
			}
			r, c = s.readRadioSide(ctx, j, j.device, revisions[0], family)
		}
		view.APCode = c
		if c == OK {
			if sample > 0 {
				AccumulateRadioChains(view.Reading, r)
			}
			view.Reading = r
		}
		if link != nil {
			confirmed := c == OK && RadioLinkPresent(*link, r)
			copy := *link
			copy.ConfirmedNow = confirmed
			view.Link = &copy
			{
				d := j.radioDevices[1]
				row, sourceCode := s.radioRow(ctx, j.command, j.radioInput)
				if sourceCode != OK || row == nil || d.Role != "cpe" || d.Address != row.Address {
					view.CPECode = "RADIO_CPE_UNREACHABLE"
				} else {
					_, p, c := store.RadioCurrent(ctx, j.command.Scope.Profile, d.ID)
					if c != OK {
						view.CPECode = c
					} else if !p.Confirmed {
						view.CPECode = "RADIO_FAMILY_CONFIRMATION_REQUIRED"
					} else {
						cpe, c := s.readRadioSide(ctx, j, d, revisions[1], p.Family)
						view.CPECode = c
						if c == OK {
							if (cpe.APBSSID != "" && cpe.APBSSID != link.BSSID) || (cpe.WirelessMAC != "" && cpe.WirelessMAC != link.MAC) {
								view.CPECode = "RADIO_LINK_UNCONFIRMED"
							} else {
								if sample > 0 {
									AccumulateRadioChains(view.CPEReading, cpe)
								}
								view.CPEReading = cpe
							}
						}
					}
				}
			}
		}
		ended := time.Now()
		seconds := ended.Sub(started).Seconds()
		if j.radioInput.Mode == "measure" {
			close(sampleDone)
			var measured loadResult
			select {
			case <-ctx.Done():
				measured.code = Cancelled
			case measured = <-loadDone:
			}
			measurement := RadioMeasurement{Seconds: seconds, Started: started, Ended: ended, Complete: view.APCode == OK && view.CPECode == OK, Load: measured.value}
			if view.APCode == OK && r != nil {
				measurement.AP = RadioTiming{CommandSeconds: r.CommandSeconds, PathSeconds: r.Seconds}
			}
			if view.CPECode == OK && view.CPEReading != nil {
				measurement.CPE = RadioTiming{CommandSeconds: view.CPEReading.CommandSeconds, PathSeconds: view.CPEReading.Seconds}
			}
			measurements = append(measurements, measurement)
			cost := AssessRadioMeasurements(measurements)
			view.Cost = &cost
			if sample == 9 {
				view.Cost = &cost
				s.mu.Lock()
				if s.radioCosts == nil {
					s.radioCosts = map[string]RadioCost{}
				}
				if _, exists := s.radioCosts[key]; exists || len(s.radioCosts) < MaxDevices {
					s.radioCosts[key] = cost
				} else {
					cost.Enabled = false
					cost.Reason = "Limite de medições desta execução atingido."
				}
				s.mu.Unlock()
			}
		}
		if j.radioInput.Mode == "once" && (view.APCode != OK || view.CPECode != OK) {
			code = Partial
			if view.APCode != OK && view.Reading == nil && view.CPEReading == nil {
				code = view.APCode
			}
		}
		if view.APCode != OK || view.CPECode != OK || j.radioInput.Mode == "live" && RadioLiveTickViolation(r, view.CPEReading, liveCost) {
			failures++
		} else {
			failures = 0
		}
		s.mu.Lock()
		if j.view.State == "RUNNING" && ctx.Err() == nil {
			copy := view
			j.radio = &copy
		}
		s.mu.Unlock()
		if j.radioInput.Mode == "live" && failures >= 2 {
			code = "RADIO_LIVE_ABORTED"
			break
		}
		if sample+1 < limit {
			timer := time.NewTimer(interval)
			select {
			case <-ctx.Done():
				timer.Stop()
			case <-timer.C:
			}
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if ctx.Err() != nil {
		code = Cancelled
		if ctx.Err() == context.DeadlineExceeded {
			code = "RADIO_WINDOW_ENDED"
		}
	}
	if j.view.State == "CANCELLED" {
		code = Cancelled
	}
	j.view.Code, j.view.State = code, "SUCCEEDED"
	if code != OK && code != Cancelled && code != "RADIO_WINDOW_ENDED" {
		j.view.State = "FAILED"
	}
	if code == Cancelled {
		j.view.State = "CANCELLED"
	}
	j.finished = s.now()
}
