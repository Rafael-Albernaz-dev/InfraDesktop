package routerssh

import (
	"context"
	"infradesktop/internal/adapters/routerssh/deviceparse"
	"infradesktop/internal/modules/equipment"
	"time"
)

func readDeviceOperational(ctx context.Context,
	d equipment.Device,
	user,
	secret string,
	check func() equipment.Code) (*equipment.PPReadResult,
	equipment.Code) {
	started := time.Now()
	connection, c := openPPConnection(ctx, d, user, secret, check, equipment.QueryBudget)
	secret = ""
	if c != equipment.OK {
		return nil, c
	}
	defer connection.close()
	handshake := time.Since(started).Nanoseconds()
	begin := time.Now()
	text, c := connection.run(deviceparse.DeviceOperationalCommand)
	end := time.Now()
	if c != equipment.OK {
		if c == equipment.CollectorDefect {
			return collectorFailure(c, "O:syntax")
		}
		return collectorFailure(c, "O:execution")
	}
	health, rows, c, reason := deviceparse.ParseDeviceOperationalWithReason(text, begin, end)
	if c != equipment.OK {
		return collectorFailure(c, reason)
	}
	return readDeviceAuthAndInsights(connection, health, rows, deviceReadTiming{started, begin, handshake})
}

type deviceReadTiming struct {
	started, begin time.Time
	handshake      int64
}

func readDeviceAuthAndInsights(
	connection *ppConnection, health *equipment.DeviceHealth, rows []equipment.PPReadRow, timing deviceReadTiming,
) (*equipment.PPReadResult, equipment.Code) {
	connection.tighten(32 * 1024)
	text, c := connection.run(deviceparse.DeviceAuthFailuresCommand)
	if c == equipment.OK {
		auth, code, authReason := deviceparse.ParseDeviceAuthFailuresWithReason(text)
		if code == equipment.OK {
			rows = append(rows, auth...)
		} else if code == equipment.Partial || code == equipment.CollectorDefect {
			return collectorFailure(code, authReason)
		} else if code == equipment.Limit {
			rows = append(rows, equipment.PPReadRow{Kind: "availability", Name: "auth", Value: "unavailable"})
		} else {
			return nil, code
		}
	} else if c == equipment.Partial || c == equipment.CollectorDefect {
		if c == equipment.CollectorDefect {
			return collectorFailure(c, "A:syntax")
		}
		return collectorFailure(c, "A:execution")
	} else if c == equipment.Limit || c == equipment.RadioCodeRemotePermissionDenied {
		rows = append(rows, equipment.PPReadRow{Kind: "availability", Name: "auth", Value: "unavailable"})
	} else {
		return nil, c
	}
	return readDeviceInsightGroups(connection, health, rows, timing)
}

func readDeviceInsightGroups(
	connection *ppConnection, health *equipment.DeviceHealth, rows []equipment.PPReadRow, timing deviceReadTiming,
) (*equipment.PPReadResult, equipment.Code) {
	// Human-proved narrow reads share the connection and unchanged20s deadline.
	// Syntax/structural failures are never treated as capability absence.
	for _, group := range []struct {
		name,
		command string
	}{{"fasttrack",
		deviceparse.DeviceFastTrackCommand},
		{"ports",
			deviceparse.DevicePortsCommand}} {
		text, code := connection.run(group.command)
		if code == equipment.RadioCodeRemotePermissionDenied {
			rows = append(rows, equipment.PPReadRow{Kind: "availability", Name: group.name, Value: "unavailable"})
			continue
		}
		if code != equipment.OK {
			return nil, code
		}
		if group.name == "fasttrack" {
			n, code := deviceparse.ParseDeviceFastTrack(text)
			if code != equipment.OK {
				return nil, code
			}
			health.FastTrack = n
		} else {
			ports, code := deviceparse.ParseDevicePorts(text)
			if code != equipment.OK {
				return nil, code
			}
			rows = append(rows, ports...)
		}
		rows = append(rows, equipment.PPReadRow{Kind: "availability", Name: group.name, Value: "complete"})
	}
	return finishDeviceOperationalRead(connection, health, rows, timing)
}

func finishDeviceOperationalRead(
	connection *ppConnection, health *equipment.DeviceHealth, rows []equipment.PPReadRow, timing deviceReadTiming,
) (*equipment.PPReadResult, equipment.Code) {
	started, begin, handshake := timing.started, timing.begin, timing.handshake
	commands, bytes := connection.metrics()
	collector := time.Since(begin).Nanoseconds()
	result := &equipment.PPReadResult{Scope: "operations",
		Started:  equipment.PPTime(started),
		Ended:    equipment.PPTime(time.Now()),
		Version:  health.Version,
		Duration: time.Since(started).Nanoseconds(),
		Rows:     rows,
		Cost: equipment.PPReadCost{Handshake: &handshake,
			Collector: &collector,
			Commands:  commands,
			Bytes:     bytes,
			CPUStart:  health.CPU,
			CPUEnd:    health.CPU}}
	result.Availability = map[string]string{}
	for _, r := range rows {
		if r.Kind == "availability" {
			result.Availability[r.Name] = r.Value
		}
	}
	return ppMeasureRead(result)
}
