package equipment

import (
	"context"
	"infradesktop/internal/modules/access"
	"time"
)

func (s *Service) startRadioIndex(ctx context.Context, q Command, in RadioInput) (*Job, Code) {
	if !access.ValidCompanyID(in.Operation) {
		return nil, Invalid
	}
	row, c := s.radioRow(ctx, q, in)
	if c != OK || row == nil {
		if c == OK {
			c = Invalid
		}
		return nil, c
	}
	st, c := s.store.ReadEquipment(ctx, q.Scope.Profile)
	if c != OK {
		return nil, c
	}
	d := device(&st, in.DeviceID)
	if d == nil {
		return nil, Invalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune()
	if s.closed {
		return nil, Unavailable
	}
	if previous, code := s.existingRadioIndexJobLocked(q, in); code != OK || previous != nil {
		return previous, code
	}
	if len(s.jobs) >= MaxJobs {
		return nil, Limit
	}
	return s.launchRadioIndexLocked(q, in, *d, st, *row)
}
func (s *Service) existingRadioIndexJobLocked(q Command, in RadioInput) (*Job, Code) {
	for _, j := range s.jobs {
		if sameScope(q, j.command) && j.command.Operation == in.Operation {
			if !sameRadioIndexRequest(j, q, in) {
				return nil, Revision
			}
			return copyJob(j), OK
		}
		if !j.released {
			return nil, access.Busy
		}
	}
	return nil, OK
}
func sameRadioIndexRequest(j *runningJob, q Command, in RadioInput) bool {
	return j.command.Op == q.Op && j.radioInput.Mode == RadioModeIndex && j.radioInput.DeviceID == in.DeviceID &&
		j.radioInput.RowID == in.RowID && j.radioInput.ObservationID == in.ObservationID
}
func (s *Service) launchRadioIndexLocked(q Command, in RadioInput, d Device, st State, row PPRow) (*Job, Code) {
	id, c := s.ids.ID()
	if c != OK {
		return nil, c
	}
	jctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	j := &runningJob{command: q,
		device:     d,
		radioInput: in,
		cancel:     cancel,
		view: Job{ID: id,
			DeviceID: d.ID,
			Kind:     "radio.start",
			State:    "RUNNING",
			Code:     OK}}
	j.command.Operation = in.Operation
	for _, target := range st.Devices {
		if target.Active && target.Role == RoleAP {
			j.radioDevices = append(j.radioDevices, target)
		}
	}
	s.jobs[id] = j
	s.wg.Add(1)
	go s.runRadioIndex(jctx, j, st, row)
	return copyJob(j), OK
}
