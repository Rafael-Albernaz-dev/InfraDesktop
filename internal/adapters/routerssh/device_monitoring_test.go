package routerssh

import (
	"infradesktop/internal/modules/equipment"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestL06T2OperationalAndAuthFooterUsesCapturedUptime(t *testing.T) {
	// The reboot guard must compare against the start captured by this command,
	// rather than an undefined variable from another projection's prefix.
	start := regexp.MustCompile(`:local ([A-Za-z][A-Za-z0-9]*) \[/system resource get uptime\]; :put \("[OA]`)
	header := regexp.MustCompile(`:put \("[OA]\\t1\\t"\.\[/system resource get version\]\."\\t"\.\$([A-Za-z][A-Za-z0-9]*)\)`)
	footer := regexp.MustCompile(`:if \(\[/system resource get uptime\]<\$([A-Za-z][A-Za-z0-9]*)\)`)
	for name, command := range map[string]string{"operations": DeviceOperationalCommand, "auth": DeviceAuthFailuresCommand} {
		a, b, h := start.FindStringSubmatch(command), footer.FindStringSubmatch(command), header.FindStringSubmatch(command)
		if len(a) != 2 || len(b) != 2 || len(h) != 2 || a[1] != b[1] || a[1] != h[1] {
			t.Fatalf("%s must reuse its own captured uptime: start=%v header=%v footer=%v", name, a, h, b)
		}
	}
}

func opFixture(version string) string {
	return "O\t1\t" + version + "\t2d1h1s\nR\tcpu-load\t=0\nR\tfree-memory\t=100\nR\ttotal-memory\t=200\nR\terror-count\t=0\nR\ttemperature\t?\nC\t=oct/28/2026\t=01:00:00\t=4294956496\t=true\t?\nS\ttelnet\t23\tfalse\nG\tservices\tcomplete\nG\tpackages\tunavailable\nI\tvlan100\tvlan\t0\t?\t2\nG\tinterfaces\tcomplete\nP\tpool\t198.51.100.1-198.51.100.10\t8\nG\tpools\tcomplete\nZ\t3\tcomplete\t2d1h1s\n"
}
func TestL06T2OptionalGroupsAndClockV6V7(t *testing.T) {
	begin := time.Date(2026, 10, 28, 4, 0, 0, 0, time.UTC)
	end := begin.Add(time.Second)
	for _, v := range []string{"6.49.18", "7.14.3"} {
		h, rows, c := ParseDeviceOperational(opFixture(v), begin, end)
		if c != equipment.OK || h.CPU == nil || *h.CPU != 0 || h.Temperature != nil || !h.ClockOffset.Known || h.ClockOffset.Min != -int64(time.Second) || h.ClockOffset.Max != int64(time.Second) || h.NTPStatus != "" {
			t.Fatal("known zero/optional/clock projection", v, h, c)
		}
		found := false
		for _, r := range rows {
			if r.Kind == "interface" {
				found = true
				if r.RXErrors == nil || *r.RXErrors != "0" || r.TXErrors != nil || r.LinkDowns == nil || *r.LinkDowns != "2" {
					t.Fatal("unknown became zero", r)
				}
			}
		}
		if !found {
			t.Fatal("static VLAN counters absent")
		}
		unknown := strings.Replace(opFixture(v), "=4294956496", "?", 1)
		h, _, c = ParseDeviceOperational(unknown, begin, end)
		if c != equipment.OK || h.ClockOffset.Known {
			t.Fatal("unknown clock offset invented", c, h)
		}
		echo := strings.Replace(opFixture(v), "R\terror-count\t=0", "0\nR\terror-count\t=0", 1)
		echo = strings.Replace(echo, "P\tpool", "8\nP\tpool", 1)
		if _, _, c = ParseDeviceOperational(echo, begin, end); c != equipment.OK {
			t.Fatal("v6 count echo rejected", c)
		}
		broken := strings.Replace(echo, "8\nP\tpool", "9\nP\tpool", 1)
		if _, _, c = ParseDeviceOperational(broken, begin, end); c != equipment.Partial {
			t.Fatal("unmatched count echo accepted", c)
		}
	}
}
func TestL06T2NoPartialGroupPrefixOrRawAuthLeak(t *testing.T) {
	now := time.Now()
	text := strings.Replace(opFixture("7.14.3"), "G\tinterfaces\tcomplete", "G\tinterfaces\tunavailable", 1)
	_, rows, c := ParseDeviceOperational(text, now, now)
	if c != equipment.OK {
		t.Fatal(c)
	}
	for _, r := range rows {
		if r.Kind == "interface" {
			t.Fatal("incomplete group prefix exposed")
		}
	}
	for _, v := range []string{"6.49.18", "7.14.3"} {
		text := "A\t1\t" + v + "\t2d1h1s\nF\tsynthetic\t01:00:00\nF\tsynthetic\t01:00:01\nG\tauth\t1\nZ\t2\tcomplete\t2d1h1s\n"
		rows, c := ParseDeviceAuthFailures(text)
		if c != equipment.OK || len(rows) != 2 || rows[0].Used == nil || *rows[0].Used != 2 || rows[1].Value != "partial" {
			t.Fatal("ranking/unknown coverage lost", rows, c)
		}
		unknown := "A\t1\t" + v + "\t2d1h1s\nG\tauth\t3\nZ\t0\tcomplete\t2d1h1s\n"
		rows, c = ParseDeviceAuthFailures(unknown)
		if c != equipment.OK || len(rows) != 1 || rows[0].Value != "partial" {
			t.Fatal("unknown formats became zero", rows, c)
		}
		for _, bad := range []string{strings.Replace(text, "F\tsynthetic\t01:00:00", "F\tuser password=canary\t01:00:00", 1), strings.Replace(text, "Z\t2\tcomplete\t2d1h1s\n", "", 1)} {
			if _, c := ParseDeviceAuthFailures(bad); c == equipment.OK {
				t.Fatal("raw/malformed auth projection accepted")
			}
		}
	}
	for _, forbidden := range []string{"password", "/ppp secret", "address=", "Available From", "/log print detail", "/log print as-value"} {
		if strings.Contains(DeviceOperationalCommand, forbidden) || strings.Contains(DeviceAuthFailuresCommand, forbidden) {
			t.Fatal("forbidden request", forbidden)
		}
	}
	if !strings.Contains(DeviceAuthFailuresCommand, `:put ("F\t"`) && !strings.Contains(DeviceAuthFailuresCommand, `"F\t".`) {
		t.Fatal("router-side projection missing")
	}
}

// Mask string bodies, including quoted code passed to [:parse]. This is a
// narrow denylist of observed v6 parse defects, NOT a RouterOS syntax parser.
func knownV6HealthSyntaxDefect(script string) bool {
	bytes := []byte(script)
	quoted, escaped := false, false
	for i, c := range bytes {
		if quoted {
			bytes[i] = ' '
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
			} else if c == '"' {
				quoted = false
			}
		} else if c == '"' {
			quoted = true
			bytes[i] = ' '
		}
	}
	invalid := regexp.MustCompile(`/system\s+health\s+get\s+(?:\[(?:[^\[\]]|\[[^\[\]]*\])*\]|\$[A-Za-z][A-Za-z0-9]*)\s+[A-Za-z][A-Za-z0-9-]*`)
	return invalid.Match(bytes)
}

