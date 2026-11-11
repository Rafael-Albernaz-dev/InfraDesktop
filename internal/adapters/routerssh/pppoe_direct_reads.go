package routerssh

import (
	"context"
	"encoding/json"
	"infradesktop/internal/adapters/routerssh/pppoeparse"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/pppoe"
	"log"
	"strings"
	"time"
)

func readPPHealth(ctx context.Context,
	d equipment.Device,
	user,
	secret string,
	check func() equipment.Code) (*equipment.PPReadResult,
	equipment.Code) {
	start := time.Now()
	connection, c := openPPConnection(ctx, d, user, secret, check, equipment.QueryBudget)
	secret = ""
	if c != equipment.OK {
		return nil, c
	}
	defer connection.close()

	first, c := samplePPHealth(connection, start, pppoeparse.PPHealthFirstCommand)
	if c != equipment.OK {
		return nil, c
	}
	timer := time.NewTimer(time.Second)
	select {
	case <-ctx.Done():
		timer.Stop()
		return nil, equipment.Cancelled
	case <-timer.C:
	}
	second, c := samplePPHealth(connection, start, pppoeparse.PPHealthCommand)
	if c != equipment.OK {
		return nil, c
	}
	if first.Version != second.Version {
		return nil, equipment.ObservationIncompatible
	}
	previous := map[string]equipment.PPTunnel{}
	for _, row := range first.Interfaces {
		previous[row.RemoteID] = row.Sample
	}
	rows := append([]equipment.PPReadRow(nil), second.Pools...)
	for _, row := range second.Interfaces {
		row.Row.TX, row.Row.RX = equipment.PPBandwidth(previous[row.RemoteID], row.Sample)
		rows = append(rows, row.Row)
	}
	commands, bytes := connection.metrics()
	return ppMeasureRead(&equipment.PPReadResult{Scope: "health",
		Version:          second.Version,
		Started:          equipment.PPTime(start),
		Ended:            equipment.PPTime(time.Now()),
		Duration:         time.Since(start).Nanoseconds(),
		Rows:             rows,
		PoolsAvailable:   &second.PoolsAvailable,
		UplinksAvailable: &second.UplinksAvailable,
		PoolTotal:        len(second.Pools),
		UplinkTotal:      len(second.Interfaces),
		Cost: equipment.PPReadCost{Commands: commands,
			Bytes:      bytes,
			FirstCount: len(first.Interfaces),
			LastCount:  len(second.Interfaces)}})
}

func readPPLog(ctx context.Context,
	d equipment.Device,
	user,
	secret string,
	target *equipment.PPReadTarget,
	check func() equipment.Code) (*equipment.PPReadResult,
	equipment.Code) {
	if target == nil || len(target.Rows) != 1 {
		return nil, equipment.Invalid
	}
	command, ok := pppoeparse.BuildPPLogCommand(target.Rows[0].Login)
	if !ok {
		return nil, equipment.Invalid
	}
	start := time.Now()
	connection, c := openPPConnection(ctx, d, user, secret, check, equipment.QueryBudget)
	secret = ""
	if c != equipment.OK {
		return nil, c
	}
	defer connection.close()
	connection.tighten(32 * 1024)
	text, c := connection.run(command)
	if c != equipment.OK {
		return nil, c
	}
	rows, version, c := pppoeparse.ParsePPLog(text)
	if c != equipment.OK {
		return nil, c
	}
	if version != target.Observation.Provenance.Version {
		return nil, equipment.ObservationIncompatible
	}
	commands, bytes := connection.metrics()
	return ppMeasureRead(&equipment.PPReadResult{Scope: "log",
		Version:  version,
		Started:  equipment.PPTime(start),
		Ended:    equipment.PPTime(time.Now()),
		Duration: time.Since(start).Nanoseconds(),
		Rows:     rows,
		Cost: equipment.PPReadCost{Commands: commands,
			Bytes: bytes}})
}

