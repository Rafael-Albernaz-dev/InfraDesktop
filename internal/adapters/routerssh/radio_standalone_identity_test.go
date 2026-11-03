package routerssh_test

import (
	"context"
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/testsupport/sshfixture"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

func standaloneIdentity(raw string) string {
	head, _, _ := strings.Cut(raw, "L07-AVULSO-HEALTH")
	head = strings.ReplaceAll(head, "L07-AVULSO-RESOURCE", "L07-ID-RESOURCE")
	head = strings.ReplaceAll(head, "L07-AVULSO-PACKAGES", "L07-ID-PACKAGES")
	head = strings.ReplaceAll(head, "L07-AVULSO-BOARD", "L07-ID-BOARD")
	return head + "L07-ID-DONE\n"
}

func TestStandaloneStationSSHComPin(t *testing.T) {
	b, e := os.ReadFile("testdata/l07_avulso_station_v6.txt")
	if e != nil {
		t.Fatal(e)
	}
	peer, e := sshfixture.Start(sshfixture.Config{ServerVersion: "SSH-2.0-ROSSSH", Command: func(cmd string) (string, bool) {
		if cmd == routerssh.RadioIdentificationCommand {
			return standaloneIdentity(string(b)), true
		}
		if cmd != routerssh.RadioStationCommand {
			t.Errorf("comando inesperado")
			return "", false
		}
		return string(b), true
	}})
	if e != nil {
		t.Fatal(e)
	}
	defer peer.Close()
	d := equipment.Device{Role: equipment.RoleCPE, Address: "127.0.0.1", Port: peer.Port(), Fingerprint: peer.Fingerprint()}
	r,
		c := (routerssh.Adapter{}).ReadRadio(context.Background(),
		d,
		"reader",
		sshfixture.Password,
		equipment.RadioStandalone,
		func() equipment.Code { return equipment.OK })
	if c != equipment.OK || r == nil || r.Model != "RBLHG-5HPnD" || peer.Commands.Load() != 2 {
		t.Fatalf("code=%s reading=%v commands=%d", c, r, peer.Commands.Load())
	}
	d.Fingerprint = "SHA256:chave-alterada"
	_,
		c = (routerssh.Adapter{}).ReadRadio(context.Background(),
		d,
		"reader",
		sshfixture.Password,
		equipment.RadioStandalone,
		func() equipment.Code { return equipment.OK })
	if c != equipment.TrustChanged || peer.Commands.Load() != 2 {
		t.Fatalf("pin alterado code=%s commands=%d", c, peer.Commands.Load())
	}
}

func TestStandaloneFamiliaDesconhecidaNaoUsaWOM(t *testing.T) {
	peer, e := sshfixture.Start(sshfixture.Config{ServerVersion: "SSH-2.0-unrecognized", Command: func(cmd string) (string, bool) {
		if (!strings.Contains(cmd, "mca-status") && cmd != routerssh.RadioWOMIdentificationCommand) || strings.Contains(cmd, "iw dev") {
			t.Error("outro driver foi consultado")
		}
		return "L07-DRIVER-ABSENT\n", true
	}})
	if e != nil {
		t.Fatal(e)
	}
	defer peer.Close()
	d := equipment.Device{Role: equipment.RoleCPE, Address: "127.0.0.1", Port: peer.Port(), Fingerprint: peer.Fingerprint()}
	r,
		c := (routerssh.Adapter{}).ReadRadio(context.Background(),
		d,
		"reader",
		sshfixture.Password,
		equipment.RadioStandalone,
		func() equipment.Code { return equipment.OK })
	if c != equipment.RadioCodeUnsupported || r == nil || r.Family != equipment.RadioUnsupported || peer.Commands.Load() != 2 {
		t.Fatalf("unknown family code=%s reading=%v commands=%d", c, r, peer.Commands.Load())
	}
}

func TestStandaloneRouterOSSemDriverPreservaIdentidade(t *testing.T) {
	b, e := os.ReadFile("testdata/l07_avulso_station_v6.txt")
	if e != nil {
		t.Fatal(e)
	}
	output := strings.ReplaceAll(string(b), "6.49.18 (long-term)", "7.12.1")
	peer, e := sshfixture.Start(sshfixture.Config{ServerVersion: "SSH-2.0-ROSSSH", Command: func(cmd string) (string, bool) {
		if cmd != routerssh.RadioIdentificationCommand {
			t.Error("driver wireless acessado antes de identificar v7")
			return "bad command name", false
		}
		return standaloneIdentity(output), true
	}})
	if e != nil {
		t.Fatal(e)
	}
	defer peer.Close()
	d := equipment.Device{Role: equipment.RolePTP, Address: "127.0.0.1", Port: peer.Port(), Fingerprint: peer.Fingerprint()}
	r,
		c := (routerssh.Adapter{}).ReadRadio(context.Background(),
		d,
		"reader",
		sshfixture.Password,
		equipment.RadioStandalone,
		func() equipment.Code { return equipment.OK })
	if c != equipment.RadioCodeUnsupported ||
		r == nil ||
		r.Family != equipment.RadioUnsupported ||
		r.Version != "7.12.1" ||
		r.Model != "RBLHG-5HPnD" {
		t.Fatalf("code=%s reading=%+v", c, r)
	}
}

func TestStandaloneAPPreservaIdentidadeModoProtocolo(t *testing.T) {
	station := recoveryGolden(t, "l07_avulso_station_v6.txt")
	station = strings.Replace(station, "mode=station-bridge", "mode=ap-bridge", 1)
	rich := strings.ReplaceAll(recoveryGolden(t, "l07_ap_v6_real.txt"), "6.49.10 (long-term)", "6.49.18 (long-term)")
	peer, e := sshfixture.Start(sshfixture.Config{ServerVersion: "SSH-2.0-ROSSSH", Command: func(cmd string) (string, bool) {
		switch cmd {
		case routerssh.RadioIdentificationCommand:
			return standaloneIdentity(station), true
		case routerssh.RadioStationCommand:
			return station, true
		case routerssh.RadioAPCommand:
			return rich, true
		}
		t.Errorf("comando indevido")
		return "", false
	}})
	if e != nil {
		t.Fatal(e)
	}
	defer peer.Close()
	d := equipment.Device{Role: equipment.RolePTP, Address: "127.0.0.1", Port: peer.Port(), Fingerprint: peer.Fingerprint()}
	r,
		c := (routerssh.Adapter{}).ReadRadio(context.Background(),
		d,
		"reader",
		sshfixture.Password,
		equipment.RadioStandalone,
		func() equipment.Code { return equipment.OK })
	if c != equipment.OK ||
		r == nil ||
		r.Manufacturer != "MikroTik" ||
		r.Model != "RBLHG-5HPnD" ||
		r.Mode != "ap-bridge" ||
		r.WirelessProtocol != "nv2-nstreme-802.11" {
		t.Fatalf("code=%s reading=%+v", c, r)
	}
}

func TestStandaloneAirOSDesconectadoSemDriverIdentifica(t *testing.T) {
	peer, e := sshfixture.Start(sshfixture.Config{ServerVersion: "SSH-2.0-unrecognized", Command: func(cmd string) (string, bool) {
		if !strings.Contains(cmd, "mca-status") || strings.Contains(cmd, "iw dev") {
			t.Error("comando não é identificação airOS fechada")
		}
		return "firmwareVersion=WA.v8.7.0,platform=AC-LAB,wlanOpmode=ap\n123.0 60.0\n", true
	}})
	if e != nil {
		t.Fatal(e)
	}
	defer peer.Close()
	d := equipment.Device{Role: equipment.RolePTP, Address: "127.0.0.1", Port: peer.Port(), Fingerprint: peer.Fingerprint()}
	r,
		c := (routerssh.Adapter{}).ReadRadio(context.Background(),
		d,
		"reader",
		sshfixture.Password,
		equipment.RadioStandalone,
		func() equipment.Code { return equipment.OK })
	if c != equipment.RadioCodeUnsupported || r == nil || r.Manufacturer != "Ubiquiti" || r.Version != "WA.v8.7.0" || r.Model != "AC-LAB" {
		t.Fatalf("code=%s reading=%+v", c, r)
	}
}

func TestStandaloneRevogacaoAntesDaSenha(t *testing.T) {
	peer, e := sshfixture.Start(sshfixture.Config{ServerVersion: "SSH-2.0-ROSSSH"})
	if e != nil {
		t.Fatal(e)
	}
	defer peer.Close()
	d := equipment.Device{Role: equipment.RolePTP, Address: "127.0.0.1", Port: peer.Port(), Fingerprint: peer.Fingerprint()}
	var checks atomic.Int32
	_,
		c := (routerssh.Adapter{}).ReadRadio(context.Background(),
		d,
		"reader",
		sshfixture.Password,
		equipment.RadioStandalone,
		func() equipment.Code {
			if checks.Add(1) >= 3 {
				return access.Unauthorized
			}
			return equipment.OK
		})
	if c != access.Unauthorized || peer.Auth.Load() != 0 || peer.Commands.Load() != 0 {
		t.Fatalf("code=%s auth=%d commands=%d", c, peer.Auth.Load(), peer.Commands.Load())
	}
}

func TestStandaloneAirOSSelecionaDriverEConservaModo(t *testing.T) {
	raw := recoveryGolden(t, "l07_airos_xw_6_3_6.txt")
	peer, err := sshfixture.Start(sshfixture.Config{ServerVersion: "SSH-2.0-OpenSSH", Command: func(cmd string) (string, bool) {
		if !strings.Contains(cmd, "mca-status") || strings.Contains(cmd, "iw dev") {
			t.Error("identificação indevida")
		}
		return raw, true
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	d := equipment.Device{Role: equipment.RoleCPE, Address: "127.0.0.1", Port: peer.Port(), Fingerprint: peer.Fingerprint()}
	r,
		c := (routerssh.Adapter{}).ReadRadio(context.Background(),
		d,
		"reader",
		sshfixture.Password,
		equipment.RadioStandalone,
		func() equipment.Code { return equipment.OK })
	if c != equipment.OK || r == nil || r.Family != equipment.RadioAirOSV6 || r.Manufacturer != "Ubiquiti" || r.Mode != "sta" {
		t.Fatalf("code=%s reading=%+v", c, r)
	}
}
