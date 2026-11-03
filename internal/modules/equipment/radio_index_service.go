package equipment

import "context"

func (s *Service) runRadioIndex(ctx context.Context, j *runningJob, state State, row PPRow) {
	defer s.wg.Done()
	defer j.cancel()
	defer func() { s.mu.Lock(); j.released = true; s.mu.Unlock() }()
	index := RadioIndex{At: PPTime(s.now()), Complete: true, Targets: map[string]Code{}, Labels: map[string]string{}}
	remote, ok := s.remote.(RadioIndexRemote)
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
	d, code = s.pinRadioTOFU(ctx, j.command, d)
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
	current, code := s.store.ReadEquipment(ctx, j.command.Scope.Profile)
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
	return s.authorized(ctx, j.command, func(uint64) Code {
		if _, code := s.radioRow(ctx, j.command, j.radioInput); code != OK {
			return code
		}
		current, code := s.store.ReadEquipment(ctx, j.command.Scope.Profile)
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
	current, code := s.store.ReadEquipment(ctx, j.command.Scope.Profile)
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
	user, secret, code := s.check(ctx, &runningJob{command: j.command, device: d, credentialRevision: revision}, true)
	if code != OK {
		return nil, code
	}
	targetCtx, cancel := context.WithCancel(ctx)
	target := &runningJob{command: j.command, device: d, credentialRevision: revision, cancel: cancel}
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
	current, storeCode := s.store.ReadEquipment(ctx, j.command.Scope.Profile)
	if storeCode != OK {
		code = storeCode
	}
	index.Context = radioIndexContext(state)
	if index.Context != radioIndexContext(current) {
		index.Complete = false
		code = Revision
	}
	if code == OK {
		code = s.publishRadioIndex(ctx, j.command, index)
	}
	view := s.radioIndexView(j.command, &row, current)
	s.completeRadioIndexJob(j, view, code)
}

func (s *Service) publishRadioIndex(ctx context.Context, q Command, index RadioIndex) Code {
	return s.authorized(ctx, q, func(uint64) Code {
		s.mu.Lock()
		s.radioIndexes = map[string]RadioIndex{}
		s.radioIndexes[radioIndexKey(q)] = index
		s.mu.Unlock()
		return OK
	})
}

func (s *Service) completeRadioIndexJob(j *runningJob, view RadioView, code Code) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j.view.State == "CANCELLED" {
		code = Cancelled
	}
	j.radio = &view
	j.view.State = "SUCCEEDED"
	j.view.Code = code
	if code != OK {
		j.view.State = "FAILED"
	}
	if code == Cancelled {
		j.view.State = "CANCELLED"
	}
	j.finished = s.now()
	j.released = true
}
