package inventory

import (
	"context"
	"fmt"
	"infradesktop/internal/modules/equipment"
	"net/netip"
	"strings"
)

func canonicalizeMutation(q *Command) Code {
	if q.Op == "inventory.enroll" || q.Op == "inventory.create" || q.Op == "inventory.update" {
		q.Name = strings.TrimSpace(q.Name)
		if q.Role == "" {
			q.Role = RoleOther
		}
		if !equipment.ValidInventoryRole(q.Role) {
			return Invalid
		}
		q.Address = strings.TrimSpace(q.Address)
		if ip, e := netip.ParseAddr(q.Address); e == nil {
			q.Address = ip.String()
		}
	}
	if q.Op == "inventory.enroll" || q.Op == "credential.put" {
		q.User = strings.TrimSpace(q.User)
		q.Alias = strings.TrimSpace(q.Alias)
	}
	if !ValidID(q.Operation) || !ValidID(q.ID) {
		return Invalid
	}
	return OK
}

func buildMutationStamp(q Command, secretStamp string) string {
	parts := []string{
		q.Op, q.ID, q.Name, q.Address, fmt.Sprint(q.Port), string(q.Transport),
		q.CredentialID, fmt.Sprint(q.Revision), fmt.Sprint(q.Active),
		q.Alias, q.User, secretStamp, fmt.Sprint(q.Affected),
	}
	stamp := fmt.Sprintf("%q", parts)
	if q.Block != "" {
		stamp += fmt.Sprintf(";block:%q", q.Block)
	}
	if q.Role != "" && q.Role != RoleOther {
		stamp += ";role:" + string(q.Role)
	}
	return stamp
}

func (s *Service) mutate(ctx context.Context, q Command, v *View) Code {
	if c := canonicalizeMutation(&q); c != OK {
		return c
	}
	secretStamp := ""
	if q.Op == "credential.put" || q.Op == "inventory.enroll" {
		var c Code
		secretStamp, c = s.Vault.Stamp(q.Secret)
		if c != OK {
			return c
		}
	}
	stamp := buildMutationStamp(q, secretStamp)
	mutated := false
	c := s.Store.UpdateEquipment(ctx, q.Scope.Profile, func(st *State) Code {
		return s.applyMutationState(st, q, &stamp, v, &mutated)
	})
	if c == OK && mutated && !mutationKeptRevision(q, v) {
		s.Invalidate(q.Scope.Profile, q.ID, q.Op == "credential.put" || q.Op == "credential.delete")
	}
	return c
}

func (s *Service) applyMutationState(st *State, q Command, stamp *string, v *View, mutated *bool) Code {
	if replayed, c := checkMutationReplay(st, q, *stamp, v); replayed {
		return c
	}
	if len(st.Operations) >= MaxOperations {
		return Limit
	}
	if c := s.dispatchMutationOp(st, q, stamp, v); c != OK {
		return c
	}
	st.Operations = append(st.Operations, Operation{ID: q.Operation, Stamp: *stamp, Entity: q.ID})
	*mutated = true
	return OK
}

func checkMutationReplay(st *State, q Command, stamp string, v *View) (bool, Code) {
	for _, op := range st.Operations {
		if op.ID != q.Operation {
			continue
		}
		if !matchingMutationStamp(op.Stamp, stamp, q.Op) {
			return true, Revision
		}
		if q.Op == "credential.put" {
			r := CredentialByID(st, op.Entity)
			if r == nil {
				return true, Unavailable
			}
			m := r.Metadata
			m.Links = CredentialLinks(st, m.ID)
			v.Credential = &m
		} else {
			v.Device = DeviceByID(st, op.Entity)
		}
		return true, OK
	}
	return false, OK
}

func matchingMutationStamp(recorded, expected, op string) bool {
	if recorded == expected {
		return true
	}
	return op == "trust.reset" && strings.HasPrefix(recorded, expected+";previous:")
}

func (s *Service) dispatchMutationOp(st *State, q Command, stamp *string, v *View) Code {
	switch q.Op {
	case "credential.default-set":
		return mutateCredentialDefault(st, q, v)
	case "credential.put":
		return s.mutateCredentialPut(st, q, v)
	case "credential.delete":
		return deleteCredential(st, q)
	case "inventory.enroll":
		return s.mutateEnroll(st, q, v)
	case "trust.reset":
		return mutateTrustReset(st, q, stamp, v)
	case "inventory.create", "inventory.update", "inventory.set-active":
		return mutateDeviceSave(st, q, v)
	default:
		return Invalid
	}
}

