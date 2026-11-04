package radio

import "testing"

func TestRadioMetricDiagnosticDoesNotFailLiveTick(t *testing.T) {
	failure := &RadioFailure{Reason: "tick-metric:tx-rate", Bytes: 100}
	ap := &RadioReading{CPU: []int{40}, CommandSeconds: .1, Failure: failure}
	run := &persistentRadioRun{apOnly: true, j: &runningJob{RadioInput: RadioInput{Mode: RadioLive}}, cost: RadioCost{PingLimit: 10}}
	tick := &persistentRadioTick{ap: ap, apCode: OK, cpeCode: OK, probeCode: OK,
		load: RadioLoad{PingSent: 1, PingReceived: 1, PingRTT: []float64{1}}}
	tick.entry = run.tickReceipt(tick)
	run.series = append(run.series, tick.entry)
	run.applyTickGates(tick)
	if tick.entry.Reason != "OK" || run.failures != 0 || ap.Failure != failure {
		t.Fatalf("metric diagnostic disabled live: %+v failures=%d", tick.entry, run.failures)
	}
	ap.CPU = nil
	run.applyTickGates(tick)
	if run.failures != 1 {
		t.Fatal("missing CPU no longer refuses tick")
	}
}
