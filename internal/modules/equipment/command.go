package equipment

import "infradesktop/internal/modules/access"

type Command struct {
	Import                                                          *ImportInput
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
	Cursor, Affected                                                int
	State, Search, Block, Model                                     string
	JobID, Fingerprint                                              string
}
