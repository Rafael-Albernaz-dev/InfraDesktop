package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"infradesktop/internal/modules/equipment"
	"time"
)

func validRadioPublication(device equipment.Device, credential uint64, reading *equipment.RadioReading) bool {
	return reading != nil && reading.DeviceID == device.ID && reading.Revision == device.Revision &&
		reading.CredentialRevision == credential && len(reading.Clients) <= equipment.RadioMaxClients && len(reading.Interfaces) <= 32
}

func radioPublicationMatchesTarget(target equipment.PPTarget, device equipment.Device, credential uint64) bool {
	return target.Device.Active && target.Device.Revision == device.Revision &&
		equipment.PPConnection(target.Device) == equipment.PPConnection(device) && target.Credential.Metadata.Revision == credential
}

func sameRadioHistory(old, reading *equipment.RadioReading, credential uint64) bool {
	return old != nil && old.Connection == reading.Connection &&
		old.Revision == reading.Revision && old.CredentialRevision == credential
}

func inheritRadioHistory(old, reading *equipment.RadioReading, credential uint64) equipment.Code {
	matching := sameRadioHistory(old, reading, credential)
	if matching {
		before, firstErr := time.Parse(time.RFC3339Nano, old.At)
		after, secondErr := time.Parse(time.RFC3339Nano, reading.At)
		if firstErr != nil || secondErr != nil || !after.After(before) {
			return equipment.Revision
		}
		reading.HadClients = old.HadClients || len(old.Clients) > 0
	}
	reading.HadClients = reading.HadClients || len(reading.Clients) > 0
	for i := range reading.Interfaces {
		current := &reading.Interfaces[i]
		current.HadClients = current.Clients != nil && *current.Clients > 0
		if matching {
			inheritRadioInterfaceHistory(current, old.Interfaces)
		}
	}
	return equipment.OK
}

func inheritRadioInterfaceHistory(current *equipment.RadioInterface, previous []equipment.RadioInterface) {
	for _, before := range previous {
		if before.Name == current.Name && before.BSSID == current.BSSID {
			current.HadClients = current.HadClients || before.HadClients || before.Clients != nil && *before.Clients > 0
		}
	}
}

func encodeRadioPublication(ctx context.Context, tx *sql.Tx, device string,
	reading *equipment.RadioReading) ([]byte, equipment.Code) {
	body, err := json.Marshal(reading)
	if err != nil || len(body) > equipment.PPObservationBytes {
		return nil, equipment.Limit
	}
	var previous int
	err = tx.QueryRowContext(ctx, "SELECT payload_bytes FROM radio_current WHERE device_id=?", device).Scan(&previous)
	if err != nil && err != sql.ErrNoRows {
		return nil, equipment.Unavailable
	}
	total, err := ppTotalBytes(ctx, tx)
	if err != nil {
		return nil, equipment.Unavailable
	}
	if total-int64(previous)+int64(len(body)) > equipment.PPCompanyBytes {
		return nil, equipment.Limit
	}
	return body, equipment.OK
}

func applyRadioProfileContext(profile *equipment.RadioProfile, reading *equipment.RadioReading,
	device equipment.Device, credential uint64) {
	if profile.Family != reading.Family || profile.Revision != device.Revision ||
		profile.CredentialRevision != credential || profile.Connection != reading.Connection {
		profile.Family, profile.Confirmed = reading.Family, false
	}
	profile.Revision, profile.CredentialRevision, profile.Connection = device.Revision, credential, reading.Connection
}

func writeRadioPublication(ctx context.Context, tx *sql.Tx, device, reading string,
	body []byte, profile *equipment.RadioProfile) equipment.Code {
	encodedProfile, err := json.Marshal(profile)
	if err != nil {
		return equipment.Unavailable
	}
	const saveCurrent = "INSERT INTO radio_current VALUES(?,?,?,?) ON CONFLICT(device_id) DO UPDATE SET " +
		"reading_id=excluded.reading_id,body=excluded.body,payload_bytes=excluded.payload_bytes"
	if _, err = tx.ExecContext(ctx, saveCurrent, device, reading, string(body), len(body)); err != nil {
		return equipment.Unavailable
	}
	const saveProfile = "INSERT INTO radio_profiles VALUES(?,?) ON CONFLICT(device_id) DO UPDATE SET body=excluded.body"
	if _, err = tx.ExecContext(ctx, saveProfile, device, string(encodedProfile)); err != nil {
		return equipment.Unavailable
	}
	return equipment.OK
}

func radioStoredProfileMatches(p *equipment.RadioProfile, target equipment.PPTarget, code equipment.Code) bool {
	return target.Device.Active && code != equipment.MissingCredential && p.Revision == target.Device.Revision &&
		p.CredentialRevision == target.Credential.Metadata.Revision && p.Connection == equipment.PPConnection(target.Device)
}
func validRadioConfirmationSettings(family equipment.RadioFamily, warning, critical int) bool {
	return equipment.IsRadioFamily(family) && family != equipment.RadioUnsupported && warning >= 1 &&
		critical > warning && critical <= equipment.RadioMaxClients
}
func radioConfirmationMatches(r *equipment.RadioReading, t equipment.PPTarget, d equipment.Device,
	credential uint64, family equipment.RadioFamily) bool {
	return r != nil && r.Family == family && t.Device.Revision == d.Revision && t.Credential.Metadata.Revision == credential &&
		r.Revision == d.Revision && r.CredentialRevision == credential && r.Connection == equipment.PPConnection(t.Device)
}
