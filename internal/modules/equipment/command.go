package equipment

import "infradesktop/internal/modules/access"

type Command struct {
	Authority                                                       uint64
	Op                                                              string
	Scope                                                           access.Command
	UID                                                             uint32
	Operation, ID, Name, Address, CredentialID, Alias, User, Secret string
	Transport                                                       Transport
	Role                                                            Role
	Revision                                                        uint64
	Port                                                            int
	Active                                                          bool
	Inactive                                                        bool
	Cursor                                                          int
	Search, Block                                                   string
	Affected                                                        int
	JobID, Fingerprint                                              string
}
