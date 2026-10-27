package routerssh_test

import (
	"encoding/json"
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/modules/equipment"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestL06DSecretStateProjection(t *testing.T) {
	for _, version := range []string{"6.49.18", "7.14.3"} {
		for _, state := range []string{"disabled", "active", "ambiguous", "none", "unavailable"} {
			body := "C\t1\t" + version + "\t2d1h1s\nT\t*A\t<pppoe-synthetic>\tsynthetic\t02:00:00:00:00:01\t198.51.100.1\t1h1s\tservice\tether1\t?\t?\t?\tdefault\t5M/10M\t\tunavailable\t" + state + "\nZ\t1\tcomplete\t2d1h1s\n"
			rows, _, _, code := routerssh.ParsePPContext(body)
			if code != equipment.OK || len(rows) != 1 {
				t.Fatalf("%s/%s: explicit secret state rejected: %s", version, state, code)
			}
			b, _ := json.Marshal(rows[0].Context)
			var fields map[string]any
			json.Unmarshal(b, &fields)
			if fields["secret_state"] != state || (state == "active") != (rows[0].Context.Profile == "default") {
				t.Fatalf("state/profile attribution %s: %s", state, b)
			}
			if rows[0].Context.UploadLimit != "5000000" || rows[0].Context.DownloadLimit != "10000000" {
				t.Fatal("queue order lost")
			}
			for _, limit := range []string{"", "?", "5M/10M/20M"} {
				missing, _, _, c := routerssh.ParsePPContext(strings.Replace(body, "5M/10M", limit, 1))
				if c != equipment.OK || missing[0].Context.UploadLimit != "" || missing[0].Context.DownloadLimit != "" {
					t.Fatal("unavailable/ambiguous queue became a known limit")
				}
			}
		}
	}
}

func TestL06DSecretScriptFiltersActiveBeforeProfile(t *testing.T) {
	s := routerssh.PPContextCommand
	if !strings.Contains(s, `name=$ppContextUser and disabled!=yes`) || !strings.Contains(s, `:set ppContextSecretState "disabled"`) || !strings.Contains(s, `:set ppContextSecretState "ambiguous"`) {
		t.Fatal("disabled secrets still eligible for profile or state not projected")
	}
	for _, forbidden := range []string{"password", "comment", "/ppp secret print"} {
		if strings.Contains(s, forbidden) {
			t.Fatal("stable context requests private field", forbidden)
		}
	}
}

func TestL06DSecretEvidenceClosedProjection(t *testing.T) {
	command, ok := routerssh.BuildPPSecretCommand(`a.$"; /system reboot`)
	if !ok || !strings.Contains(command, `:local ppSecretLogin "a.\$\"; /system reboot"`) || !strings.Contains(command, `[/ppp secret find where name=$ppSecretLogin]`) {
		t.Fatal("login not a quoted exact literal")
	}
	for _, bad := range []string{"password", "/ppp secret print", "as-value", " get $ppSecretID value-name", ":local disabled "} {
		if strings.Contains(command, bad) {
			t.Fatal("unsafe secret projection", bad)
		}
	}
	for _, property := range []string{"disabled", "service", "profile", "last-logged-out", "last-caller-id", "last-disconnect-reason", "comment"} {
		if !strings.Contains(command, "get $ppSecretID "+property+"]") {
			t.Fatal("missing projected scalar", property)
		}
	}
	for _, version := range []string{"6.49.18", "7.14.3"} {
		header := "S\t1\t" + version + "\t2d1h1s\n"
		ordered, _, orderCode := routerssh.ParsePPSecret(header + "E\ttrue\t=pppoe\t=zzz\t?\t?\t?\t?\nE\ttrue\t=pppoe\t=aaa\t?\t?\t?\t?\nZ\t2\tcomplete\t2d1h1s\n")
		if orderCode != equipment.OK || *ordered[0].Secret.Profile != "aaa" {
			t.Fatal("evidence order depends on router enumeration")
		}
		for _, n := range []int{0, 1, 2, 4} {
			body := header + strings.Repeat("E\ttrue\t=pppoe\t=default\t?\t=\t?\t="+strings.Repeat("á", 130)+"\\nnota\n", n) + "Z\t" + string(rune('0'+n)) + "\tcomplete\t2d1h1s\n"
			rows, gotVersion, c := routerssh.ParsePPSecret(body)
			if c != equipment.OK || gotVersion != version || len(rows) != n {
				t.Fatal("closed evidence parsing", version, n, c)
			}
			for _, row := range rows {
				e := row.Secret
				if row.Kind != "secret" || e == nil || e.Disabled == nil || !*e.Disabled || e.LastOut != nil || e.LastCaller == nil || *e.LastCaller != "" || e.Comment == nil || utf8.RuneCountInString(*e.Comment) != 128 || strings.ContainsAny(*e.Comment, "\r\n\t") {
					t.Fatal("availability or comment sanitization lost", row)
				}
			}
		}
		for _, body := range []string{header + "Z\t0\trows\t2d1h1s\n", header + "E\ttrue\t=x\t=y\t?\t?\t?\t=z\n", header + "Z\t1\tcomplete\t2d1h1s\n", header + strings.Repeat("E\t?\t?\t?\t?\t?\t?\t?\n", 5) + "Z\t5\tcomplete\t2d1h1s\n"} {
			if rows, _, c := routerssh.ParsePPSecret(body); c == equipment.OK || len(rows) > 0 {
				t.Fatal("partial/oversized evidence accepted")
			}
		}
	}
}
