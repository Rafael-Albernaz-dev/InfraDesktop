package equipment

import "infradesktop/internal/modules/access"

type Command struct {
	Bulk                                                            *BulkUpdate
	Authority                                                       uint64
	Op                                                              string
	Scope                                                           access.Command
	UID                                                             uint32
	Operation, ID, Name, Address, CredentialID, Alias, User, Secret string
	Transport                                                       Transport
	Role                                                            Role
	Revision                                                        uint64
	Port                                                            int
	Active, Inactive, IncludeInactive, BlockEmpty                   bool
	Cursor                                                          int
	Search, Block, Model                                            string
	Affected                                                        int
	JobID, Fingerprint                                              string
}
