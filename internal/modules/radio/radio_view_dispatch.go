package radio

import "context"

func (s *Service) dispatchRadioView(ctx context.Context, q Command, in RadioInput, store RadioStore, v *RadioView) Code {
	switch q.Op {
	case "radio.status", "radio.cancel":
		return s.handleRadioJob(ctx, q, in, store, v)
	case "radio.current", "radio.confirm-family":
		return s.handleRadioCurrent(ctx, q, in, store, v)
	case "radio.link-current", "radio.index-current", "radio.link-confirm":
		return s.handleRadioLink(ctx, q, in, store, v)
	case "radio.start":
		var c Code
		v.Job, c = s.startRadio(ctx, q, in)
		return c
	}
	return Invalid
}
