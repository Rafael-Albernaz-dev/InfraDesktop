package pppoeparse

import (
	"infradesktop/internal/adapters/routerssh/internal/routeros"
	"strings"
)

const PPPoECommand = routeros.EscapeScript + `; :local count [/ppp active print count-only where service=pppoe` +
	`]; :local up [/system resource get uptime]; :put ("H\t1\t".$coun` +
	`t."\t".[/system resource get version]."\t".$up); :if ($count>204` +
	`8) do={:put "Z\t0\trows"} else={:local ids [/ppp active find whe` +
	`re service=pppoe]; :local n 0; :local bytes 0; :local reason "co` +
	`mplete"; :if ([:len $ids]>2048) do={:set reason "rows"}; :foreac` +
	`h i in=$ids do={:if ($reason="complete") do={:local present true` +
	`; :local sid ""; :local after ""; :local u; :local name; :local ` +
	`address; :local caller; :do {:do {:set sid [/ppp active get $i s` +
	`ession-id]} on-error={}; :set u [/ppp active get $i uptime]; :se` +
	`t name [/ppp active get $i name]; :set address [/ppp active get ` +
	`$i address]; :set caller [/ppp active get $i caller-id]; :do {:s` +
	`et after [/ppp active get $i session-id]} on-error={}; :if (($si` +
	`d!=$after)||([/ppp active get $i uptime]<$u)) do={:set reason "i` +
	`tem_changed"}} on-error={:set present false; :foreach remaining ` +
	`in=[/ppp active find where service=pppoe] do={:if ($remaining=$i` +
	`) do={:set reason "item_error"}}}; :if (($present=true)&&($reaso` +
	`n="complete")) do={:local line ("R\t".[$esc $i]."\t".[$esc $sid]` +
	`."\t".[$esc $name]."\t".[$esc $address]."\t".[$esc $caller]."\t"` +
	`.$u); :if (($bytes+[:len $line]+2)>458752) do={:set reason "wire` +
	`_bytes"}; :if ($reason="complete") do={:put $line; :set bytes ($` +
	`bytes+[:len $line]+2); :set n ($n+1)}}}}; :if ([/system resource` +
	` get uptime]<$up) do={:set reason "reboot"}; :put ("Z\t".$n."\t"` +
	`.$reason."\t".[/system resource get uptime])}`

// PPPoEEscape is shared only with the synthetic peer's serializer; the parser is independent.
func PPPoEEscape(s string) string {
	r := strings.NewReplacer("\\", "\\\\", "\t", "\\t", "\r", "\\r", "\n", "\\n")
	return r.Replace(s)
}
