package radio

import (
	"context"
	"infradesktop/internal/modules/access"
	"time"
)

func (s *Service) existingRadioJobLocked(q Command, plan radioStartPlan) (*Job, Code) {
	for _, job := range s.Jobs {
		if sameScope(q, job.Command) && job.Command.Operation == plan.input.Operation {
			if !sameRadioStartRequest(q, plan.input, job) {
				return nil, Revision
			}
			return copyJob(job), OK
		}
		if job.Released {
			continue
		}
		if plan.input.Mode != RadioOnce && job.Command.Op == "radio.start" && job.RadioInput.Mode != RadioOnce {
			return nil, access.Busy
		}
		for _, device := range plan.devices {
			if jobUsesEndpoint(job, device) {
				return nil, access.Busy
			}
		}
	}
	return nil, OK
}

func sameRadioStartRequest(q Command, in RadioInput, job *runningJob) bool {
	if job.Command.Op != q.Op || job.RadioInput.DeviceID != in.DeviceID || job.RadioInput.Mode != in.Mode {
		return false
	}
	if job.RadioInput.RowID != in.RowID || job.RadioInput.ObservationID != in.ObservationID {
		return false
	}
	return job.RadioInput.Family == in.Family
}

func (s *Service) radioJobCapacityLocked() Code {
	active := 0
	for _, job := range s.Jobs {
		if !job.Released {
			active++
		}
	}
	if active >= MaxConcurrent {
		return access.Busy
	}
	if len(s.Jobs) >= MaxJobs {
		return Limit
	}
	return OK
}

func (s *Service) radioStartCostLocked(plan radioStartPlan, key string) Code {
	if plan.input.Mode == RadioLive {
		cost, ok := s.radioCosts[key]
		if !ok || !cost.Enabled || cost.APOnly != plan.apOnly {
			return RadioCodeLiveCostRequired
		}
	}
	if plan.input.Mode == RadioMeasure {
		delete(s.radioCosts, key)
	}
	return OK
}

func (s *Service) launchRadioJobLocked(q Command, plan radioStartPlan, key string) (*Job, Code) {
	id, code := s.IDs.ID()
	if code != OK {
		return nil, code
	}
	budget := 45 * time.Second
	if plan.input.Mode == RadioLive {
		budget = 90 * time.Second
	}
	if plan.input.Mode == RadioMeasure {
		budget = 180 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	job := &runningJob{Command: q, Device: plan.target.Device, CredentialRevision: plan.target.Credential.Metadata.Revision,
		RadioInput: plan.input, RadioDevices: plan.devices, Cancel: cancel,
		View: Job{ID: id, DeviceID: plan.id, Kind: "radio.start", State: "RUNNING", Code: OK}}
	s.Jobs[id] = job
	s.WG.Add(1)
	go s.runRadio(ctx, job, plan.link, plan.revisions, key)
	return copyJob(job), OK
}
