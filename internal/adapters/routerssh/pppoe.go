package routerssh

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"golang.org/x/crypto/ssh"
	"infradesktop/internal/modules/equipment"
	"math"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const PPPoECommand = `:local esc do={:local v [:tostr $1]; :local out ""; :if ([:len $v]=0) do={:return ""}; :if ([:len $v] > 512) do={:error "field-limit"}; :for n from=0 to=([:len $v]-1) do={:local c [:pick $v $n ($n+1)]; :if ($c="\\") do={:set c "\\\\"} else={:if ($c="\t") do={:set c "\\t"} else={:if ($c="\r") do={:set c "\\r"} else={:if ($c="\n") do={:set c "\\n"}}}}; :set out ($out.$c)}; :return $out}; :local count [/ppp active print count-only where service=pppoe]; :local up [/system resource get uptime]; :put ("H\t1\t".$count."\t".[/system resource get version]."\t".$up); :if ($count>2048) do={:put "Z\t0\trows"} else={:local ids [/ppp active find where service=pppoe]; :local n 0; :local bytes 0; :local reason "complete"; :foreach i in=$ids do={:if (($n<2048)&&($reason="complete")) do={:local sid ""; :do {:set sid [/ppp active get $i session-id]} on-error={}; :local u [/ppp active get $i uptime]; :local line ("R\t".[$esc $i]."\t".[$esc $sid]."\t".[$esc [/ppp active get $i name]]."\t".[$esc [/ppp active get $i address]]."\t".[$esc [/ppp active get $i caller-id]]."\t".$u); :local after ""; :do {:set after [/ppp active get $i session-id]} on-error={}; :if (($sid!=$after)||([/ppp active get $i uptime]<$u)) do={:set reason "churn"}; :if (($bytes+[:len $line]+2)>458752) do={:set reason "wire_bytes"}; :if ($reason="complete") do={:put $line; :set bytes ($bytes+[:len $line]+2); :set n ($n+1)}}}; :if (([:tostr $ids]!=[:tostr [/ppp active find where service=pppoe]])||([/system resource get uptime]<$up)) do={:set reason "churn"}; :put ("Z\t".$n."\t".$reason."\t".[/system resource get uptime])}`

func (Adapter) CollectPPPoE(ctx context.Context, d equipment.Device, user, secret string, check func() equipment.Code) (*equipment.PPObservation, equipment.Code) {
	if c := check(); c != equipment.OK {
		return nil, c
	}
	conn, done, c := dial(ctx, d)
	if c != equipment.OK {
		return nil, c
	}
	defer done()
	cfg := config(user)
	mismatch := false
	hostSeen := false

	cfg.HostKeyCallback = func(_ string, _ net.Addr, key ssh.PublicKey) error {
		hostSeen = true
		if ssh.FingerprintSHA256(key) != d.Fingerprint {
			mismatch = true
			return errors.New("host key mismatch")
		}
		if c := check(); c != equipment.OK {
			return errors.New("authorization changed")
		}
		return nil
	}
	cfg.Auth = []ssh.AuthMethod{ssh.PasswordCallback(func() (string, error) {
		if c := check(); c != equipment.OK {
			return "", errors.New("authorization changed")
		}
		return secret, nil
	})}
	cc, channels, requests, e := ssh.NewClientConn(conn, net.JoinHostPort(d.Address, strconv.Itoa(d.Port)), cfg)
	secret = ""
	if e != nil {
		if mismatch {
			return nil, equipment.TrustChanged
		}
		if timedOut(ctx, e) {
			return nil, "DEADLINE_EXCEEDED"
		}
		// x/crypto wraps authentication rejection as this fixed protocol diagnostic.
		// Callback invocation alone does not prove an authentication failure.
		if strings.Contains(e.Error(), "unable to authenticate, attempted methods") {
			return nil, "REMOTE_AUTH_FAILED"
		}
		if !hostSeen || strings.Contains(e.Error(), "no common algorithm") || strings.Contains(e.Error(), "no common") {
			return nil, "SSH_NEGOTIATION_FAILED"
		}
		return nil, "SSH_CONNECTION_LOST"
	}
	client := ssh.NewClient(cc, channels, requests)
	defer client.Close()
	budget := &outputBudget{conn: conn}
	run := func(command string) (string, equipment.Code) {
		if c := check(); c != equipment.OK {
			return "", c
		}
		session, e := client.NewSession()
		if e != nil {
			var denied *ssh.OpenChannelError
			if errors.As(e, &denied) && denied.Reason == ssh.Prohibited {
				return "", "REMOTE_PERMISSION_DENIED"
			}
			return "", "SSH_CONNECTION_LOST"
		}
		defer session.Close()
		out, stderr := &limitedOutput{budget: budget}, &limitedOutput{budget: budget}
		session.Stdout = out
		session.Stderr = stderr
		e = session.Run(command)
		budget.mu.Lock()
		overflow := budget.overflow
		budget.mu.Unlock()
		if overflow {
			return "", equipment.Limit
		}
		if timedOut(ctx, e) {
			return "", "DEADLINE_EXCEEDED"
		}
		text := out.buf.String()
		if permissionDiagnostic(text) || permissionDiagnostic(stderr.buf.String()) {
			return "", "REMOTE_PERMISSION_DENIED"
		}
		if e != nil || stderr.buf.Len() != 0 {
			return "", equipment.Partial
		}
		return text, equipment.OK
	}
	commandStarted := time.Now()
	text, c := run(PPPoECommand)
	if c != equipment.OK {
		return nil, c
	}
	o, c := ParsePPPoE(text)
	if c == equipment.OK && o != nil && o.Quality.Status == "complete" {
		start, end := o.Provenance.RouterUptime, o.Provenance.RouterFinalUptime
		elapsed := time.Since(commandStarted).Nanoseconds()
		// Reboot cannot be excluded if the initial uptime fits inside this command's
		// entire elapsed window, even when final uptime is greater than initial.
		if !start.Known || !end.Known || start.Min <= elapsed || end.Max < start.Min || end.Min-start.Max > elapsed {
			o.Rows = nil
			o.Quality.Status = "partial"
			o.Quality.Code = equipment.Partial
			o.Quality.Reason = "churn"
			o.Quality.MembershipChecked = false
		}
	}
	return o, c
}

