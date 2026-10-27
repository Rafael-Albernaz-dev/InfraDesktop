package routerssh_test

import (
	"context"
	"encoding/json"
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/testsupport/sshfixture"
	"math/big"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestL06DContextProjectionAndSSH(t *testing.T) {
	for _, version := range []string{"6.49.18", "7.14.3"} {
		t.Run(version, func(t *testing.T) {
			contextBody := "C\t1\t" + version + "\t2d1h1s\nT\t*A\t<pppoe-synthetic>\tsynthetic\t02:00:00:00:00:01\t198.51.100.1\t1h1s\tservice\tvlan100\t100\tfe80::1\tfe80::2\tlocal-profile\t5M/10M\t2001:db8:100::/56\tbound\tactive\nZ\t1\tcomplete\t2d1h1s\n"
			tunnels, v, _, code := routerssh.ParsePPContext(contextBody)
			if code != equipment.OK || v != version || len(tunnels) != 1 {
				t.Fatal("documented projection", code)
			}
			c := tunnels[0].Context
			if c.VLAN == nil || *c.VLAN != 100 || c.IPv6 == nil || !*c.IPv6 || c.PDStatus != "bound" || c.Profile != "local-profile" || c.UploadLimit != "5000000" || c.DownloadLimit != "10000000" {
				t.Fatal("projection fields", c)
			}
			unknown := strings.ReplaceAll(strings.ReplaceAll(contextBody, "\t100\tfe80::1\tfe80::2", "\t?\t?\t?"), "2001:db8:100::/56\tbound", "\tunavailable")
			missing, _, _, code := routerssh.ParsePPContext(unknown)
			if code != equipment.OK || missing[0].Context.VLAN != nil || missing[0].Context.IPv6 != nil || missing[0].Context.PDStatus != "unavailable" {
				t.Fatal("absent field became known", code)
			}
			for _, bad := range []string{strings.Split(contextBody, "Z\t")[0], strings.Replace(contextBody, "Z\t1\tcomplete", "Z\t0\tcomplete", 1)} {
				if _, _, _, c := routerssh.ParsePPContext(bad); c == equipment.OK {
					t.Fatal("truncated context accepted")
				}
			}
			peer, err := sshfixture.Start(sshfixture.Config{PPPoE: func() string {
				return "H\t1\t1\t" + version + "\t2d1h1s\nR\t*1\tsid\tsynthetic\t198.51.100.1\t02:00:00:00:00:01\t1h1s\nZ\t1\tcomplete\t2d1h1s\n"
			}, Command: func(command string) (string, bool) { return contextBody, command == routerssh.PPContextCommand }})
			if err != nil {
				t.Fatal(err)
			}
			defer peer.Close()
			o, code := (routerssh.Adapter{}).CollectPPPoE(context.Background(), equipment.Device{Address: "127.0.0.1", Port: peer.Port(), Fingerprint: peer.Fingerprint()}, "reader", sshfixture.Password, func() equipment.Code { return equipment.OK })
			if code != equipment.OK || o == nil || len(o.Rows) != 1 || o.Rows[0].Context == nil || o.Rows[0].Context.Service != "service" {
				t.Fatal("SSH did not enrich real adapter", code)
			}
			if peer.Commands.Load() != 2 {
				t.Fatal("unexpected commands", peer.Commands.Load())
			}
		})
	}
	for _, forbidden := range []string{" password", "/ppp secret print", "/log print", "reset-counters", " remove ", " set "} {
		if strings.Contains(routerssh.PPContextCommand, forbidden) {
			t.Fatal("forbidden command/property", forbidden)
		}
	}
}

func TestL06DContextLimitPreservesCompleteBase(t *testing.T) {
	for _, mode := range []string{"ssh-bytes", "parser-bytes", "router-rows", "router-bytes"} {
		t.Run(mode, func(t *testing.T) {
			body := strings.Repeat("x", routerssh.MaxOutput+1)
			switch mode {
			case "parser-bytes":
				body = strings.Repeat("x", equipment.PPWireBytes+1)
			case "router-rows":
				body = "C\t1\t7.14.3\t2d1h1s\nZ\t0\trows\t2d1h1s\n"
			case "router-bytes":
				body = "C\t1\t7.14.3\t2d1h1s\nZ\t0\twire_bytes\t2d1h1s\n"
			}
			peer, err := sshfixture.Start(sshfixture.Config{PPPoE: func() string {
				return "H\t1\t1\t7.14.3\t2d1h1s\nR\t*1\tsid\tsynthetic\t198.51.100.1\t02:00:00:00:00:01\t1h1s\nZ\t1\tcomplete\t2d1h1s\n"
			}, Command: func(command string) (string, bool) { return body, command == routerssh.PPContextCommand }})
			if err != nil {
				t.Fatal(err)
			}
			defer peer.Close()
			o, code := (routerssh.Adapter{}).CollectPPPoE(context.Background(), equipment.Device{Address: "127.0.0.1", Port: peer.Port(), Fingerprint: peer.Fingerprint()}, "reader", sshfixture.Password, func() equipment.Code { return equipment.OK })
			if code != equipment.OK || o == nil || len(o.Rows) != 1 || o.Rows[0].Login != "synthetic" || o.Rows[0].Context != nil || o.Quality.Status != "complete" || !o.Quality.MembershipChecked {
				t.Fatal("optional context limit rejected complete base", code)
			}
			b, _ := json.Marshal(o)
			var fields map[string]any
			json.Unmarshal(b, &fields)
			if fields["context_omitted"] != true {
				t.Fatal("optional context limit hidden as absence")
			}
			if peer.Commands.Load() != 2 || peer.Auth.Load() != 1 {
				t.Fatal("limit triggered extra requests")
			}
		})
	}
}

func TestL06DOptionalLimitCannotBypassRevocation(t *testing.T) {
	peer, err := sshfixture.Start(sshfixture.Config{PPRows: 1, Command: func(command string) (string, bool) {
		return strings.Repeat("x", routerssh.MaxOutput+1), command == routerssh.PPContextCommand
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	o, c := (routerssh.Adapter{}).CollectPPPoE(context.Background(), equipment.Device{Address: "127.0.0.1", Port: peer.Port(), Fingerprint: peer.Fingerprint()}, "reader", sshfixture.Password, func() equipment.Code {
		if peer.Commands.Load() == 2 {
			return equipment.Revision
		}
		return equipment.OK
	})
	if o != nil || c != equipment.Revision {
		t.Fatal("optional fallback bypassed revoked authority", c)
	}
}

// The literal RouterOS program is an external safety contract. This checks the
// causal branch/selection that a transcript peer cannot interpret. Native P6
// remains necessary for RouterOS execution and cost acceptance.
func TestL06DProjectionGuardsAmbiguousPDAndBoundsReads(t *testing.T) {
	script := routerssh.PPContextCommand
	if !strings.Contains(script, `:if ([:len $servers]>1) do={:set pdstate "ambiguous"}`) {
		t.Fatal("multiple DHCPv6 servers can be emitted as no delegation")
	}
	if strings.Contains(script, `:local qs [/queue simple find where dynamic=yes];`) || strings.Contains(script, `binding find where status=bound];`) {
		t.Fatal("per-tunnel unfiltered queue/binding reads")
	}
	if !strings.Contains(script, `target=($ppContextIP."/32") or target=$ppContextName`) || !strings.Contains(script, `dynamic=yes and server=$sn`) {
		t.Fatal("missing tuple-scoped plan/PD projection")
	}
}

func TestL06DLogIsExactFilteredProjectedAndBounded(t *testing.T) {
	pattern := routerssh.PPLogPattern("ana")
	re, e := regexp.Compile(pattern)
	if e != nil {
		t.Fatal(e)
	}
	for _, message := range []string{"ana logged in", "<pppoe-ana>: terminating", "ana authentication failed", "user 'ana' authentication failed"} {
		if !re.MatchString(message) {
			t.Fatal("matching login omitted", message)
		}
	}
	for _, message := range []string{"mariana logged in", "<pppoe-mariana>: terminating", "anabel authentication failed", "user 'ana,foo' authentication failed", "user 'ana:foo' logged in", "user 'ana foo' logged in"} {
		if re.MatchString(message) {
			t.Fatal("different login matched", message)
		}
	}
	special, _ := regexp.Compile(routerssh.PPLogPattern(`a.$[x]`))
	if !special.MatchString(`a.$[x] logged in`) || special.MatchString(`abZx logged in`) {
		t.Fatal("login regex was not literal")
	}
	command, ok := routerssh.BuildPPLogCommand(`a.$[x]`)
	if !ok {
		t.Fatal("safe login rejected")
	}
	if strings.Contains(command, `:put $message`) || !strings.Contains(command, `topics~"ppp"`) || !strings.Contains(command, `message~$pattern`) {
		t.Fatal("log projection/filter missing")
	}
	for _, version := range []string{"6.49.18", "7.14.3"} {
		body := "L\t1\t" + version + "\t2d1h1s\t2\nE\t2026-10-27 01:00:00\tauth_failed\nE\t01:00:01\tdisconnected\nZ\t2\tcomplete\t2d1h1s\n"
		rows, v, c := routerssh.ParsePPLog(body)
		if c != equipment.OK || v != version || len(rows) != 2 || rows[0].Event != "auth_failed" || rows[1].At != "01:00:01" {
			t.Fatal("log projection", c, rows)
		}
		tainted := strings.ReplaceAll(strings.ReplaceAll(body, "2026-10-27 01:00:00", "canary-secret"), "auth_failed", "canary-password")
		rows, _, c = routerssh.ParsePPLog(tainted)
		if c != equipment.OK {
			t.Fatal(c)
		}
		raw, _ := json.Marshal(rows)
		if strings.Contains(string(raw), "canary-") || rows[0].Event != "other" || rows[0].At != "" {
			t.Fatal("unknown log text escaped allowlist", string(raw))
		}
		if _, _, c = routerssh.ParsePPLog(strings.Split(body, "Z\t")[0]); c == equipment.OK {
			t.Fatal("truncated log accepted")
		}
	}
}

func TestL06DLogLimitAndAvailabilityAreNotEmptySuccess(t *testing.T) {
	header := "L\t1\t7.14.3\t2d1h1s\t51\n"
	body := header + strings.Repeat("E\t01:00:00\tother\n", 51) + "Z\t51\tcomplete\t2d1h1s\n"
	if rows, _, c := routerssh.ParsePPLog(body); c == equipment.OK || len(rows) != 0 {
		t.Fatal("over50 log rows accepted")
	}
	if rows, _, c := routerssh.ParsePPLog("U\tlog\n"); c == equipment.OK || len(rows) != 0 {
		t.Fatal("unavailable log became empty success")
	}
	if rows, _, c := routerssh.ParsePPLog("L\t1\t7.14.3\t2d1h1s\t0\nZ\t0\tcomplete\t2d1h1s\n"); c != equipment.OK || len(rows) != 0 {
		t.Fatal("genuine empty projection rejected")
	}
}

func TestL06DLogCategoryNeverReadsIdentityWords(t *testing.T) {
	command, ok := routerssh.BuildPPLogCommand("authentication failed")
	if !ok {
		t.Fatal("ordinary literal username rejected")
	}
	if strings.Contains(command, `:find $message `) || !strings.Contains(command, `:find $body `) {
		t.Fatal("username can contaminate event classification")
	}
	pattern, e := regexp.Compile(routerssh.PPLogPattern("authentication failed"))
	if e != nil || !pattern.MatchString("authentication failed logged in") {
		t.Fatal("legitimate identity phrase lost")
	}
}

func TestL06DPoolAndStaticCountersKeepUnknownDistinctFromZero(t *testing.T) {
	size := routerssh.PPPoolSize("198.51.100.1-198.51.100.3,198.51.100.3-198.51.100.4")
	if size == nil || *size != 4 {
		t.Fatal("pool size double-counted or guessed", size)
	}
	for _, bad := range []string{"", "?", "invalid", "2001:db8::/64", "198.51.100.4-198.51.100.1"} {
		if routerssh.PPPoolSize(bad) != nil {
			t.Fatal("unknown pool became known size", bad)
		}
	}
	for _, version := range []string{"6.49.18", "7.14.3"} {
		body := "N\t1\t" + version + "\t2d1h1s\ttrue\ttrue\n0\nP\t*P\tpool\t198.51.100.1-198.51.100.3\t0\nI\t*1\tether1\tether\t2026-10-27 00:00:00\t100\t200\t0\t?\t1\t?\ttrue\tfalse\nZ\t2\tcomplete\t2d1h1s\n"
		snap, c := routerssh.ParsePPHealth(body)
		if c != equipment.OK || !snap.PoolsAvailable || !snap.UplinksAvailable || len(snap.Pools) != 1 || len(snap.Interfaces) != 1 {
			t.Fatal("health projection", c)
		}
		if snap.Pools[0].Used == nil || *snap.Pools[0].Used != 0 || snap.Pools[0].Size == nil || *snap.Pools[0].Size != 3 {
			t.Fatal("zero pool read lost", snap.Pools)
		}
		row := snap.Interfaces[0].Row
		if row.RXErrors == nil || *row.RXErrors != "0" || row.TXErrors != nil {
			t.Fatal("unknown interface error became zero", row)
		}
		raw, _ := json.Marshal(snap.Pools)
		if strings.Contains(string(raw), "198.51.") {
			t.Fatal("pool ranges escaped projection")
		}
		unknown := strings.Replace(body, "2026-10-27 00:00:00", "?", 1)
		u, c := routerssh.ParsePPHealth(unknown)
		if c != equipment.OK || u.Interfaces[0].Sample.ID != "" {
			t.Fatal("missing link identity can sustain rate", c)
		}
	}
	unavailable, c := routerssh.ParsePPHealth("N\t1\t7.14.3\t2d1h1s\tfalse\tfalse\nZ\t0\tcomplete\t2d1h1s\n")
	if c != equipment.OK || unavailable.PoolsAvailable || unavailable.UplinksAvailable {
		t.Fatal("unsupported health groups became known empty")
	}
}

func TestL06DHealthUnknownLinkNeverProducesIdentity(t *testing.T) {
	for _, fields := range []string{"ether\t", "ether\t?", "ether\tunknown", "ether\t00:00:00", "\t2026-10-27 00:00:00"} {
		body := "N\t1\t7.14.3\t2d1h1s\tfalse\ttrue\nI\t*1\tether1\t" + fields + "\t100\t200\t0\t?\t1\t?\ttrue\tfalse\nZ\t1\tcomplete\t2d1h1s\n"
		snapshot, c := routerssh.ParsePPHealth(body)
		if c != equipment.OK || snapshot.Interfaces[0].Sample.ID != "" {
			t.Fatal("unknown link marker became identity", fields, c)
		}
		if snapshot.Interfaces[0].Sample.RX == nil || snapshot.Interfaces[0].Row.RXErrors == nil {
			t.Fatal("known independent counters were lost")
		}
	}
}

func TestL06DHealthRechecksLinkAfterCounters(t *testing.T) {
	for _, script := range []string{routerssh.PPHealthFirstCommand, routerssh.PPHealthCommand} {
		read := strings.Index(script, `:set ppHealthTX [/interface get $i tx-byte]`)
		again := strings.Index(script, `:set afterLink [/interface get $i last-link-up-time]`)
		guard := strings.Index(script, `:if (($afterLink!=$ppHealthLink)||($afterName!=$ppHealthName)||($afterKind!=$ppHealthKind)) do={:set ppHealthRX "?"; :set ppHealthTX "?"; :set ppHealthLink "?"}`)
		emit := strings.Index(script, `:local line ("I\t"`)
		if read < 0 || again <= read || guard <= again || emit <= guard {
			t.Fatal("link change during counters can preserve old identity and new counters")
		}
		if strings.Contains(script, "/ip pool used print detail") || strings.Contains(script, "/ip pool used get") || strings.Contains(script, "/interface print") {
			t.Fatal("raw output request")
		}
	}
}

// Independent exact-rational oracle stays outside the domain. Counter delta
// above JS integer precision and maximum uint64 must still be enclosed.
func TestL06DRateEnclosesExactRational(t *testing.T) {
	value := func(n uint64) *uint64 { return &n }
	for _, delta := range []uint64{1, (1 << 53) - 1, 1 << 53, (1 << 53) + 1, ^uint64(0)} {
		a := equipment.PPTunnel{ID: "id", Name: "if", RX: value(0), TX: value(0), Uptime: equipment.PPInterval{Known: true, Min: int64(time.Hour), Max: int64(time.Hour + time.Second)}, RouterUptime: equipment.PPInterval{Known: true, Min: int64(24 * time.Hour), Max: int64(24*time.Hour + time.Second)}, Sample: equipment.PPInterval{Known: true, Min: 1e9, Max: 1100000001}}
		b := a
		b.RX = value(delta)
		b.TX = value(delta)
		b.Sample = equipment.PPInterval{Known: true, Min: 3100000001, Max: 3300000003}
		b.Uptime.Min += 2e9
		b.Uptime.Max += 2e9
		b.RouterUptime.Min += 2e9
		b.RouterUptime.Max += 2e9
		d, u := equipment.PPBandwidth(a, b)
		if d == nil || u == nil {
			t.Fatal("valid pair rejected", delta)
		}
		numerator := new(big.Int).SetUint64(delta)
		numerator.Mul(numerator, big.NewInt(8000000000))
		exactLow := new(big.Rat).SetFrac(numerator, big.NewInt(b.Sample.Max-a.Sample.Min))
		exactHigh := new(big.Rat).SetFrac(numerator, big.NewInt(b.Sample.Min-a.Sample.Max))
		if new(big.Rat).SetFloat64(d.Min).Cmp(exactLow) > 0 || new(big.Rat).SetFloat64(d.Max).Cmp(exactHigh) < 0 {
			t.Fatal("interval narrowed at float boundary", delta, d)
		}
	}
}

// RouterOS reserves menu property names as variables. The literal program must
// keep its filter operands distinct, without rewriting an authorized identity.
func TestL06DCommandsAvoidPropertyVariableCollisions(t *testing.T) {
	login := `literal $name $user $profile`
	band, ok := routerssh.BuildPPBandCommand([]equipment.PPRow{{Login: login}})
	if !ok {
		t.Fatal("literal selection rejected")
	}
	if !strings.Contains(band, `user="literal \$name \$user \$profile"`) {
		t.Fatal("authorized login literal rewritten")
	}
	for _, script := range []string{band, routerssh.PPContextCommand, routerssh.PPHealthFirstCommand, routerssh.PPHealthCommand} {
		for _, reserved := range []string{"name", "user", "profile", "ranges", "used", "running", "disabled"} {
			if strings.Contains(script, ":local "+reserved+" ") {
				t.Fatal("RouterOS property/local collision", reserved)
			}
		}
	}
	if !strings.Contains(band, `where name=$ppBandName and type="pppoe-in"`) {
		t.Fatal("counter selection no longer bound to monitored name")
	}
}

func TestL06DHealthUsesDocumentedEmptyArraySyntax(t *testing.T) {
	for _, script := range []string{routerssh.PPHealthFirstCommand, routerssh.PPHealthCommand} {
		if strings.Contains(script, `:local pools {};`) || strings.Contains(script, `:local ifs {};`) {
			t.Fatal("RouterOS rejects empty array literal before health header")
		}
		if !strings.Contains(script, `:local pools [:toarray ""];`) || !strings.Contains(script, `:local ifs [:toarray ""];`) {
			t.Fatal("health empty-array initialization not documented form")
		}
	}
}

// Native v6 positive control through InterfacesCommand sees ten physical ports;
// dynamic=no in Health saw zero because an unset flag is not explicit false.
func TestL06DStaticFilterAndUnsetFlagsMatchValidatedInterfacePath(t *testing.T) {
	selection := `[/interface find where dynamic!=yes]`
	if !strings.Contains(routerssh.InterfacesCommand, selection) {
		t.Fatal("validated interface selection changed")
	}
	for _, script := range []string{routerssh.PPHealthFirstCommand, routerssh.PPHealthCommand} {
		if !strings.Contains(script, selection) || strings.Contains(script, `where dynamic=no`) {
			t.Fatal("health excludes interfaces with unset dynamic flag")
		}
		if !strings.Contains(script, `[$ppflag [/interface get $i running]]`) || !strings.Contains(script, `[$ppflag [/interface get $i disabled]]`) {
			t.Fatal("supported unset flags become unavailable instead of false")
		}
	}
}
