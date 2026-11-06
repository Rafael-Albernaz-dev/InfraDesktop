package ipc

import (
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/foundation"
)

func validErrorDetailForOperation(op string, r Response) bool {
	if (op == "access.change-password" || op == "access.renew-recovery") &&
		r.Code == foundation.Unauthenticated && r.ErrorDetail != nil &&
		r.ErrorDetail.Reason == access.WrongPasswordReason && r.ErrorDetail.Entity == "" {
		return true
	}
	return validErrorDetail(r)
}