func mutateCredentialDefault(st *State, q Command, v *View) Code {
	if !validRadioDefaultRole(q.Role) {
		return Invalid
	}
	if CredentialByID(st, q.CredentialID) == nil {
		return MissingCredential
	}
	if st.CredentialDefaults == nil {
		st.CredentialDefaults = map[string]string{}
	}
	st.CredentialDefaults[string(q.Role)] = q.CredentialID
	v.CredentialDefaults = st.CredentialDefaults
	return OK
}

func (s *Service) mutateEnroll(st *State, q Command, v *View) Code {
	if !s.VaultReady(st) {
		return VaultUnavailable
	}
	if !canEnrollNewDevice(st, q) {
		return Revision
	}
	if len(st.Devices) >= MaxDevices || len(st.Credentials) >= MaxCredentials {
		return Limit
	}
	d := Device{
		Role: q.Role, ID: q.ID, Name: q.Name, Block: q.Block, Address: q.Address, Port: q.Port,
		Transport: q.Transport, CredentialID: q.ID, Revision: 1, Active: true,
	}
	if c := ValidateDevice(d); c != OK {
		return c
	}
	if c := ValidateCredential(q.Name, q.User, q.Secret); c != OK {
		return c
	}
	envelope, wrapped, c := s.Vault.Seal(st.Installation, st.Company, q.ID, 1, q.Secret, st.WrappedKey)
	if c != OK {
		return c
	}
	st.WrappedKey = wrapped
	meta := Credential{ID: q.ID, Alias: q.Name, User: q.User, Revision: 1}
	st.Credentials = append(st.Credentials, SealedCredential{Metadata: meta, Envelope: envelope})
	st.Devices = append(st.Devices, d)
	v.Device = &d
	return OK
}

func canEnrollNewDevice(st *State, q Command) bool {
	return q.Revision == 0 &&
		q.CredentialID == "" &&
		DeviceByID(st, q.ID) == nil &&
		CredentialByID(st, q.ID) == nil
}

func mutateTrustReset(st *State, q Command, stamp *string, v *View) Code {
	d := DeviceByID(st, q.ID)
	if d == nil || d.Revision != q.Revision {
		return Revision
	}
	if !d.Active {
		return Inactive
	}
	if d.Fingerprint == "" {
		return TrustRequired
	}
	*stamp += fmt.Sprintf(";previous:%q:%q", d.Fingerprint, fmt.Sprint(d.TrustAcceptance))
	d.Fingerprint = ""
	d.TrustAcceptance = nil
	d.Revision++
	d.LastAttempt = nil
	v.Device = d
	return OK
}

func mutateDeviceSave(st *State, q Command, v *View) Code {
	old := DeviceByID(st, q.ID)
	d := Device{
		Role: q.Role, ID: q.ID, Name: q.Name, Block: q.Block, Address: q.Address, Port: q.Port,
		Transport: q.Transport, CredentialID: q.CredentialID, Revision: 1, Active: true,
	}
	if q.Op == "inventory.create" {
		if c := prepareDeviceCreate(st, old, q, &d); c != OK {
			return c
		}
	} else {
		if c := prepareDeviceUpdate(old, q, &d); c != OK {
			return c
		}
	}
	if c := ValidateDevice(d); c != OK {
		return c
	}
	if d.CredentialID != "" && CredentialByID(st, d.CredentialID) == nil {
		return MissingCredential
	}
	if old != nil {
		*old = d
	} else {
		st.Devices = append(st.Devices, d)
	}
	v.Device = &d
	return OK
}

func prepareDeviceCreate(st *State, old *Device, q Command, d *Device) Code {
	if d.CredentialID == "" {
		d.CredentialID = st.CredentialDefaults[string(d.Role)]
	}
	if old != nil || q.Revision != 0 {
		return Revision
	}
	if len(st.Devices) >= MaxDevices {
		return Limit
	}
	return OK
}

func prepareDeviceUpdate(old *Device, q Command, d *Device) Code {
	if old == nil || old.Revision != q.Revision {
		return Revision
	}
	d.Revision = deviceUpdateRevision(*d, *old, q.Op)
	d.Active = old.Active
	if q.Op == "inventory.set-active" {
		*d = *old
		d.Active = q.Active
		d.Revision++
	}
	if sameDeviceEndpoint(*d, *old) {
		d.Fingerprint = old.Fingerprint
		d.TrustOrigin = old.TrustOrigin
		d.TrustAcceptance = old.TrustAcceptance
		if d.CredentialID == old.CredentialID {
			d.Result = old.Result
			d.LastAttempt = old.LastAttempt
		}
	}
	return OK
}

func sameDeviceEndpoint(a, b Device) bool {
	return a.Address == b.Address && a.Port == b.Port && a.Transport == b.Transport
}
