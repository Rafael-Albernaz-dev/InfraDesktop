package routerssh

import (
	"context"
	"errors"
	"golang.org/x/crypto/ssh"
	"infradesktop/internal/adapters/routerssh/pppoeparse"
	"infradesktop/internal/modules/equipment"
	"net"
	"strconv"
	"strings"
	"time"
)

type ppConnection struct {
	legacy  []string
	run     func(string) (string, equipment.Code)
	close   func()
	tighten func(int)
	metrics func() (int, int)
}

func openPPConnection(ctx context.Context,
	d equipment.Device,
	user,
	secret string,
	check func() equipment.Code,
	deadlineBudget time.Duration) (*ppConnection,
	equipment.Code) {
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
		return nil, ppHandshakeFailure(ctx, e, mismatch, hostSeen)
	}
	client := ssh.NewClient(cc, channels, requests)
	budget := &outputBudget{conn: conn}
	commands := 0
	runner := &ppCommandRunner{ctx: ctx, client: client, budget: budget, check: check, commands: &commands}
	return &ppConnection{legacy: legacyNegotiated(cc),
		run:     runner.run,
		metrics: runner.metrics,
		close: func() {
			// ignored: cleanup closes the transport after the operation result has been determined
			_ = client.Close()
			done()
		},
		tighten: runner.tighten}, equipment.OK
}

func ppHandshakeFailure(ctx context.Context, e error, mismatch, hostSeen bool) equipment.Code {
	if mismatch {
		return equipment.TrustChanged
	}
	if timedOut(ctx, e) {
		return equipment.RadioCodeDeadlineExceeded
	}
	// x/crypto wraps authentication rejection as this fixed protocol diagnostic.
	// Callback invocation alone does not prove an authentication failure.
	if strings.Contains(e.Error(), "unable to authenticate, attempted methods") {
		return equipment.RadioCodeRemoteAuthFailed
	}
	if !hostSeen || strings.Contains(e.Error(), "no common algorithm") || strings.Contains(e.Error(), "no common") {
		return equipment.RadioCodeSSHNegotiationFailed
	}
	return equipment.RadioCodeSSHConnectionLost
}

func (Adapter) CollectPPPoE(ctx context.Context,
	d equipment.Device,
	user,
	secret string,
	check func() equipment.Code) (*equipment.PPObservation,
	equipment.Code) {
	connection, c := openPPConnection(ctx, d, user, secret, check, equipment.QueryBudget)
	if c != equipment.OK {
		return nil, c
	}
	defer connection.close()
	commandStarted := time.Now()
	text, c := connection.run(pppoeparse.PPPoECommand)
	if c != equipment.OK {
		return nil, c
	}
	o, c := pppoeparse.ParsePPPoE(text)
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
		return enrichPPObservation(ctx, connection, o, commandStarted, check)
	}
	return o, c
}

func enrichPPObservation(
	ctx context.Context, connection *ppConnection, o *equipment.PPObservation,
	commandStarted time.Time, check func() equipment.Code,
) (*equipment.PPObservation, equipment.Code) {
	// Optional enrichment shares this connection's cumulative SSH budget.
	// Failure never invents a join or discards the complete active reading.
	enriched, code := connection.run(pppoeparse.PPContextCommand)
	if c := check(); c != equipment.OK {
		return nil, c
	}
	if ctx.Err() != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, equipment.RadioCodeDeadlineExceeded
		}
		return nil, equipment.Cancelled
	}
	if code == equipment.OK {
		tunnels, version, up, parsed := pppoeparse.ParsePPContext(enriched)
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
	return o, equipment.OK
}

func ppCollectionStable(provenance equipment.PPProvenance, elapsed time.Duration) bool {
	a, b := provenance.RouterUptime, provenance.RouterFinalUptime
	return a.Known && b.Known && a.Min > elapsed.Nanoseconds() && b.Max >= a.Min && b.Min-a.Max <= elapsed.Nanoseconds()
}

func (Adapter) CollectPPDiagnostic(
	ctx context.Context, req equipment.PDDiagnosticRequest,
) (*equipment.PDDiagnostic, equipment.Code) {
	d, user, secret, scope, ping, check := req.Device, req.User, req.Secret, req.Scope, req.Ping, req.Check
	req.Secret = ""
	if !equipment.PDValidScope(scope) {
		return nil, equipment.Invalid
	}
	if scope == equipment.PDPing {
		// ignored: validate the target here; the ping collector builds the command when executing
		if _, ok := pppoeparse.BuildPDPingCommand(ping); !ok {
			return nil, equipment.ObservationIncompatible
		}
	}
	connection, c := openPPConnection(ctx, d, user, secret, check, equipment.QueryBudget)
	if c != equipment.OK {
		return nil, c
	}
	defer connection.close()
	if scope == equipment.PDPing {
		return collectPDPing(ctx, connection, ping, check)
	}
	command := pppoeparse.PDCatalogCommand
	if scope == equipment.PDRadius {
		command = pppoeparse.PDRadiusCommand
	} else if scope != equipment.PDCatalog {
		return nil, equipment.Invalid
	}
	started := time.Now()
	text, c := connection.run(command)
	if c != equipment.OK {
		return nil, c
	}
	o, c := pppoeparse.ParsePPDiagnostic(scope, text)
	if c == equipment.OK && o.Quality.Status == "complete" {
		if !ppCollectionStable(o.Provenance, time.Since(started)) {
			o.Rows = nil
			o.Quality.Status = "partial"
			o.Quality.Code = equipment.Partial
			o.Quality.Reason = "churn"
			o.Quality.MembershipChecked = false
		}
	}
	return o, c
}
