package ipc

import (
	"context"
	"crypto/subtle"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/foundation"
)

var radioFields = map[string][]string{
	"radio.index-current":  {"device_id", "observation_id", "row_id", "offset"},
	"radio.current":        {"device_id", "reading_id", "offset"},
	"radio.start":          {"device_id", "revision", "operation_id", "observation_id", "row_id", "family", "mode"},
	"radio.status":         {"job_id", "reading_id", "offset"},
	"radio.cancel":         {"job_id", "device_id", "operation_id"},
	"radio.confirm-family": {"device_id", "revision", "family", "warning", "critical"},
	"radio.link-current":   {"device_id", "observation_id", "row_id"},
	"radio.link-confirm":   {"device_id", "observation_id", "row_id", "operation_id", "link"},
}

func isRadio(op string) bool { _, ok := radioFields[op]; return ok }

func (s *Server) dispatchRadio(ctx context.Context, uid uint32, q Request, owner string) Response {
	r := Response{Version: 1, RequestID: q.RequestID, Code: foundation.Invalid}
	if s.equipment == nil || q.Radio == nil {
		return r
	}
	if subtle.ConstantTimeCompare([]byte(q.Capability), []byte(s.capability)) != 1 {
		r.Code = foundation.Unauthenticated
		return r
	}
	s.account.Reap(ownerAlive)
	scope := access.Command{Session: q.Session,
		Owner:              owner,
		Profile:            q.Profile,
		Activation:         q.Activation,
		ExpectedProfile:    q.ExpectedProfile,
		ExpectedActivation: q.ExpectedActivation}
	v, c := s.equipment.HandleRadio(ctx, equipment.Command{Op: q.Op, UID: uid, Scope: scope}, *q.Radio)
	r.Code = foundation.Code(c)
	if c != equipment.OK {
		return r
	}
	_, code := encodeRadioProjection(v)
	if code != equipment.OK {
		r.Code = foundation.Code(code)
		return r
	}
	r.Data = &Data{ContractRevision: ProductRevision,
		ProductVersion: ProductVersion,
		Instance:       s.identity.Instance,
		Profile:        q.Profile,
		Activation:     q.Activation,
		Authenticated:  true,
		Radio:          &v}
	return r
}
