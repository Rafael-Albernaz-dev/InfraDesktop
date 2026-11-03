package radio

import (
	"context"
	"infradesktop/internal/modules/access"
)

func (s *Service) handleRadioLink(ctx context.Context, q Command, in RadioInput, store RadioStore, v *RadioView) Code {
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
		saved, replay, c := s.confirmRadioLink(ctx, q, in, store, row, links)
		if c != OK {
			return c
		}
		if replay {
			v.Link = saved
			return OK
		}
		links = []RadioLink{*saved}
	}
	if len(links) == 1 {
		link := links[0]
		reading, _, c := store.RadioCurrent(ctx, q.Scope.Profile, link.AP)
		link.ConfirmedNow = c == OK && RadioLinkPresent(link, reading)
		v.Link = &link
	}
	st, c := s.Store.ReadEquipment(ctx, q.Scope.Profile)
	if c != OK {
		return c
	}
	assisted := s.radioIndexView(q, row, st)
	v.Index, v.Proposal, v.MatchReason = assisted.Index, assisted.Proposal, assisted.MatchReason
	*v, c = radioPage(*v, in)
	return c
}

func (s *Service) confirmRadioLink(ctx context.Context, q Command, in RadioInput, store RadioStore,
	row *PPRow, links []RadioLink) (*RadioLink, bool, Code) {
	if in.Link == nil || !access.ValidCompanyID(in.Operation) {
		return nil, false, Invalid
	}
	choice := in.Link
	var enroll *Device
	if choice.Origin == "sugerido-confirmado" {
		st, c := s.Store.ReadEquipment(ctx, q.Scope.Profile)
		if c != OK {
			return nil, false, c
		}
		if radioOperationExists(st, in.Operation) {
			saved, c := replayRadioLink(ctx, q, in, store, links)
			return saved, true, c
		}
		choice, enroll, c = s.resolveRadioLinkChoice(q, in, row, st)
		if c != OK {
			return nil, false, c
		}
	}
	link, c := validatedRadioLink(choice, enroll)
	if c != OK {
		return nil, false, c
	}
	if c = s.validateRadioLinkEndpoints(ctx, q, link, row); c != OK {
		return nil, false, c
	}
	link.Concentrator, link.Scope, link.Login = in.DeviceID, PPScope, row.Login
	link.At, link.ConfirmedNow = PPTime(s.Now()), false
	c = store.SaveRadioLink(ctx, q.Scope.Profile, *link, in.Operation, radioLinkSource(in))
	return link, false, c
}

func radioOperationExists(st State, operation string) bool {
	for _, op := range st.Operations {
		if op.ID == operation {
			return true
		}
	}
	return false
}

func radioLinkSource(in RadioInput) PPInput {
	return PPInput{DeviceID: in.DeviceID, ObservationID: in.ObservationID, RowID: in.RowID}
}

// Replay uses the durable operation, even when enrollment changed the current proposal.
func replayRadioLink(ctx context.Context, q Command, in RadioInput, store RadioStore,
	links []RadioLink) (*RadioLink, Code) {
	for _, saved := range links {
		if !radioReplayMatches(saved, *in.Link, in.Operation) {
			continue
		}
		saved.NewCPE = nil
		if c := store.SaveRadioLink(ctx, q.Scope.Profile, saved, in.Operation, radioLinkSource(in)); c != OK {
			return nil, c
		}
		return &saved, OK
	}
	return nil, Revision
}

func radioReplayMatches(saved RadioLink, choice RadioLinkChoice, operation string) bool {
	if saved.AP != choice.AP || saved.MAC != choice.MAC || saved.Interface != choice.Interface {
		return false
	}
	if saved.BSSID != choice.BSSID || saved.Origin != choice.Origin {
		return false
	}
	return choice.CPE == saved.CPE || (choice.CPE == "" && saved.CPE == operation)
}

func (s *Service) resolveRadioLinkChoice(q Command, in RadioInput, row *PPRow, st State) (*RadioLinkChoice, *Device, Code) {
	choice := in.Link
	proposed := s.radioIndexView(q, row, st).Proposal
	if !radioProposalMatches(proposed, *choice) {
		return nil, nil, RadioCodeLinkUnconfirmed
	}
	if choice.CredentialID != proposed.CredentialID {
		return nil, nil, Revision
	}
	if choice.CPE != "" {
		return choice, nil, OK
	}
	if credential(&st, proposed.CredentialID) == nil {
		return nil, nil, MissingCredential
	}
	if len(st.Devices) >= MaxDevices {
		return nil, nil, Limit
	}
	enroll := &Device{ID: in.Operation, Name: "CPE " + row.Login, Role: RoleCPE,
		Address: row.Address, Port: 22, Transport: TransportSSH, CredentialID: proposed.CredentialID, Revision: 1, Active: true}
	if ValidateDevice(*enroll) != OK {
		return nil, nil, Invalid
	}
	copy := *choice
	copy.CPE = enroll.ID
	return &copy, enroll, OK
}

func radioProposalMatches(proposed *RadioProposal, choice RadioLinkChoice) bool {
	if proposed == nil {
		return false
	}
	p := proposed.Choice
	return p.AP == choice.AP && p.MAC == choice.MAC && p.Interface == choice.Interface && p.BSSID == choice.BSSID && p.CPE == choice.CPE
}

func validatedRadioLink(choice *RadioLinkChoice, enroll *Device) (*RadioLink, Code) {
	link := &RadioLink{CPE: choice.CPE, AP: choice.AP, MAC: choice.MAC, Interface: choice.Interface,
		BSSID: choice.BSSID, Evidence: choice.Evidence, Origin: choice.Origin, NewCPE: enroll}
	if !validRadioLinkInput(*link) {
		return nil, Invalid
	}
	var valid bool
	link.MAC, valid = RadioMAC(link.MAC)
	if !valid {
		return nil, Invalid
	}
	link.BSSID, valid = RadioMAC(link.BSSID)
	if !valid {
		return nil, Invalid
	}
	return link, OK
}

func validRadioLinkInput(link RadioLink) bool {
	if !access.ValidCompanyID(link.AP) || !access.ValidCompanyID(link.CPE) {
		return false
	}
	if !ValidText(link.Interface, 80) || !ValidText(link.Evidence, 256) {
		return false
	}
	return link.Origin == "manual" || link.Origin == "sugerido-confirmado"
}

func (s *Service) validateRadioLinkEndpoints(ctx context.Context, q Command, link *RadioLink, row *PPRow) Code {
	store, ok := s.Store.(PPStore)
	if !ok {
		return Unavailable
	}
	ap, c := store.ReadPPTarget(ctx, q.Scope.Profile, link.AP, false)
	if c != OK {
		return c
	}
	cpe := PPTarget{}
	if link.NewCPE != nil {
		cpe.Device = *link.NewCPE
	} else {
		cpe, c = store.ReadPPTarget(ctx, q.Scope.Profile, link.CPE, false)
		if c != OK {
			return c
		}
	}
	if ap.Device.Role != RoleAP || cpe.Device.Role != RoleCPE {
		return RadioCodeCPEUnreachable
	}
	if !ap.Device.Active || !cpe.Device.Active || cpe.Device.Address != row.Address {
		return RadioCodeCPEUnreachable
	}
	return OK
}