// PPPoEEscape is shared only with the synthetic peer's serializer; the parser is independent.
func PPPoEEscape(s string) string {
	r := strings.NewReplacer("\\", "\\\\", "\t", "\\t", "\r", "\\r", "\n", "\\n")
	return r.Replace(s)
}
func ppUnescape(s string) (string, bool) {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' {
			if s[i] < 32 || s[i] == 127 {
				return "", false
			}
			b.WriteByte(s[i])
			continue
		}
		i++
		if i >= len(s) {
			return "", false
		}
		switch s[i] {
		case '\\':
			b.WriteByte('\\')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case 'n':
			b.WriteByte('\n')
		default:
			return "", false
		}
	}
	v := b.String()
	return v, utf8.ValidString(v)
}

// ParsePPDuration preserves quantization and rejects overflow/unknown formats.
func ParsePPDuration(s string) equipment.PPInterval {
	if s == "" {
		return equipment.PPInterval{}
	}
	if strings.Contains(s, ":") {
		return ppClockDuration(s)
	}
	normalized := s
	total := int64(0)
	quantum := int64(0)
	digits := ""
	for _, c := range normalized {
		if c >= '0' && c <= '9' {
			digits += string(c)
			if len(digits) > 18 {
				return equipment.PPInterval{}
			}
			continue
		}
		factor := int64(0)
		switch c {
		case 'w':
			factor = int64(7 * 24 * time.Hour)
		case 'd':
			factor = int64(24 * time.Hour)
		case 'h':
			factor = int64(time.Hour)
		case 'm':
			factor = int64(time.Minute)
		case 's':
			factor = int64(time.Second)
		default:
			return equipment.PPInterval{}
		}
		v, e := strconv.ParseInt(digits, 10, 64)
		if e != nil || v < 0 || v > (math.MaxInt64-total-factor)/factor {
			return equipment.PPInterval{}
		}
		total += v * factor
		quantum = factor
		digits = ""
	}
	if digits != "" || quantum == 0 || total > math.MaxInt64-quantum {
		return equipment.PPInterval{}
	}
	return equipment.PPInterval{Min: total, Max: total + quantum - 1, Known: true}
}
func ParsePPPoE(text string) (*equipment.PPObservation, equipment.Code) {
	o := &equipment.PPObservation{Scope: equipment.PPScope, Quality: equipment.PPQuality{Status: "partial", Code: equipment.Partial, WireBytes: len(text)}}
	fail := func(reason string) (*equipment.PPObservation, equipment.Code) {
		o.Quality.Returned = len(o.Rows)
		o.Rows = nil
		o.Quality.Status = "partial"
		o.Quality.Code = equipment.Partial
		o.Quality.Reason = reason
		return o, equipment.OK
	}
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 4096), 8192)
	line := 0
	ended := false
	rowBytes := 0
	seen := map[string]bool{}
	for scanner.Scan() {
		raw := strings.TrimSuffix(scanner.Text(), "\r")
		line++
		f := strings.Split(raw, "\t")
		if ended {
			return fail("codec")
		}
		if line == 1 {
			if len(f) != 5 || f[0] != "H" || f[1] != "1" {
				return fail("codec")
			}
			n, e := strconv.Atoi(f[2])
			if e != nil || n < 0 || n > 1000000 {
				return fail("codec")
			}
			o.Quality.CountsKnown = true
			o.Quality.Eligible = n
			o.Provenance.Projection = "active-v1"
			o.Provenance.Codec = "pppoe-fields-v1"
			o.Provenance.Capabilities = "base-fields-v1"
			o.Provenance.Version = f[3]
			if len(f[3]) > 128 || !utf8.ValidString(f[3]) || strings.IndexFunc(f[3], func(r rune) bool { return r < 32 || r == 127 }) >= 0 {
				return fail("codec")
			}
			if strings.HasPrefix(f[3], "6.") {
				o.Provenance.Family = "v6"
			} else if strings.HasPrefix(f[3], "7.") {
				o.Provenance.Family = "v7"
			} else {
				o.Quality.Status = "unsupported"
				o.Quality.Code = "UNSUPPORTED"
				return o, equipment.OK
			}
			o.Provenance.RouterUptime = ParsePPDuration(f[4])
			continue
		}
		if (len(f) == 3 || len(f) == 4) && f[0] == "Z" {
			n, e := strconv.Atoi(f[1])
			if e != nil || n != len(o.Rows) {
				return fail("codec")
			}
			ended = true
			o.Quality.Terminated = true
			o.Quality.Returned = n
			switch f[2] {
			case "complete":
				if len(f) != 4 {
					return fail("codec")
				}
				o.Provenance.RouterFinalUptime = ParsePPDuration(f[3])
				if !o.Provenance.RouterFinalUptime.Known {
					return fail("codec")
				}
				if n != o.Quality.Eligible {
					return fail("churn")
				}
				o.Quality.Status = "complete"
				o.Quality.Code = equipment.OK
				o.Quality.MembershipChecked = true
			case "rows", "wire_bytes":
				o.Quality.Status = "limited"
				o.Quality.Code = equipment.Limit
				o.Quality.Reason = f[2]
				o.Rows = nil
			case "churn":
				return fail("churn")
			default:
				return fail("codec")
			}
			continue
		}
		if len(f) != 7 || f[0] != "R" || len(o.Rows) >= equipment.PPMaxRows || o.Quality.Eligible > equipment.PPMaxRows {
			return fail("rows")
		}
		limits := []int{32, 64, 256, 45, 128, 64}
		for i := 1; i < len(f); i++ {
			v, ok := ppUnescape(f[i])
			if !ok {
				return fail("codec")
			}
			if len(v) > limits[i-1] {
				return fail("row_bytes")
			}
			f[i] = v
		}
		r := equipment.PPRow{RemoteID: f[1], SessionID: f[2], Login: f[3], Address: f[4], Caller: f[5], Uptime: ParsePPDuration(f[6])}
		if r.RemoteID == "" || seen[r.RemoteID] {
			return fail("churn")
		}
		seen[r.RemoteID] = true
		if r.Address != "" {
			ip, e := netip.ParseAddr(r.Address)
			if e != nil || ip.Zone() != "" || ip.IsUnspecified() || ip.IsMulticast() {
				return fail("codec")
			}
			r.Address = ip.String()
		}
		b, _ := json.Marshal(r)
		if len(b)+200 > equipment.PPRowBytes {
			return fail("row_bytes")
		}
		rowBytes += len(raw) + 2
		if rowBytes > equipment.PPWireBytes {
			return fail("wire_bytes")
		}
		o.Rows = append(o.Rows, r)
	}
	if scanner.Err() != nil || !ended {
		return fail("codec")
	}
	if o.Quality.Status == "complete" {
		o.Count = len(o.Rows)
	}
	return o, equipment.OK
}

