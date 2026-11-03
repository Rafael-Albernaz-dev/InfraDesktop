package routerssh

import (
	"context"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/pppoe"
	"log"
	"time"
)

func (Adapter) ReadPP(
	ctx context.Context, d equipment.Device, user, secret, scope string,
	target *equipment.PPReadTarget,
	publish func(*equipment.PPReadResult) equipment.Code,
	check func() equipment.Code,
) (returned *equipment.PPReadResult, code equipment.Code) {
	defer func() {
		returned = normalizePPReadFailure(returned, code)
	}()
	if handled, res, c := dispatchDirectReadScope(ctx, d, user, secret, scope, target, check); handled {
		return res, c
	}
	if !isBandSampledScope(scope) || target == nil {
		return nil, equipment.Invalid
	}
	return runBandSampledRead(ctx, d, user, secret, scope, target, publish, check)
}

func normalizePPReadFailure(
	returned *equipment.PPReadResult, code equipment.Code,
) *equipment.PPReadResult {
	if code != equipment.Partial && code != equipment.CollectorDefect {
		return returned
	}
	reason := "READ:inconsistent"
	if code == equipment.CollectorDefect {
		reason = "READ:syntax"
	}
	if returned != nil && equipment.ValidCollectorReason(code, returned.FailureReason) {
		reason = returned.FailureReason
	}
	log.Printf("routerssh collector failure: code=%s reason=%s", code, reason)
	return &equipment.PPReadResult{FailureReason: reason}
}

func dispatchDirectReadScope(
	ctx context.Context, d equipment.Device, user, secret, scope string,
	target *equipment.PPReadTarget, check func() equipment.Code,
) (bool, *equipment.PPReadResult, equipment.Code) {
	switch scope {
	case "operations":
		res, c := readDeviceOperational(ctx, d, user, secret, check)
		return true, res, c
	case "health":
		res, c := readPPHealth(ctx, d, user, secret, check)
		return true, res, c
	case "log":
		res, c := readPPLog(ctx, d, user, secret, target, check)
		return true, res, c
	case "secret":
		res, c := readPPSecret(ctx, d, user, secret, target, check)
		return true, res, c
	default:
		return false, nil, equipment.OK
	}
}

const ppScopeLive = string(equipment.RadioLive)

func isBandSampledScope(scope string) bool {
	return scope == "bandwidth" || scope == ppScopeLive || scope == "traffic" || scope == "top"
}

type ppBandSampler struct {
	conn            *ppConnection
	command         string
	scope           string
	origin          time.Time
	started         string
	expectedVersion string
	target          *equipment.PPReadTarget
}

func runBandSampledRead(
	ctx context.Context, d equipment.Device, user, secret, scope string,
	target *equipment.PPReadTarget,
	publish func(*equipment.PPReadResult) equipment.Code,
	check func() equipment.Code,
) (*equipment.PPReadResult, equipment.Code) {
	origin := time.Now()
	connection, c := openPPConnection(ctx, d, user, secret, check, pppoe.PPReadBudget(scope))
	secret = ""
	if c != equipment.OK {
		return nil, c
	}
	defer connection.close()
	command, ok := BuildPPBandCommand(target.Rows)
	if !ok {
		return nil, equipment.Invalid
	}
	sampler := &ppBandSampler{
		conn:            connection,
		command:         command,
		scope:           scope,
		origin:          origin,
		started:         equipment.PPTime(origin),
		expectedVersion: target.Observation.Provenance.Version,
		target:          target,
	}
	return sampler.loop(ctx, publish)
}

func (s *ppBandSampler) sample() ([]equipment.PPTunnel, equipment.Code) {
	begin := time.Since(s.origin).Nanoseconds()
	ageStart := time.Since(s.target.Prepared).Nanoseconds()
	text, c := s.conn.run(s.command)
	if c != equipment.OK {
		return nil, c
	}
	rows, version, c := ParsePPBand(text)
	if c != equipment.OK {
		return nil, c
	}
	if s.expectedVersion == "" && s.scope == "top" {
		s.expectedVersion = version
	}
	if version != s.expectedVersion {
		return nil, equipment.ObservationIncompatible
	}
	window := equipment.PPInterval{Known: true, Min: begin, Max: time.Since(s.origin).Nanoseconds()}
	age := equipment.PPInterval{
		Known: true,
		Min:   s.target.Age.Min + ageStart,
		Max:   s.target.Age.Max + time.Since(s.target.Prepared).Nanoseconds(),
	}
	for i := range rows {
		rows[i].Sample = window
	}
	if s.scope == "top" {
		return rows, equipment.OK
	}
	selected := pppoe.MatchPPRead(s.target, rows, age)
	for i := range selected {
		if selected[i].ID == "" {
			selected[i].Sample = window
		}
	}
	return selected, equipment.OK
}