func TestL06T2GeneratedCollectorsAvoidKnownV6ParseDefects(t *testing.T) {
	// Include the other generated collectors in the same compatibility audit;
	// item/property reads in interface/queue/etc. are legitimate v6 syntax.
	band, ok := BuildPPBandCommand([]equipment.PPRow{{Login: "synthetic"}})
	if !ok {
		t.Fatal("synthetic band selection invalid")
	}
	log, ok := BuildPPLogCommand("synthetic")
	if !ok {
		t.Fatal("synthetic log selection invalid")
	}
	secret, ok := BuildPPSecretCommand("synthetic")
	if !ok {
		t.Fatal("synthetic secret selection invalid")
	}
	commands := map[string]string{"operations": DeviceOperationalCommand, "auth": DeviceAuthFailuresCommand, "sessions": PPPoECommand, "context": PPContextCommand, "pools": PPHealthCommand, "interfaces": PPHealthFirstCommand, "catalog": PDCatalogCommand, "radius": PDRadiusCommand, "band": band, "log": log, "secret": secret, "resource": ResourceCommand, "inventory-interfaces": InterfacesCommand}
	for name, command := range commands {
		t.Run(name, func(t *testing.T) {
			if knownV6HealthSyntaxDefect(command) {
				t.Fatal("known v6 singleton-health item/property syntax outside deferred string")
			}
		})
	}
	// Independent boundary examples protect against both a trivial always-green
	// check and incorrectly rejecting valid v6 reads in other menus.
	for _, bad := range []string{`[/system health get [:pick $s 0] value]`, `[/system health get $s value]`} {
		if !knownV6HealthSyntaxDefect(bad) {
			t.Fatal("oracle missed known invalid construction", bad)
		}
	}
	for _, valid := range []string{`[/system health get temperature]`, `[/interface get [:pick $ids 0] name]`, `[:parse ":return [/system health get [:pick \$ids 0] value]"]`} {
		if knownV6HealthSyntaxDefect(valid) {
			t.Fatal("oracle rejected legitimate/deferred read", valid)
		}
	}
}

