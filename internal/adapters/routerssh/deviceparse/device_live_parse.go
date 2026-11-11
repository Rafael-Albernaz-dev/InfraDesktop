package deviceparse

import (
	"infradesktop/internal/adapters/routerssh/internal/routeros"
	"infradesktop/internal/adapters/routerssh/pppoeparse"
	"infradesktop/internal/modules/pulse"
	"strconv"
	"strings"
	"time"
)

const liveCommandPrefix = `:put ("L\t1\t".[/system resource get uptime]."\t".[/system resource get cpu-load]."\t".`
const LiveConcentratorScript = liveCommandPrefix + `[:len [/ppp active find where service=pppoe]])`
const LiveAPScript = liveCommandPrefix + `[:len [/interface wireless registration-table find]])`

func ParseLiveReading(output string) (pulse.Reading, pulse.ReadFault) {
	fields := strings.Split(strings.TrimRight(output, "\r\n"), "\t")
	if len(fields) != 5 || fields[0] != "L" || fields[1] != "1" {
		return pulse.Reading{}, pulse.ReadLost
	}
	uptime := pppoeparse.ParsePPDuration(fields[2])
	cpu, errCPU := strconv.Atoi(fields[3])
	clients, errClients := strconv.Atoi(fields[4])
	if !uptime.Known || !routeros.Decimal(fields[3]) || !routeros.Decimal(fields[4]) ||
		errCPU != nil || errClients != nil || cpu < 0 || cpu > 100 || clients < 0 {
		return pulse.Reading{}, pulse.ReadLost
	}
	return pulse.Reading{CPU: cpu, Clients: clients, Uptime: time.Duration(uptime.Min)}, pulse.ReadOK
}