func collectPDPing(
	ctx context.Context, connection *ppConnection, target *equipment.PPPingTarget, check func() equipment.Code,
) (*equipment.PDDiagnostic, equipment.Code) {
	command, ok := pppoeparse.BuildPDPingCommand(target)
	if !ok {
		return nil, equipment.ObservationIncompatible
	}
	family := target.Observation.Provenance.Family
	contextCommand := pppoeparse.PDPingContextV7
	if family == equipment.RouterFamilyV6 {
		contextCommand = pppoeparse.PDPingContextV6
	}
	text, c := connection.run(contextCommand)
	if c != equipment.OK {
		return nil, c
	}
	if !pppoeparse.PingContextSafe(family, text) {
		return nil, "PING_CONTEXT_UNAVAILABLE"
	}
	if c = check(); c != equipment.OK {
		return nil, c
	}
	pingContext, cancel := context.WithTimeout(ctx, equipment.PDPingBudget)
	defer cancel()
	stop := context.AfterFunc(pingContext, connection.close)
	defer stop()
	connection.tighten(8 * 1024)
	text, c = connection.run(command)
	if pingContext.Err() != nil {
		return nil, equipment.RadioCodeDeadlineExceeded
	}
	if c != equipment.OK {
		return nil, c
	}
	if strings.HasSuffix(strings.TrimSpace(text), "E\tcontext") {
		return nil, "PING_CONTEXT_UNAVAILABLE"
	}
	if strings.HasSuffix(strings.TrimSpace(text), "E\tsession") {
		return nil, "PING_SESSION_CHANGED"
	}
	result, c := pppoeparse.ParsePDPing(text, 3, target.Row.Address)
	if c != equipment.OK {
		logPDPingParseFailure(text)
		return nil, c
	}
	o := &equipment.PDDiagnostic{Scope: equipment.PDPing,
		Provenance: target.Observation.Provenance,
		Quality: equipment.PPQuality{Status: "complete",
			Code:              equipment.OK,
			CountsKnown:       true,
			Eligible:          1,
			Returned:          1,
			WireBytes:         len(text),
			Terminated:        true,
			MembershipChecked: true},
		Rows: []equipment.PDRow{{Kind: "ping",
			Address: target.Row.Address,
			Ping:    result}}}
	return o, equipment.OK
}

// ppMeasureRead enforces the adapter JSON payload and row byte budgets.
func ppMeasureRead(result *equipment.PPReadResult) (*equipment.PPReadResult, equipment.Code) {
	if result == nil {
		return nil, equipment.Partial
	}
	for _, row := range result.Rows {
		body, e := json.Marshal(row)
		if e != nil {
			return nil, equipment.Partial
		}
		result.MaxRowBytes = max(result.MaxRowBytes, len(body))
	}
	metadata := *result
	metadata.Rows = nil
	body, e := json.Marshal(metadata)
	if e != nil {
		return nil, equipment.Partial
	}
	result.MetadataBytes = len(body)
	body, e = json.Marshal(result)
	if e != nil {
		return nil, equipment.Partial
	}
	result.PayloadBytes = len(body)
	if !pppoe.PPReadWithinBudget(result) {
		return nil, equipment.Limit
	}
	return result, equipment.OK
}

func readPPSecret(ctx context.Context,
	d equipment.Device,
	user,
	secret string,
	target *equipment.PPReadTarget,
	check func() equipment.Code) (*equipment.PPReadResult,
	equipment.Code) {
	if target == nil || len(target.Rows) != 1 {
		return nil, equipment.Invalid
	}
	command, ok := pppoeparse.BuildPPSecretCommand(target.Rows[0].Login)
	if !ok {
		return nil, equipment.Invalid
	}
	start := time.Now()
	connection, c := openPPConnection(ctx, d, user, secret, check, equipment.QueryBudget)
	secret = ""
	if c != equipment.OK {
		return nil, c
	}
	defer connection.close()
	connection.tighten(32 * 1024)
	text, c := connection.run(command)
	if c != equipment.OK {
		return nil, c
	}
	rows, version, c := pppoeparse.ParsePPSecret(text)
	if c != equipment.OK {
		return nil, c
	}
	if version != target.Observation.Provenance.Version {
		return nil, equipment.ObservationIncompatible
	}
	commands, bytes := connection.metrics()
	return ppMeasureRead(&equipment.PPReadResult{Scope: "secret",
		Version:  version,
		Started:  equipment.PPTime(start),
		Ended:    equipment.PPTime(time.Now()),
		Duration: time.Since(start).Nanoseconds(),
		Rows:     rows,
		Cost: equipment.PPReadCost{Commands: commands,
			Bytes: bytes}})
}

func samplePPHealth(connection *ppConnection, start time.Time, command string) (*pppoeparse.PPHealthSnapshot, equipment.Code) {
	begin := time.Since(start).Nanoseconds()
	text, c := connection.run(command)
	if c != equipment.OK {
		return nil, c
	}
	snapshot, c := pppoeparse.ParsePPHealth(text)
	if c != equipment.OK {
		return nil, c
	}
	window := equipment.PPInterval{Known: true, Min: begin, Max: time.Since(start).Nanoseconds()}
	for i := range snapshot.Interfaces {
		snapshot.Interfaces[i].Sample.Sample = window
	}
	return snapshot, equipment.OK

}

func logPDPingParseFailure(text string) {
	log.Printf("routerssh ping parsing: bytes=%d esc=%d lf=%d cr=%d headers=%d s"+
		"ummaries=%d leading_blank=%t",
		len(text),
		strings.Count(text,
			"\x1b"),
		strings.Count(text,
			"\n"),
		strings.Count(text,
			"\r"),
		strings.Count(text,
			"SEQ"),
		strings.Count(text,
			"sent="),
		strings.HasPrefix(text,
			"\r") ||
			strings.HasPrefix(text,
				"\n"))
}
