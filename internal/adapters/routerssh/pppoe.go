package routerssh

import (
	"context"
	"errors"
	"golang.org/x/crypto/ssh"
	"infradesktop/internal/modules/equipment"
	"log"
	"net"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const ppEscapeScript = `:local esc do={:local v [:tostr $1]; :local out ""; :if ([:len $v]=0) do={:return ""}; :if ([:len $v] > 512) do={:error "field-limit"}; :for n from=0 to=([:len $v]-1) do={:local c [:pick $v $n ($n+1)]; :if ($c="\\") do={:set c "\\\\"} else={:if ($c="\t") do={:set c "\\t"} else={:if ($c="\r") do={:set c "\\r"} else={:if ($c="\n") do={:set c "\\n"}}}}; :set out ($out.$c)}; :return $out}`

const PPPoECommand = ppEscapeScript + `; :local count [/ppp active print count-only where service=pppoe]; :local up [/system resource get uptime]; :put ("H\t1\t".$count."\t".[/system resource get version]."\t".$up); :if ($count>2048) do={:put "Z\t0\trows"} else={:local ids [/ppp active find where service=pppoe]; :local n 0; :local bytes 0; :local reason "complete"; :if ([:len $ids]>2048) do={:set reason "rows"}; :foreach i in=$ids do={:if ($reason="complete") do={:local present true; :local sid ""; :local after ""; :local u; :local name; :local address; :local caller; :do {:do {:set sid [/ppp active get $i session-id]} on-error={}; :set u [/ppp active get $i uptime]; :set name [/ppp active get $i name]; :set address [/ppp active get $i address]; :set caller [/ppp active get $i caller-id]; :do {:set after [/ppp active get $i session-id]} on-error={}; :if (($sid!=$after)||([/ppp active get $i uptime]<$u)) do={:set reason "item_changed"}} on-error={:set present false; :foreach remaining in=[/ppp active find where service=pppoe] do={:if ($remaining=$i) do={:set reason "item_error"}}}; :if (($present=true)&&($reason="complete")) do={:local line ("R\t".[$esc $i]."\t".[$esc $sid]."\t".[$esc $name]."\t".[$esc $address]."\t".[$esc $caller]."\t".$u); :if (($bytes+[:len $line]+2)>458752) do={:set reason "wire_bytes"}; :if ($reason="complete") do={:put $line; :set bytes ($bytes+[:len $line]+2); :set n ($n+1)}}}}; :if ([/system resource get uptime]<$up) do={:set reason "reboot"}; :put ("Z\t".$n."\t".$reason."\t".[/system resource get uptime])}`

type ppConnection struct {
	legacy  []string
	run     func(string) (string, equipment.Code)
	close   func()
	tighten func(int)
	metrics func() (int, int)
}

func openPPConnection(ctx context.Context, d equipment.Device, user, secret string, check func() equipment.Code, deadlineBudget time.Duration) (*ppConnection, equipment.Code) {
	if c := check(); c != equipment.OK {
		return nil, c
	}
	conn, done, c := dialBudget(ctx, d, deadlineBudget)
	if c != equipment.OK {
		return nil, c
	}
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
		done()
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
	budget := &outputBudget{conn: conn}
	commands := 0
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
		budget.mu.Lock()
		commands++
		budget.mu.Unlock()
		e = session.Run(command)
		if strings.Contains(command, "/tool ping address=") || strings.Contains(command, "/ping address=") {
			exit := -1
			var status *ssh.ExitError
			if e == nil {
				exit = 0
			} else if errors.As(e, &status) {
				exit = status.ExitStatus()
			}
			// Structural diagnostics only; no command, target, reply, credential
			// or remote error text is written to the log.
			log.Printf("routerssh ping execution: exit=%d stdout=%d stderr=%d esc=%d", exit, out.buf.Len(), stderr.buf.Len(), strings.Count(out.buf.String(), "\x1b"))
		}
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
		if collectorSyntaxDiagnostic(text) || collectorSyntaxDiagnostic(stderr.buf.String()) {
			return "", equipment.CollectorDefect
		}
		if permissionDiagnostic(text) || permissionDiagnostic(stderr.buf.String()) {
			return "", "REMOTE_PERMISSION_DENIED"
		}
		if e != nil || stderr.buf.Len() != 0 {
			return "", equipment.Partial
		}
		return text, equipment.OK
	}
	return &ppConnection{legacy: legacyNegotiated(cc), run: run, metrics: func() (int, int) { budget.mu.Lock(); defer budget.mu.Unlock(); return commands, budget.n }, close: func() { _ = client.Close(); done() }, tighten: func(bytes int) {
		budget.mu.Lock()
		defer budget.mu.Unlock()
		next := budget.n + bytes
		if budget.limit == 0 || next < budget.limit {
			budget.limit = next
		}
	}}, equipment.OK
}

func (Adapter) CollectPPPoE(ctx context.Context, d equipment.Device, user, secret string, check func() equipment.Code) (*equipment.PPObservation, equipment.Code) {
	connection, c := openPPConnection(ctx, d, user, secret, check, equipment.QueryBudget)
	if c != equipment.OK {
		return nil, c
	}
	defer connection.close()
	commandStarted := time.Now()
	text, c := connection.run(PPPoECommand)
	if c != equipment.OK {
		return nil, c
	}
	o, c := ParsePPPoE(text)
	if c == equipment.OK && o != nil && o.Quality.Status == "complete" {
		// Reboot cannot be excluded if the initial uptime fits inside this command's
		// entire elapsed window, even when final uptime is greater than initial.
		if !ppCollectionStable(o.Provenance, time.Since(commandStarted)) {
			o.Rows = nil
			o.Quality.Status = "partial"
			o.Quality.Code = equipment.Partial
			o.Quality.Reason = "reboot"
			o.Quality.MembershipChecked = false
		}
	}
	if c == equipment.OK && o != nil && o.Quality.Status == "complete" && len(o.Rows) > 0 {
		// Optional enrichment shares this connection's cumulative SSH budget.
		// Failure never invents a join or discards the complete active reading.
		enriched, code := connection.run(PPContextCommand)
		if c := check(); c != equipment.OK {
			return nil, c
		}
		if ctx.Err() != nil {
			if ctx.Err() == context.DeadlineExceeded {
				return nil, "DEADLINE_EXCEEDED"
			}
			return nil, equipment.Cancelled
		}
		if code == equipment.OK {
			tunnels, version, up, parsed := ParsePPContext(enriched)
			if parsed == equipment.Limit {
				o.ContextOmitted = true
			}
			age := equipment.PPInterval{Known: true, Max: time.Since(commandStarted).Nanoseconds()}
			if parsed == equipment.OK && version == o.Provenance.Version && up.Known && up.Min > age.Max && up.Max >= o.Provenance.RouterUptime.Min {
				equipment.AssociatePPTunnels(o.Rows, tunnels, age)
			}
		} else if code == equipment.Limit {
			o.ContextOmitted = true
		}
	}
	return o, c
}

func ppCollectionStable(provenance equipment.PPProvenance, elapsed time.Duration) bool {
	a, b := provenance.RouterUptime, provenance.RouterFinalUptime
	return a.Known && b.Known && a.Min > elapsed.Nanoseconds() && b.Max >= a.Min && b.Min-a.Max <= elapsed.Nanoseconds()
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
