package inventory

func (s *Service) mutateCredentialPut(st *State, q Command, v *View) Code {
	if !s.VaultReady(st) {
		return VaultUnavailable
	}
	old := CredentialByID(st, q.ID)
	if handled, code := renameCredentialOnly(st, old, q, v); handled {
		return code
	}
	if c := s.preserveCredentialSecret(st, old, &q); c != OK {
		return c
	}
	if c := ValidateCredential(q.Alias, q.User, q.Secret); c != OK {
		return c
	}
	revision, c := nextCredentialRevision(st, old, q)
	if c != OK {
		return c
	}
	envelope, wrapped, c := s.Vault.Seal(st.Installation, st.Company, q.ID, revision, q.Secret, st.WrappedKey)
	if c != OK {
		return c
	}
	st.WrappedKey = wrapped
	item := SealedCredential{
		Metadata: Credential{ID: q.ID, Alias: q.Alias, User: q.User, Revision: revision},
		Envelope: envelope,
	}
	saveCredentialItem(st, old, item, q.ID)
	m := item.Metadata
	m.Links = CredentialLinks(st, q.ID)
	v.Credential = &m
	return OK
}

func nextCredentialRevision(st *State, old *SealedCredential, q Command) (uint64, Code) {
	if old != nil {
		if !credentialEditMatches(st, old, q) {
			return 0, Revision
		}
		return old.Metadata.Revision + 1, OK
	}
	if q.Revision != 0 {
		return 0, Revision
	}
	if len(st.Credentials) >= MaxCredentials {
		return 0, Limit
	}
	return 1, OK
}

func saveCredentialItem(st *State, old *SealedCredential, item SealedCredential, id string) {
	if old != nil {
		*old = item
	} else {
		st.Credentials = append(st.Credentials, item)
	}
	for i := range st.Devices {
		if st.Devices[i].CredentialID == id {
			st.Devices[i].Result = nil
		}
	}
}
