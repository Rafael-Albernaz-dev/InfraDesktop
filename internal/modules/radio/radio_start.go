package radio

import (
	"context"
	"infradesktop/internal/modules/access"
)

type radioStartPlan struct {
	input     RadioInput
	target    PPTarget
	id        string
	link      *RadioLink
	devices   []Device
	revisions []uint64
	apOnly    bool
}

func (s *Service) startRadio(ctx context.Context, q Command, in RadioInput) (*Job, Code) {
	if in.Mode == RadioModeIndex {
		return s.startRadioIndex(ctx, q, in)
	}
	if !validRadioStartInput(in) {
		return nil, Invalid
	}
	if _, ok := s.Remote.(RadioRemote); !ok {
		return nil, Unavailable
	}
	plan, code := s.prepareRadioStart(ctx, q, in)
	if code != OK {
		return nil, code
	}
	q.Operation = plan.input.Operation
	s.Mu.Lock()
	defer s.Mu.Unlock()
	s.prune()
	if s.Closed {
		return nil, Unavailable
	}
	if job, code := s.existingRadioJobLocked(q, plan); job != nil || code != OK {
		return job, code
	}
	if code = s.radioJobCapacityLocked(); code != OK {
		return nil, code
	}
	key := radioKey(q, plan.link, plan.devices, plan.revisions)
	if code = s.radioStartCostLocked(plan, key); code != OK {
		return nil, code
	}
	return s.launchRadioJobLocked(q, plan, key)
}

func validRadioStartInput(in RadioInput) bool {
	if !access.ValidCompanyID(in.Operation) || !access.ValidCompanyID(in.DeviceID) {
		return false
	}
	return in.Mode == RadioOnce || in.Mode == RadioMeasure || in.Mode == RadioLive
}

func (s *Service) prepareRadioStart(ctx context.Context, q Command, in RadioInput) (radioStartPlan, Code) {
	plan := radioStartPlan{input: in, id: in.DeviceID}
	row, code := s.radioRow(ctx, q, in)
	if code != OK {
		return plan, code
	}
	plan.link, plan.id, code = s.resolveRadioStartLink(ctx, q, in, row)
	if code != OK {
		return plan, code
	}
	plan.target, code = s.readRadioStartTarget(ctx, q, in, plan.id, row)
	if code != OK {
		return plan, code
	}
	plan.devices, plan.revisions, code = s.radioStartDevices(ctx, q, plan.target, plan.link)
	if code != OK {
		return plan, code
	}
	if row == nil {
		if plan.input.Family != RadioStandalone && !IsRadioFamily(plan.input.Family) {
			return plan, Invalid
		}
		// Standalone admission always identifies its driver through the remote protocol.
		plan.input.Family = RadioStandalone
	}
	plan.apOnly, code = s.radioStartAPOnly(ctx, q, plan)
	return plan, code
}

func (s *Service) resolveRadioStartLink(ctx context.Context, q Command, in RadioInput, row *PPRow) (*RadioLink, string, Code) {
	if row == nil {
		if in.Mode != RadioOnce {
			return nil, "", Invalid
		}
		return nil, in.DeviceID, OK
	}
	store, ok := s.Store.(RadioStore)
	if !ok {
		return nil, "", Unavailable
	}
	links, code := store.RadioLinks(ctx, q.Scope.Profile, in.DeviceID, row.Login)
	if code != OK {
		return nil, "", code
	}
	if len(links) != 1 {
		return nil, "", RadioCodeLinkRequired
	}
	return &links[0], links[0].AP, OK
}

func (s *Service) readRadioStartTarget(ctx context.Context, q Command, in RadioInput, id string, row *PPRow) (PPTarget, Code) {
	store, ok := s.Store.(PPStore)
	if !ok {
		return PPTarget{}, Unavailable
	}
	target, code := store.ReadPPTarget(ctx, q.Scope.Profile, id, false)
	if code != OK && code != MissingCredential {
		return target, code
	}
	if in.Revision == 0 || (row == nil && in.Revision != target.Device.Revision) {
		return target, Revision
	}
	if row == nil {
		return target, validateStandaloneRadioTarget(target, code)
	}
	return target, OK
}

func validateStandaloneRadioTarget(target PPTarget, code Code) Code {
	device := target.Device
	if !device.Active {
		return Inactive
	}
	if code == MissingCredential || device.CredentialID == "" {
		return MissingCredential
	}
	if device.Role != RoleAP && device.Role != RoleCPE && device.Role != RolePTP {
		return RadioCodeUnsupported
	}
	return OK
}

func (s *Service) radioStartDevices(ctx context.Context, q Command, target PPTarget, link *RadioLink) ([]Device, []uint64, Code) {
	devices, revisions := []Device{target.Device}, []uint64{target.Credential.Metadata.Revision}
	if link == nil {
		return devices, revisions, OK
	}
	store, ok := s.Store.(PPStore)
	if !ok {
		return nil, nil, Unavailable
	}
	cpe, code := store.ReadPPTarget(ctx, q.Scope.Profile, link.CPE, false)
	if code != OK && code != MissingCredential {
		return nil, nil, code
	}
	return append(devices, cpe.Device), append(revisions, cpe.Credential.Metadata.Revision), OK
}

func (s *Service) radioStartAPOnly(ctx context.Context, q Command, plan radioStartPlan) (bool, Code) {
	if plan.input.Mode == RadioOnce || len(plan.devices) != 2 {
		return false, OK
	}
	store, ok := s.Store.(RadioStore)
	if !ok {
		return false, Unavailable
	}
	observed, profile, code := store.RadioCurrent(ctx, q.Scope.Profile, plan.devices[1].ID)
	if code != OK {
		return false, code
	}
	return radioUnsupportedCPE(observed, profile, plan.devices[1], plan.revisions[1]), OK
}
