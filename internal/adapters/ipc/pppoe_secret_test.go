package ipc

import (
	"context"
	"encoding/json"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/foundation"
	"infradesktop/internal/testsupport/sshfixture"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestL06DSecretEvidenceThroughIPCIsEphemeral(t *testing.T) {
	for _, family := range []string{"v6", "v7"} {
		t.Run(family, func(t *testing.T) {
			version := "7.26"
			if family == "v6" {
				version = "6.49.19"
			}
			j := newPPJourney(t, sshfixture.Config{Family: family, PPRows: 1, RealTime: true, Command: func(command string) (string, bool) {
				if !strings.Contains(command, `:local ppSecretLogin `) {
					return "", false
				}
				if strings.Contains(command, "password") || strings.Contains(command, "/ppp secret print") || !strings.Contains(command, `:local ppSecretLogin "synthetic-0000"`) {
					t.Error("unsafe projection reached SSH")
				}
				return "S\t1\t" + version + "\t2d1h1s\nE\ttrue\t=pppoe\t=default\t?\t?\t?\t=ephemeral-comment-canary\nE\ttrue\t=pppoe\t=other\t?\t?\t?\t=\nZ\t2\tcomplete\t2d1h1s\n", true
			}})
			o := j.collect(uuid.NewString())
			row := j.call("pppoe.sessions-list", equipment.PPInput{DeviceID: j.device, ObservationID: o.ID}, foundation.OK).Data.PPPoE.Rows[0]
			before, c := j.store.ReadEquipment(context.Background(), j.client.profile)
			if c != equipment.OK {
				t.Fatal(c)
			}
			in := equipment.PPInput{DeviceID: j.device, Revision: 2, Operation: uuid.NewString(), Scope: "secret", ObservationID: o.ID, RowIDs: []string{row.ID}}
			job := j.call("pppoe.read-start", in, foundation.OK).Data.PPPoE.Job
			var result *equipment.PPView
			until := time.Now().Add(3 * time.Second)
			for time.Now().Before(until) {
				result = j.call("pppoe.read-status", equipment.PPInput{JobID: job.ID}, foundation.OK).Data.PPPoE
				if result.Job.State != "RUNNING" {
					break
				}
				time.Sleep(time.Millisecond)
			}
			if result.Job.State != "SUCCEEDED" || result.Read == nil || len(result.Read.Rows) != 2 || result.Read.Rows[0].Secret == nil || *result.Read.Rows[0].Secret.Comment != "ephemeral-comment-canary" {
				t.Fatal("secret evidence path", result.Job)
			}
			raw, _ := json.Marshal(result.Read)
			if strings.Contains(string(raw), "password") || strings.Contains(string(raw), row.Login) {
				t.Fatal("secret projection escaped allowlist")
			}
			commands := j.peer.Commands.Load()
			invalid := in
			invalid.Operation = uuid.NewString()
			invalid.RowIDs = append(invalid.RowIDs, uuid.NewString())
			j.call("pppoe.read-start", invalid, foundation.Invalid)
			j.call("pppoe.read-page", equipment.PPInput{JobID: job.ID}, foundation.OK)
			if commands != j.peer.Commands.Load() {
				t.Fatal("page/invalid subjects triggered SSH")
			}
			j.call("pppoe.read-cancel", equipment.PPInput{JobID: job.ID}, foundation.OK)
			if v := j.call("pppoe.read-status", equipment.PPInput{JobID: job.ID}, foundation.OK).Data.PPPoE; v.Read != nil {
				t.Fatal("cancel retained private result")
			}
			after, c := j.store.ReadEquipment(context.Background(), j.client.profile)
			if c != equipment.OK || len(before.Operations) != len(after.Operations) {
				t.Fatal("secret consumed mutation receipt")
			}
			ex := j.call("pppoe.export-page", equipment.PPInput{DeviceID: j.device, TargetID: o.ID, ExportKey: "CQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQk"}, foundation.OK).Data.PPPoE
			raw, _ = json.Marshal(ex)
			if strings.Contains(string(raw), "ephemeral-comment-canary") || strings.Contains(string(raw), "secret") {
				t.Fatal("private result entered sanitized export")
			}
			err := filepath.WalkDir(filepath.Join(j.root, "state"), func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !d.IsDir() {
					b, e := os.ReadFile(path)
					if e != nil {
						return e
					}
					if strings.Contains(string(b), "ephemeral-comment-canary") {
						t.Error("private evidence persisted")
					}
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