func ppClockDuration(s string) equipment.PPInterval {
	total := int64(0)
	for _, unit := range []struct {
		suffix string
		factor int64
	}{{"w", int64(7 * 24 * time.Hour)}, {"d", int64(24 * time.Hour)}} {
		if at := strings.Index(s, unit.suffix); at >= 0 {
			v, e := strconv.ParseInt(s[:at], 10, 64)
			if e != nil || v < 0 || v > (math.MaxInt64-total)/unit.factor {
				return equipment.PPInterval{}
			}
			total += v * unit.factor
			s = s[at+1:]
		}
	}
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return equipment.PPInterval{}
	}
	h, e := strconv.ParseInt(parts[0], 10, 64)
	if e != nil || h < 0 || h > (math.MaxInt64-total)/int64(time.Hour) {
		return equipment.PPInterval{}
	}
	total += h * int64(time.Hour)
	m, e := strconv.ParseInt(parts[1], 10, 64)
	if e != nil || m < 0 || m > 59 {
		return equipment.PPInterval{}
	}
	seconds := strings.Split(parts[2], ".")
	if len(seconds) > 2 {
		return equipment.PPInterval{}
	}
	sec, e := strconv.ParseInt(seconds[0], 10, 64)
	if e != nil || sec < 0 || sec > 59 {
		return equipment.PPInterval{}
	}
	rest := m*int64(time.Minute) + sec*int64(time.Second)
	quantum := int64(time.Second)
	if len(seconds) == 2 {
		f := seconds[1]
		if len(f) == 0 || len(f) > 9 {
			return equipment.PPInterval{}
		}
		v, e := strconv.ParseInt(f, 10, 64)
		if e != nil || v < 0 {
			return equipment.PPInterval{}
		}
		for i := len(f); i < 9; i++ {
			v *= 10
			quantum /= 10
		}
		if len(f) == 9 {
			quantum = 1
		} else {
			quantum = 1
			for i := len(f); i < 9; i++ {
				quantum *= 10
			}
		}
		rest += v
	}
	if total > math.MaxInt64-rest-quantum {
		return equipment.PPInterval{}
	}
	total += rest
	return equipment.PPInterval{Min: total, Max: total + quantum - 1, Known: true}
}
