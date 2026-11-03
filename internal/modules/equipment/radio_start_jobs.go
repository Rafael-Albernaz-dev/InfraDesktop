package equipment

import (
	"context"
	"infradesktop/internal/modules/access"
	"time"
)

func (s *Service) existingRadioJobLocked(q Command, plan radioStartPlan) (*Job, Code) {
	for _, job := range s.jobs {
		if sameScope(q, job.command) && job.command.Operation == plan.input.Operation {
			if !sameRadioStartRequest(q, plan.input, job) {
				return nil, Revision
			}
			return copyJob(job), OK
		}
		if job.released {
			continue
		}
		if plan.input.Mode != RadioOnce && job.command.Op == "radio.start" && job.radioInput.Mode != RadioOnce {
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
	if job.command.Op != q.Op || job.radioInput.DeviceID != in.DeviceID || job.radioInput.Mode != in.Mode {
		return false
	}
	if job.radioInput.RowID != in.RowID || job.radioInput.ObservationID != in.ObservationID {
		return false
	}
	return job.radioInput.Family == in.Family
}

func (s *Service) radioJobCapacityLocked() Code {
	active := 0
	for _, job := range s.jobs {
		if !job.released {
			active++
		}
	}
	if active >= MaxConcurrent {
		return access.Busy
	}
	if len(s.jobs) >= MaxJobs {
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
	id, code := s.ids.ID()
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
	job := &runningJob{command: q, device: plan.target.Device, credentialRevision: plan.target.Credential.Metadata.Revision,
		radioInput: plan.input, radioDevices: plan.devices, cancel: cancel,
		view: Job{ID: id, DeviceID: plan.id, Kind: "radio.start", State: "RUNNING", Code: OK}}
	s.jobs[id] = job
	s.wg.Add(1)
	go s.runRadio(ctx, job, plan.link, plan.revisions, key)
	return copyJob(job), OK
}
