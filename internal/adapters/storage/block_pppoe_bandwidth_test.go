package storage

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"infradesktop/internal/adapters/vault"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/inventory"
	"infradesktop/internal/modules/pppoe"
)

type blockBandwidthRemote struct{ equipment.Remote }

func (blockBandwidthRemote) ReadPP(_ context.Context, _ equipment.Device, _, _, scope string,
	target *equipment.PPReadTarget, _ func(*equipment.PPReadResult) equipment.Code, _ func() equipment.Code,
) (*equipment.PPReadResult, equipment.Code) {
	return &equipment.PPReadResult{Scope: scope, PayloadBytes: 100, MetadataBytes: 50, MaxRowBytes: 50,
		Rows: []equipment.PPReadRow{{RowID: target.Rows[0].ID, Download: &equipment.PPRate{Min: 2000, Max: 2000}}}}, equipment.OK
}

func TestSavedPPPoEBandwidthStillWorksAfterBlock(t *testing.T) {
	j, target := ppStoreSetup(t)
	ctx := context.Background()
	if code := j.st.UpdateEquipment(ctx, companyX, func(st *equipment.State) equipment.Code {
		st.Devices[0].Fingerprint = "synthetic-pin"
		return equipment.OK
	}); code != equipment.OK {
		t.Fatal(code)
	}
	target, code := j.st.ReadPPTarget(ctx, companyX, target.Device.ID, true)
	if code != equipment.OK {
		t.Fatal(code)
	}
	provider := vault.New(func() ([]byte, error) { return bytes.Repeat([]byte{37}, 32), nil })
	inv := inventory.New(j.svc, j.st, provider, blockBandwidthRemote{}, equipmentIDs{}, time.Now)
	defer inv.Close()
	saved := ppSynthetic(t, j, target, 1)
	_, err := j.st.unit.Exec(`UPDATE pppoe_observations SET meta_json=json_set(meta_json,
 '$.provenance.epoch',?,'$.provenance.revision',?,'$.provenance.credential_revision',?,
 '$.provenance.connection',?,'$.provenance.sample.known',json('true'),
 '$.provenance.sample.min',0,'$.provenance.sample.max',0) WHERE id=?`,
		inv.Epoch, target.Device.Revision, target.Credential.Metadata.Revision, equipment.PPConnection(target.Device), saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = j.st.unit.Exec(`UPDATE pppoe_rows SET row_json=json_set(row_json,'$.session_id','synthetic',
 '$.state','ONLINE','$.uptime.known',json('true'),'$.uptime.min',1,'$.uptime.max',1) WHERE observation_id=?`, saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	block := "Torre sintetica"
	q := equipmentScope(j)
	q.Op = "inventory.bulk-update"
	q.Operation = uuid.NewString()
	q.Bulk = &equipment.BulkUpdate{Block: &block, Items: []equipment.BulkItem{{ID: target.Device.ID, Revision: target.Device.Revision}}}
	equipmentCall(t, newEquipment(t, j), q, equipment.OK)
	service := pppoe.New(inv, nil)
	q.Op = "pppoe.read-start"
	q.Operation = ""
	input := equipment.PPInput{DeviceID: target.Device.ID, Revision: target.Device.Revision,
		Operation: uuid.NewString(), Scope: "bandwidth", ObservationID: saved.ID, RowIDs: []string{saved.Rows[0].ID}}
	v, code := service.HandlePP(ctx, q, input)
	if code != equipment.OK || v.Job == nil {
		t.Fatal("bandwidth start", code)
	}
	inv.WG.Wait()
	q.Op = "pppoe.read-status"
	result, code := service.HandlePP(ctx, q, equipment.PPInput{JobID: v.Job.ID})
	if code != equipment.OK || result.Job == nil || result.Job.Code != equipment.OK || result.Read == nil ||
		len(result.Read.Rows) != 1 || result.Read.Rows[0].Download == nil || result.Read.Rows[0].Download.Min != 2000 {
		t.Fatal("bandwidth result", code, result.Job, result.Read)
	}
}