func TestL06T2RealV6OperationalGolden(t *testing.T) {
	data, e := os.ReadFile("testdata/device_monitoring_v6_6_49_18.txt")
	if e != nil {
		t.Fatal(e)
	}
	// Human SSH transcript, parsed offline. This test does not execute RouterOS
	// or establish that the application made the reported collection.
	begin := time.Date(2026, 10, 28, 13, 56, 22, 0, time.UTC)
	h, rows, c := ParseDeviceOperational(string(data), begin, begin.Add(time.Second))
	if c != equipment.OK {
		t.Fatal("human operational envelope rejected", c)
	}
	if h.CPU == nil || *h.CPU != 6 || h.FreeMemory == nil || *h.FreeMemory != 33832960 || h.TotalMemory == nil || *h.TotalMemory != 67108864 || h.Version != "6.49.18 (long-term)" || !h.Uptime.Known {
		t.Fatal("resource values changed", h)
	}
	if h.Temperature != nil {
		t.Fatal("empty temperature became known/zero", h.Temperature)
	}
	if h.NTPEnabled == nil || !*h.NTPEnabled || h.NTPStatus != "" {
		t.Fatal("unknown NTP status invented", h)
	}
	if !h.ClockOffset.Known || h.ClockOffset.Min != -int64(time.Second) || h.ClockOffset.Max != int64(time.Second) {
		t.Fatal("oct date or unsigned GMT-10800s misparsed", h.ClockOffset)
	}
	if h.ErrorCount == nil || *h.ErrorCount != 8 {
		t.Fatal("error-count echo lost", h.ErrorCount)
	}
	counts := map[string]int{}
	active := map[string]bool{}
	pools := map[string]uint64{}
	wireless, link := false, false
	for _, r := range rows {
		counts[r.Kind]++
		switch r.Kind {
		case "service":
			if r.Disabled != nil && !*r.Disabled {
				active[r.Name] = true
			}
		case "package":
			if r.Name == "wireless" {
				wireless = r.Disabled != nil && *r.Disabled && r.Value == "6.49.18"
			}
		case "interface":
			if r.Name == "lab-l2tp" {
				link = r.LinkDowns != nil && *r.LinkDowns == "317"
			}
		case "pool":
			if r.Used == nil {
				t.Fatal("known pool count became unavailable", r.Name)
			}
			pools[r.Name] = *r.Used
		case "availability":
			if r.Value != "complete" {
				t.Fatal("complete group changed", r)
			}
		}
	}
	if len(rows) != 37 || counts["service"] != 3 || counts["package"] != 11 || counts["interface"] != 15 || counts["pool"] != 3 || counts["availability"] != 4 {
		t.Fatal("Z32 complete lost a group or emitted prefix", len(rows), counts)
	}
	if len(active) != 3 || !active["telnet"] || !active["ftp"] || !active["www"] {
		t.Fatal("three exposed-service findings lost", active)
	}
	_, hasPPPoE := pools["lab-pool-01"]
	_, hasBlocked := pools["lab-pool-03"]
	if !wireless || !link || len(pools) != 3 || !hasPPPoE || !hasBlocked || pools["lab-pool-01"] != 0 || pools["lab-pool-02"] != 8 || pools["lab-pool-03"] != 0 {
		t.Fatal("package/interface/pool facts lost", wireless, link, pools)
	}
}

func TestL06T2V7KnownTemperatureAndOptionalStatusPreserved(t *testing.T) {
	body := strings.Replace(opFixture("7.14.3"), "R\ttemperature\t?", "R\ttemperature\t=42.5", 1)
	body = strings.Replace(body, "=true\t?", "=true\t=synchronized", 1)
	h, _, code := ParseDeviceOperational(body, time.Now(), time.Now())
	if code != equipment.OK || h.Temperature == nil || *h.Temperature != 42.5 || h.NTPStatus != "synchronized" {
		t.Fatal("v7 known sensor/status changed", code, h)
	}
}
