package equipment

import (
	"context"
	"strings"
	"testing"
	"time"
)

type radioSequenceCase struct {
	name   string
	cpu    [][]int
	ping   [][]float64
	abort  bool
	reason string
}

func radioSequenceCases() []radioSequenceCase {
	return []radioSequenceCase{
		{"isolated", [][]int{{91}, {50}}, nil, false, ""},
		{"twoCPU", [][]int{{91}, {92}}, nil, true, "CPU"},
		{"twoCPU100", [][]int{{100}, {100}}, nil, true, "CPU"},
		{"twoPingInBatch", [][]int{{50}}, [][]float64{{3, 3, 1}}, true, "Ping"},
		{"mixedSinglePeaks", [][]int{{91}, {50}}, [][]float64{{1}, {3}}, false, ""},
	}
}
func assertPersistentRadioSequence(t *testing.T, tc radioSequenceCase) {
	t.Helper()
	r := &radioLiveRemote{cpuTicks: tc.cpu, pingTicks: tc.ping, holdAfter: len(tc.cpu)}
	s, q, in, store := radioHarness(t, r)
	defer func() {
		if store.saves.Load() != 2 {
			t.Error("ticks overwrote rich inventory", store.saves.Load())
		}
	}()
	v, c := s.HandleRadio(context.Background(), q, in)
	if c != OK {
		t.Fatal(c)
	}
	job := v.Job.ID
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		status := q
		status.Op = "radio.status"
		v, c = s.HandleRadio(context.Background(), status, RadioInput{JobID: job})
		if c != OK {
			t.Fatal(c)
		}
		if tc.abort && v.Job.State != "RUNNING" {
			assertRadioSequenceAbort(t, s, r, v, tc.reason)
			return
		}
		if !tc.abort && v.Cost != nil && len(v.Cost.Series) >= len(tc.cpu) {
			if v.Job.State != "RUNNING" {
				t.Fatal("isolated/mixed peak aborted", v.Job)
			}
			stop := q
			stop.Op = "radio.cancel"
			if _, c = s.HandleRadio(context.Background(), stop, RadioInput{JobID: job}); c != OK {
				t.Fatal(c)
			}
			s.wg.Wait()
			if r.closes.Load() != 2 {
				t.Fatal("stop leaked sessions")
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("expected sequence not observed")
}
func assertRadioSequenceAbort(t *testing.T, s *Service, r *radioLiveRemote, v RadioView, reason string) {
	t.Helper()
	if v.Job.Code != RadioCodeLiveAborted || v.Cost == nil || !strings.Contains(v.Cost.Reason, reason) {
		t.Fatal(v.Job, v.Cost)
	}
	s.wg.Wait()
	if r.closes.Load() != 2 {
		t.Fatal("sessions not closed", r.closes.Load())
	}
}
