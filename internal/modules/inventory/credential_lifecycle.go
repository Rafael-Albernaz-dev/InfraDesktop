package inventory

import "infradesktop/internal/modules/equipment"

const RoleOther Role = equipment.RoleOther

func validRadioDefaultRole(r Role) bool { return r == RoleAP || r == RoleCPE || r == RolePTP }

const CredentialInUse Code = "CREDENTIAL_IN_USE"

func (s *Service) preserveCredentialSecret(st *State, old *SealedCredential, q *Command) Code {
	if q.Secret != "" || old == nil {
		return OK
	}
	secret, c := s.Vault.Open(st.Installation, st.Company, q.ID, old.Metadata.Revision, old.Envelope, st.WrappedKey)
	if c != OK {
		return c
	}
	q.Secret = secret
	return OK
}

func deleteCredential(st *State, q Command) Code {
	old := CredentialByID(st, q.ID)
	if old == nil || old.Metadata.Revision != q.Revision {
		return Revision
	}
	if CredentialLinks(st, q.ID) != 0 {
		return CredentialInUse
	}
	for _, id := range st.CredentialDefaults {
		if id == q.ID {
			return CredentialInUse
		}
	}
	for i := range st.Credentials {
		if st.Credentials[i].Metadata.ID == q.ID {
			st.Credentials = append(st.Credentials[:i], st.Credentials[i+1:]...)
			return OK
		}
	}
	return Revision
}
