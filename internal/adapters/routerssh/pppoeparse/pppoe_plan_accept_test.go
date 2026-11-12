package pppoeparse_test

// Aceite QA2-A (decisão A1, 12/11/2026): o plano vem do secret mesmo desativado
// (cópia do SGP, autenticação pelo RADIUS) e a velocidade do perfil acompanha,
// para a tela comparar com a fila aplicada. Escrito por Claude; não editar.

import (
	"infradesktop/internal/adapters/routerssh/pppoeparse"
	"infradesktop/internal/modules/equipment"
	"strings"
	"testing"
)

func qa2aContextLine(profile, state, rate string) string {
	return "T\t*A\t<pppoe-synthetic>\tsynthetic\t02:00:00:00:00:01\t198.51.100.1\t1h1s\tservice\tether1\t?\t?\t?\t" +
		profile + "\t5M/10M\t\tunavailable\t" + state + "\t" + rate
}

func TestQA2APlanComesFromSingleSecretEvenWhenDisabled(t *testing.T) {
	cases := []struct {
		name, profile, state, rate    string
		wantProfile, wantUp, wantDown string
	}{
		{"secret desativado do SGP", "SGP-PLANO-ID3", "disabled", "11M/30M", "SGP-PLANO-ID3", "11000000", "30000000"},
		{"secret ativo local", "Plano 100M", "active", "20M/100M", "Plano 100M", "20000000", "100000000"},
		{"perfil com burst usa só a primeira dupla", "Plano 50M", "disabled",
			"10M/50M 15M/60M 8M/40M 8/8 8", "Plano 50M", "10000000", "50000000"},
		{"perfil sem rate-limit", "perfil-clientes", "disabled", "", "perfil-clientes", "", ""},
		{"ambíguo nunca escolhe", "Plano 100M", "ambiguous", "20M/100M", "", "", ""},
		{"sem secret", "", "none", "", "", "", ""},
		{"leitura do secret falhou", "Plano 100M", "unavailable", "20M/100M", "", "", ""},
	}
	for _, version := range []string{"6.49.18", "7.14.3"} {
		for _, c := range cases {
			body := "C\t1\t" + version + "\t2d1h1s\n" + qa2aContextLine(c.profile, c.state, c.rate) + "\nZ\t1\tcomplete\t2d1h1s\n"
			rows, _, _, code := pppoeparse.ParsePPContext(body)
			if code != equipment.OK || len(rows) != 1 {
				t.Fatalf("%s/%s: linha recusada: %s", version, c.name, code)
			}
			ctx := rows[0].Context
			if ctx.SecretState != c.state {
				t.Fatalf("%s/%s: estado do secret %q", version, c.name, ctx.SecretState)
			}
			if ctx.Profile != c.wantProfile {
				t.Fatalf("%s/%s: plano %q, esperado %q", version, c.name, ctx.Profile, c.wantProfile)
			}
			if ctx.ProfileUploadLimit != c.wantUp || ctx.ProfileDownloadLimit != c.wantDown {
				t.Fatalf("%s/%s: velocidade do plano %q/%q, esperada %q/%q", version, c.name,
					ctx.ProfileUploadLimit, ctx.ProfileDownloadLimit, c.wantUp, c.wantDown)
			}
			if ctx.UploadLimit != "5000000" || ctx.DownloadLimit != "10000000" {
				t.Fatalf("%s/%s: a fila aplicada mudou: %q/%q", version, c.name, ctx.UploadLimit, ctx.DownloadLimit)
			}
		}
	}
}

func TestQA2AContextScriptReadsProfileRateWithoutPrivateFields(t *testing.T) {
	s := pppoeparse.PPContextCommand
	if !strings.Contains(s, "rate-limit") {
		t.Fatal("o script não lê a velocidade do perfil")
	}
	for _, forbidden := range []string{"password", "comment", "/ppp secret print"} {
		if strings.Contains(s, forbidden) {
			t.Fatal("o script pede campo privado:", forbidden)
		}
	}
}
