package deviceparse

import (
	"infradesktop/internal/adapters/routerssh/internal/routeros"
)

// Label data before classifying diagnostics: even an identity equal to an error
// phrase remains data. RouterOS scripting supports concatenation in v6/v7.
const IdentityCommand = `:put ("identity=" . [/system identity get name])`
const ResourceCommand = `:put ("version=" . [/system resource get version]); :put ("board` +
	`=" . [/system resource get board-name]); :put ("uptime=" . [/sys` +
	`tem resource get uptime]); :put ("cpu=" . [/system resource get ` +
	`cpu-load]); :put ("free=" . [/system resource get free-memory]);` +
	` :put ("total=" . [/system resource get total-memory]); :put "do` +
	`ne"`

// Dynamic interfaces (including PPP tunnels) are counted, not enumerated as sessions.
// Static identifiers are captured once; payload and displayed sample are bounded.
const InterfacesCommand = routeros.FlagScript + `:local ids [/interface find where dynamic!=yes]; :put ("eligible` +
	`=" . [:len $ids]); :put ("dynamic=" . [/interface print count-on` +
	`ly where dynamic=yes]); :local n 0; :foreach i in=$ids do={:if (` +
	`$n < 128) do={:put ("name=" . [/interface get $i name]); :put ("` +
	`type=" . [/interface get $i type]); :put ("running=" . [$ppflag ` +
	`[/interface get $i running]]); :put ("disabled=" . [/interface g` +
	`et $i disabled]); :put "end"; :set n ($n + 1)}}; :put "done"`
