package radio

import "context"

func (s *Service) runRadioIndex(ctx context.Context, j *runningJob, state State, row PPRow) {
	defer s.WG.Done()
	defer j.Cancel()
	defer func() { s.Mu.Lock(); j.Released = true; s.Mu.Unlock() }()
	index := RadioIndex{At: PPTime(s.Now()), Complete: true, Targets: map[string]Code{}, Labels: map[string]string{}}
	remote, ok := s.Remote.(RadioIndexRemote)
	code := Unavailable
	if ok {
		code = s.collectRadioIndexAPs(ctx, j, &state, &index, remote)
	}
	s.finishRadioIndex(ctx, j, state, row, index, code)
}

func (s *Service) collectRadioIndexAPs(ctx context.Context, j *runningJob, state *State,
	index *RadioIndex, remote RadioIndexRemote) Code {
	for _, old := range state.Devices {
		if !old.Active || old.Role != RoleAP {
			continue
		}
		if code := s.collectRadioIndexAP(ctx, j, old, state, index, remote); code != OK {
			return code
		}
	}
	return OK
}

func (s *Service) collectRadioIndexAP(ctx context.Context, j *runningJob, d Device,
	state *State, index *RadioIndex, remote RadioIndexRemote) Code {
	index.Labels[d.ID] = d.Name + " (" + d.Address + ")"
	// This closure must observe d after pinRadioTOFU updates its revision and pin.
	check := func() Code { return s.checkRadioIndexTarget(ctx, j, d) }
	if code := check(); code != OK {
		return code
	}
	var code Code
	d, code = s.pinRadioTOFU(ctx, j.Command, d)
	if code != OK {
		index.Complete = false
		index.Targets[d.ID] = code
		return OK
	}
	for i := range state.Devices {
		if state.Devices[i].ID == d.ID {
			state.Devices[i] = d
		}
	}
	current, code := s.Store.ReadEquipment(ctx, j.Command.Scope.Profile)
	if code != OK {
		return code
	}
	credential := credential(&current, d.CredentialID)
	if credential == nil {
		index.Complete = false
		index.Targets[d.ID] = MissingCredential
		return OK
	}
	entries, code := s.readRadioIndexAuthorized(ctx, j, d, credential.Metadata.Revision, check, remote)
	index.Targets[d.ID] = code
	if code != OK {
		index.Complete = false
		return OK
	}
	if len(index.Entries)+len(entries) > RadioMaxClients*MaxDevices {
		return Limit
	}
	for i := range entries {
		entries[i].AP = d.ID
	}
	index.Entries = append(index.Entries, entries...)
	return OK
}

func (s *Service) checkRadioIndexTarget(ctx context.Context, j *runningJob, d Device) Code {
	return s.authorized(ctx, j.Command, func(uint64) Code {
		if _, code := s.radioRow(ctx, j.Command, j.RadioInput); code != OK {
			return code
		}
		current, code := s.Store.ReadEquipment(ctx, j.Command.Scope.Profile)
		if code != OK {
			return code
		}
		if !sameRadioTarget(device(&current, d.ID), d) {
			return Revision
		}
		return OK
	})
}

func (s *Service) checkRadioIndexCredential(ctx context.Context, j *runningJob, d Device, revision uint64, check func() Code) Code {
	if code := check(); code != OK {
		return code
	}
	current, code := s.Store.ReadEquipment(ctx, j.Command.Scope.Profile)
	if code != OK {
		return code
	}
	credential := credential(&current, d.CredentialID)
	if credential == nil || credential.Metadata.Revision != revision {
		return Revision
	}
	return OK
}

func (s *Service) readRadioIndexAuthorized(ctx context.Context, j *runningJob, d Device, revision uint64,
	check func() Code, remote RadioIndexRemote) ([]RadioIndexEntry, Code) {
	checked := func() Code { return s.checkRadioIndexCredential(ctx, j, d, revision, check) }
	user, secret, code := s.check(ctx, &runningJob{Command: j.Command, Device: d, CredentialRevision: revision}, true)
	if code != OK {
		return nil, code
	}
	targetCtx, cancel := context.WithCancel(ctx)
	target := &runningJob{Command: j.Command, Device: d, CredentialRevision: revision, Cancel: cancel}
	stop := s.watchPP(targetCtx, target)
	entries, code := remote.ReadRadioIndex(targetCtx, d, user, secret, checked)
	secret = ""
	stop()
	cancel()
	return entries, code
}

func (s *Service) finishRadioIndex(ctx context.Context, j *runningJob, state State, row PPRow, index RadioIndex, code Code) {
	if ctx.Err() != nil {
		code = Cancelled
		index.Complete = false
	}
	current, storeCode := s.Store.ReadEquipment(ctx, j.Command.Scope.Profile)
	if storeCode != OK {
		code = storeCode
	}
	index.Context = radioIndexContext(state)
	if index.Context != radioIndexContext(current) {
		index.Complete = false
		code = Revision
	}
	if code == OK {
		code = s.publishRadioIndex(ctx, j.Command, index)
	}
	view := s.radioIndexView(j.Command, &row, current)
	s.completeRadioIndexJob(j, view, code)
}

func (s *Service) publishRadioIndex(ctx context.Context, q Command, index RadioIndex) Code {
	return s.authorized(ctx, q, func(uint64) Code {
		s.Mu.Lock()
		s.radioIndexes = map[string]RadioIndex{}
		s.radioIndexes[radioIndexKey(q)] = index
		s.Mu.Unlock()
		return OK
	})
}

func (s *Service) completeRadioIndexJob(j *runningJob, view RadioView, code Code) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if j.View.State == "CANCELLED" {
		code = Cancelled
	}
	j.Radio = &view
	j.View.State = "SUCCEEDED"
	j.View.Code = code
	if code != OK {
		j.View.State = "FAILED"
	}
	if code == Cancelled {
		j.View.State = "CANCELLED"
	}
	j.Finished = s.Now()
	j.Released = true
}
