package equipment

import (
	"fmt"
	"math"
	"sort"
)

type radioMeasurementBaselines struct{ cpu, ping []float64 }

func AssessRadioMeasurements(samples []RadioMeasurement) RadioCost {
	cost, bases := collectRadioCost(samples)
	if len(samples) != 10 {
		return cost
	}
	cost.Reason = ""
	retainFirstRadioReason(&cost, failedRadioMeasurementReason(samples, cost.APOnly))
	retainFirstRadioReason(&cost, slowRadioCommandReason(samples))
	cost.CPUBaseline = radioMedianTen(bases.cpu)
	retainFirstRadioReason(&cost, radioCPUStreakReason(samples))
	if len(bases.ping) == 10 {
		cost.PingBaseline = radioMedianTen(bases.ping)
		cost.PingLimit = 2 * cost.PingBaseline
		retainFirstRadioReason(&cost, radioPingStreakReason(samples, cost.PingLimit))
	}
	retainFirstRadioReason(&cost, radioPingLossReason(samples))
	retainFirstRadioReason(&cost, radioCoverageReason(samples))
	if cost.Reason == "" {
		cost.Enabled = true
		cost.Reason = "Custo medido em dez ticks; comandos AP/CPE até 3 s; " +
			"duas CPUs consecutivas acima de 90% recusam o live."
		if cost.APOnly {
			cost.Reason = "Custo medido em dez ticks só do AP; comando até 3 s; " +
				"duas CPUs consecutivas acima de 90% recusam o live."
		}
	}
	return cost
}

func collectRadioCost(samples []RadioMeasurement) (RadioCost, radioMeasurementBaselines) {
	cost := RadioCost{Interval: 1, Reason: "Custo ainda não medido em dez leituras.",
		Measurements: append([]RadioMeasurement{}, samples...)}
	if len(samples) > 0 {
		cost.APOnly = samples[0].APOnly
	}
	var times []float64
	var bases radioMeasurementBaselines
	for _, sample := range samples {
		cost.Samples = append(cost.Samples, sample.Seconds)
		cost.Load = append(cost.Load, sample.Load)
		command := math.Max(sample.AP.CommandSeconds, sample.CPE.CommandSeconds)
		if radioFinite(command) && command > 0 {
			times = append(times, command)
		}
		if len(sample.Load.PingRTT) > 0 && radioFinite(sample.Load.PingRTT[0]) {
			bases.ping = append(bases.ping, sample.Load.PingRTT[0])
		}
		if len(sample.Load.CPU) > 0 && sample.Load.CPU[0] >= 0 && sample.Load.CPU[0] <= 100 {
			bases.cpu = append(bases.cpu, float64(sample.Load.CPU[0]))
		}
	}
	if len(times) > 0 {
		sort.Float64s(times)
		cost.P95 = times[int(math.Ceil(.95*float64(len(times))))-1]
		cost.Interval = math.Max(1, 3*cost.P95)
	}
	return cost, bases
}

func retainFirstRadioReason(cost *RadioCost, reason string) {
	if cost.Reason == "" {
		cost.Reason = reason
	}
}

func radioMedianTen(values []float64) float64 {
	if len(values) != 10 {
		return 0
	}
	sort.Float64s(values)
	return (values[4] + values[5]) / 2
}

func failedRadioMeasurementReason(samples []RadioMeasurement, apOnly bool) string {
	for _, sample := range samples {
		if !sample.Complete || sample.APOnly != apOnly {
			return "Amostra de leitura falhou; live desligado."
		}
	}
	return ""
}

func slowRadioCommandReason(samples []RadioMeasurement) string {
	for _, side := range []struct {
		name string
		get  func(RadioMeasurement) RadioTiming
	}{
		{"do AP", func(s RadioMeasurement) RadioTiming { return s.AP }},
		{"da CPE", func(s RadioMeasurement) RadioTiming { return s.CPE }},
	} {
		for _, sample := range samples {
			timing := side.get(sample)
			if timing.CommandSeconds > 3 {
				return "Comando " + side.name + " levou " + radioSeconds(timing.CommandSeconds) + " s (limite 3 s); live desligado."
			}
		}
	}
	return ""
}

func radioCPUStreakReason(samples []RadioMeasurement) string {
	consecutive := 0
	for _, sample := range samples {
		for _, cpu := range sample.Load.CPU {
			if cpu <= 100 && cpu > RadioMaxCPU {
				consecutive++
			} else {
				consecutive = 0
			}
			if consecutive >= 2 {
				return fmt.Sprintf("CPU do AP chegou a %d%% em duas leituras consecutivas (limite %d%%); live desligado.", cpu, RadioMaxCPU)
			}
		}
	}
	return ""
}

func radioPingStreakReason(samples []RadioMeasurement, limit float64) string {
	consecutive := 0
	for _, sample := range samples {
		for _, rtt := range sample.Load.PingRTT {
			if radioFinite(rtt) && rtt > limit {
				consecutive++
			} else {
				consecutive = 0
			}
			if consecutive >= 2 {
				return "Ping degradou: " + radioSeconds(rtt) + " ms, duas latências consecutivas acima de " +
					radioSeconds(limit) + " ms (2× a mediana); live desligado."
			}
		}
	}
	return ""
}

func radioPingLossReason(samples []RadioMeasurement) string {
	for _, sample := range samples {
		load := sample.Load
		if load.PingSent > 0 && load.PingReceived >= 0 && load.PingReceived < load.PingSent {
			return fmt.Sprintf("Ping perdido: %d de %d respostas; live desligado.", load.PingReceived, load.PingSent)
		}
	}
	return ""
}

func radioCoverageReason(samples []RadioMeasurement) string {
	for _, sample := range samples {
		if !radioMeasurementCovered(sample) {
			return "Carga incompleta: CPU, ping ou janela não cobrem a leitura; live desligado."
		}
	}
	return ""
}

func radioMeasurementCovered(sample RadioMeasurement) bool {
	if !radioFinite(sample.Seconds) || sample.Seconds <= 0 || !radioLoadWindowValid(sample.Load) {
		return false
	}
	if !radioTimingValid(sample.AP) {
		return false
	}
	if !sample.APOnly && !radioTimingValid(sample.CPE) {
		return false
	}
	for _, rtt := range sample.Load.PingRTT {
		if !radioFinite(rtt) {
			return false
		}
	}
	for _, cpu := range sample.Load.CPU {
		if cpu < 0 || cpu > 100 {
			return false
		}
	}
	return true
}

func radioLoadWindowValid(load RadioLoad) bool {
	if load.Started.IsZero() || load.Ended.IsZero() || load.Ended.Before(load.Started) {
		return false
	}
	if len(load.CPU) < 1 || load.PingSent < 1 {
		return false
	}
	return load.PingReceived == load.PingSent && len(load.PingRTT) == load.PingReceived
}

func radioTimingValid(timing RadioTiming) bool {
	return radioFinite(timing.CommandSeconds) && timing.CommandSeconds > 0 &&
		radioFinite(timing.PathSeconds) && timing.PathSeconds >= timing.CommandSeconds
}
