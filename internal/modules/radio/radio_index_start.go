package radio

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
	st, c := s.Store.ReadEquipment(ctx, q.Scope.Profile)
	if c != OK {
		return nil, c
	}
	d := device(&st, in.DeviceID)
	if d == nil {
		return nil, Invalid
	}
	s.Mu.Lock()
	defer s.Mu.Unlock()
	s.prune()
	if s.Closed {
		return nil, Unavailable
	}
	if previous, code := s.existingRadioIndexJobLocked(q, in); code != OK || previous != nil {
		return previous, code
	}
	if len(s.Jobs) >= MaxJobs {
		return nil, Limit
	}
	return s.launchRadioIndexLocked(q, in, *d, st, *row)
}
func (s *Service) existingRadioIndexJobLocked(q Command, in RadioInput) (*Job, Code) {
	for _, j := range s.Jobs {
		if sameScope(q, j.Command) && j.Command.Operation == in.Operation {
			if !sameRadioIndexRequest(j, q, in) {
				return nil, Revision
			}
			return copyJob(j), OK
		}
		if !j.Released {
			return nil, access.Busy
		}
	}
	return nil, OK
}
func sameRadioIndexRequest(j *runningJob, q Command, in RadioInput) bool {
	return j.Command.Op == q.Op && j.RadioInput.Mode == RadioModeIndex && j.RadioInput.DeviceID == in.DeviceID &&
		j.RadioInput.RowID == in.RowID && j.RadioInput.ObservationID == in.ObservationID
}
func (s *Service) launchRadioIndexLocked(q Command, in RadioInput, d Device, st State, row PPRow) (*Job, Code) {
	id, c := s.IDs.ID()
	if c != OK {
		return nil, c
	}
	jctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	j := &runningJob{Command: q,
		Device:     d,
		RadioInput: in,
		Cancel:     cancel,
		View: Job{ID: id,
			DeviceID: d.ID,
			Kind:     "radio.start",
			State:    "RUNNING",
			Code:     OK}}
	j.Command.Operation = in.Operation
	for _, target := range st.Devices {
		if target.Active && target.Role == RoleAP {
			j.RadioDevices = append(j.RadioDevices, target)
		}
	}
	s.Jobs[id] = j
	s.WG.Add(1)
	go s.runRadioIndex(jctx, j, st, row)
	return copyJob(j), OK
}
