package equipment

import (
	"context"
	"fmt"
	"infradesktop/internal/modules/access"
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
	if !validRadioSourceIDs(in) {
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
	if target.Device.Role != RoleConcentrator {
		return nil, Invalid
	}
	if in.Revision != 0 && target.Device.Revision != in.Revision {
		return nil, Revision
	}
	p := current.Current.Provenance
	if !radioProvenanceMatchesTarget(p, target) {
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
		return nil, RadioCodeSessionOffline
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
	var code Code
	v, code = pageRadioIndex(v, in.Offset)
	if code != OK {
		return RadioView{}, code
	}
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
			return s.handleRadioJob(ctx, q, in, store, &v)
		case "radio.current", "radio.confirm-family":
			return s.handleRadioCurrent(ctx, q, in, store, &v)
		case "radio.link-current", "radio.index-current", "radio.link-confirm":
			return s.handleRadioLink(ctx, q, in, store, &v)
		case "radio.start":
			var c Code
			v.Job, c = s.startRadio(ctx, q, in)
			return c
		}
		return Invalid
	})
	return v, c
}

// The store's default profile is also called RadioUnsupported. Only an observed,
// current identification authorizes omitting the CPE; absence is not evidence.
func radioUnsupportedCPE(r *RadioReading, p *RadioProfile, d Device, credential uint64) bool {
	return p != nil &&
		p.Family == RadioUnsupported &&
		r != nil &&
		r.Family == RadioUnsupported &&
		r.Version != "" &&
		r.Model != "" &&
		r.Compatible &&
		r.DeviceID == d.ID &&
		r.Revision == d.Revision &&
		r.CredentialRevision == credential &&
		r.Connection == PPConnection(d)
}

func validRadioSourceIDs(in RadioInput) bool {
	return access.ValidCompanyID(in.DeviceID) && access.ValidCompanyID(in.ObservationID) && access.ValidCompanyID(in.RowID)
}
func radioProvenanceMatchesTarget(p PPProvenance, target PPTarget) bool {
	return p.Revision == target.Device.Revision && p.CredentialRevision == target.Credential.Metadata.Revision &&
		p.Connection == PPConnection(target.Device)
}
