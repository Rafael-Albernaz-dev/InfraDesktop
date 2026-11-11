package radioparse

import (
	"infradesktop/internal/modules/equipment"
)

// Project only the selected registration's used fields. Escape device data as
// quoted values; no name or interface supplied by the UI becomes RouterOS code.
const radioTickEscape = (":local esc do={:local v [:tostr $1]; :local out \"\"; :if ([:len $v]>256) do={" +
	":error \"field-limit\"}; :if ([:len $v]=0) do={:return \"\"}; :for n from=0 to" +
	"=([:len $v]-1) do={:local c [:pick $v $n ($n+1)]; :if (($c=\"\\\\\")||($c=\"\\" +
	"\"\")) do={:set c (\"\\\\\".$c)}; :if ($c=\"\\n\") do={:set c \"\\\\n\"}; :if " +
	"($c=\"\\r\") do={:set c \"\\\\r\"}; :if ($c=\"\\t\") do={:set c \"\\\\t\"}; :s" +
	"et out ($out.$c)}; :return $out}; ")

func RadioAPTickCommand(mac string) string {
	mac, ok := equipment.RadioMAC(mac)
	if !ok {
		return ""
	}
	return radioTickEscape + (":put \"L07-TICK\"; :put (\"cpu=\" . [/system resource get cpu-load]); :local i" +
		"ds [/interface wireless registration-table find where mac-address=") +
		mac + ("]; :if ([:len $ids]=1) do={:local iface [/interface wireless registration-tabl" +
		"e get $ids interface]; :put (\"bssid=\" . [/interface wireless get [/interface" +
		" wireless find where name=$iface] mac-address])} else={:put \"bssid=unconfirme" +
		"d\"}; :put \"L07-CLIENTS\"; :local rows [/interface wireless registration-tabl" +
		"e print stats as-value where mac-address=") +
		mac + ("]; :if ([:len $rows]=1) do={:foreach r in=$rows do={:put \" 0 row=1\"; :foreac" +
		"h k in={\"interface\";\"mac-address\";\"uptime\";\"last-activity\";\"signal-st" +
		"rength\";\"signal-strength-ch0\";\"signal-strength-ch1\";\"signal-to-noise\";" +
		"\"tx-signal-strength\";\"tx-ccq\";\"rx-ccq\";\"tx-rate\";\"rx-rate\";\"bytes\"" +
		";\"packets\";\"frames\";\"hw-frames\"} do={:local v ($r->$k); :if ([:typeof $v" +
		"]!=\"nil\") do={:put (\"  \".$k.\"=\\\"\".[$esc $v].\"\\\"\")}}}}; :put \"L07-" +
		"TICK-DONE\"")
}

const RadioWOMTickCommand = RadioWOMCommand