func (s *ppBandSampler) loop(
	ctx context.Context, publish func(*equipment.PPReadResult) equipment.Code,
) (*equipment.PPReadResult, equipment.Code) {
	first, c := s.sample()
	if c != equipment.OK {
		return nil, c
	}
	accumulators := make([]pppoe.PPTrafficAccumulator, len(s.target.Rows))
	var result *equipment.PPReadResult
	interval := time.Second
	if s.scope == ppScopeLive || s.scope == "traffic" {
		interval = 2 * time.Second
	}
	for {
		if done, res, code := s.checkStreamDeadline(ctx, interval, result); done {
			return res, code
		}
		if c := waitSampleInterval(ctx, interval); c != equipment.OK {
			return nil, c
		}
		second, c := s.sample()
		if c != equipment.OK {
			return nil, s.handleSampleFailure(c, first, result, accumulators, publish)
		}
		result, c = s.buildMeasuredSampleResult(first, second, accumulators)
		if c != equipment.OK {
			return nil, c
		}
		if s.scope != ppScopeLive && s.scope != "traffic" {
			return result, equipment.OK
		}
		if c := publish(result); c != equipment.OK {
			return nil, c
		}
		first = second
	}
}

func (s *ppBandSampler) checkStreamDeadline(
	ctx context.Context, interval time.Duration, result *equipment.PPReadResult,
) (bool, *equipment.PPReadResult, equipment.Code) {
	if s.scope != ppScopeLive && s.scope != "traffic" {
		return false, nil, equipment.OK
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return true, nil, equipment.Invalid
	}
	if time.Until(deadline) <= interval+time.Second {
		if result == nil {
			return true, nil, equipment.Partial
		}
		return true, result, equipment.OK
	}
	return false, nil, equipment.OK
}

func waitSampleInterval(ctx context.Context, interval time.Duration) equipment.Code {
	timer := time.NewTimer(interval)
	select {
	case <-ctx.Done():
		timer.Stop()
		return equipment.Cancelled
	case <-timer.C:
		return equipment.OK
	}
}

func (s *ppBandSampler) handleSampleFailure(
	c equipment.Code, first []equipment.PPTunnel, result *equipment.PPReadResult,
	accumulators []pppoe.PPTrafficAccumulator,
	publish func(*equipment.PPReadResult) equipment.Code,
) equipment.Code {
	if s.scope != "traffic" || result == nil {
		return c
	}
	snapshot := *result
	snapshot.Rows = append([]equipment.PPReadRow{}, result.Rows...)
	for i := range snapshot.Rows {
		gap := equipment.PPTunnel{
			Sample: equipment.PPInterval{Known: true, Max: time.Since(s.origin).Nanoseconds()},
		}
		snapshot.Rows[i].Download = nil
		snapshot.Rows[i].Upload = nil
		snapshot.Rows[i].Traffic = accumulators[i].Add(first[i], gap)
	}
	snapshot.Ended = equipment.PPTime(time.Now())
	snapshot.Duration = time.Since(s.origin).Nanoseconds()
	measured, code := ppMeasureRead(&snapshot)
	if code == equipment.OK {
		if code = publish(measured); code != equipment.OK {
			return code
		}
	}
	return c
}

func (s *ppBandSampler) buildMeasuredSampleResult(
	first, second []equipment.PPTunnel, accumulators []pppoe.PPTrafficAccumulator,
) (*equipment.PPReadResult, equipment.Code) {
	commands, bytes := s.conn.metrics()
	result := &equipment.PPReadResult{
		Version:  s.expectedVersion,
		Scope:    s.scope,
		Started:  s.started,
		Ended:    equipment.PPTime(time.Now()),
		Duration: time.Since(s.origin).Nanoseconds(),
		Cost: equipment.PPReadCost{
			Commands: commands, Bytes: bytes, FirstCount: len(first), LastCount: len(second),
		},
	}
	if len(first) > 0 {
		result.Cost.CPUStart = first[0].RouterCPU
	}
	if len(second) > 0 {
		result.Cost.CPUEnd = second[0].RouterCPU
	}
	if s.scope == "top" {
		populateTopConsumptionRows(result, first, second)
	} else {
		s.populateSessionReadRows(result, first, second, accumulators)
	}
	return ppMeasureRead(result)
}

func populateTopConsumptionRows(
	result *equipment.PPReadResult, first, second []equipment.PPTunnel,
) {
	previous := map[string]equipment.PPTunnel{}
	for _, row := range first {
		previous[row.ID] = row
	}
	for _, row := range second {
		download, upload := pppoe.PPBandwidth(previous[row.ID], row)
		result.Rows = append(result.Rows, equipment.PPReadRow{
			Kind: "session", Name: row.Name, Login: row.Login,
			Download: download, Upload: upload, Uptime: row.Uptime,
		})
	}
	pppoe.SortPPConsumption(result.Rows)
}

func (s *ppBandSampler) populateSessionReadRows(
	result *equipment.PPReadResult, first, second []equipment.PPTunnel,
	accumulators []pppoe.PPTrafficAccumulator,
) {
	for i, row := range s.target.Rows {
		download, upload := pppoe.PPBandwidth(first[i], second[i])
		readRow := equipment.PPReadRow{
			Kind: "session", RowID: row.ID, Login: row.Login,
			Download: download, Upload: upload, Uptime: second[i].Uptime,
		}
		if s.scope == "traffic" {
			readRow.Traffic = accumulators[i].Add(first[i], second[i])
		}
		result.Rows = append(result.Rows, readRow)
	}
}
