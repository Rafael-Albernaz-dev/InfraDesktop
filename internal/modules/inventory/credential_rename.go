package inventory

// Alias-only updates preserve the sealed connection identity, its readings and running jobs.
func renameCredentialOnly(st *State, old *SealedCredential, q Command, v *View) (bool, Code) {
	if old == nil || q.Secret != "" || q.User != old.Metadata.User || q.Alias == old.Metadata.Alias {
		return false, OK
	}
	if code := validateCredentialIdentity(q.Alias, q.User); code != OK {
		return true, code
	}
	if !credentialEditMatches(st, old, q) {
		return true, Revision
	}
	old.Metadata.Alias = q.Alias
	metadata := old.Metadata
	metadata.Links = CredentialLinks(st, q.ID)
	v.Credential = &metadata
	return true, OK
}
func validateCredentialIdentity(alias, user string) Code {
	if !ValidText(alias, 80) {
		return InvalidCredentialAlias
	}
	if !ValidText(user, 80) {
		return InvalidCredentialUser
	}
	return OK
}
func credentialEditMatches(st *State, old *SealedCredential, q Command) bool {
	return old.Metadata.Revision == q.Revision && CredentialLinks(st, q.ID) == q.Affected
}
func mutationKeptRevision(q Command, v *View) bool {
	if q.Op == "credential.put" {
		return v.Credential != nil && v.Credential.Revision == q.Revision
	}
	return blockUpdateKeptRevision(q, v)
}
