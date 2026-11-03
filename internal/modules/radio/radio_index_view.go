package radio

import (
	"context"
	"time"
)

func (s *Service) radioIndexView(q Command, row *PPRow, st State) RadioView {
	s.Mu.Lock()
	index, ok := s.radioIndexes[radioIndexKey(q)]
	s.Mu.Unlock()
	v := RadioView{MatchReason: "Índice vazio. Atualize o índice dos APs conhecidos."}
	if !ok {
		return v
	}
	v.Index = &index
	if reason := s.radioIndexReason(index, st); reason != "" {
		v.MatchReason = reason
		return v
	}
	matches := matchingRadioIndexEntries(index, row)
	if len(matches) == 0 {
		v.MatchReason = "Sem match de MAC ou last-ip; caller-id pode ser de roteador em bridge. Use o vínculo manual."
		return v
	}
	if len(matches) != 1 {
		v.MatchReason = "MAC/IP visto em mais de um registro ou AP. Use o vínculo manual."
		return v
	}
	v.Proposal, v.MatchReason = s.radioProposalForEntry(q, row, st, matches[0])
	return v
}
func (s *Service) radioIndexReason(index RadioIndex, st State) string {
	at, err := time.Parse(time.RFC3339Nano, index.At)
	if err != nil || s.Now().Sub(at) > RadioIndexTTL || s.Now().Before(at) || index.Context != radioIndexContext(st) {
		return "Índice antigo ou inventário alterado. Atualize antes de confirmar."
	}
	if !index.Complete {
		return "Índice incompleto: há APs sem leitura. Atualize ou use o vínculo manual."
	}
	return ""
}
func matchingRadioIndexEntries(index RadioIndex, row *PPRow) []RadioIndexEntry {
	caller, valid := RadioMAC(row.Caller)
	var matches []RadioIndexEntry
	for _, e := range index.Entries {
		if (valid && e.MAC == caller) || e.LastIP == row.Address && row.Address != "" {
			matches = append(matches, e)
		}
	}
	return matches
}
func (s *Service) radioProposalForEntry(q Command, row *PPRow, st State, e RadioIndexEntry) (*RadioProposal, string) {
	choice := RadioLinkChoice{AP: e.AP,
		MAC:       e.MAC,
		Interface: e.Interface,
		BSSID:     e.BSSID,
		Origin:    "sugerido-confirmado",
		Evidence:  "Índice registration-table: MAC/interface/BSSID; confirmado pelo técnico"}
	family := RadioAuto
	for _, d := range st.Devices {
		if d.Active && d.Role == RoleCPE && d.Address == row.Address {
			if choice.CPE != "" {
				return nil, "Mais de uma CPE no mesmo IP. Use o vínculo manual."
			}
			choice.CPE = d.ID
		}
	}
	cr := st.CredentialDefaults[string(RoleCPE)]
	if choice.CPE != "" {
		d := device(&st, choice.CPE)
		cr = d.CredentialID
		if r, _, c := s.currentRadioProposalFamily(q, d); c == OK && r != nil && r.Compatible {
			family = r.Family
		}
	}
	choice.CredentialID = cr
	proposal := &RadioProposal{Choice: choice, Address: row.Address, Family: family, CredentialID: cr}
	if ap := device(&st, e.AP); ap != nil {
		proposal.APName, proposal.APAddress = ap.Name, ap.Address
	}
	if selected := credential(&st, cr); selected != nil {
		proposal.CredentialAlias = selected.Metadata.Alias
	}
	return proposal, "Match inequívoco por MAC/IP na registration-table."
}

func (s *Service) currentRadioProposalFamily(q Command, d *Device) (*RadioReading, *RadioProfile, Code) {
	store, ok := s.Store.(RadioStore)
	if !ok {
		return nil, nil, Unavailable
	}
	return store.RadioCurrent(context.Background(), q.Scope.Profile, d.ID)
}
