package ipc

import (
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/foundation"
	"testing"
)

func TestIncorrectPasswordReasonIsLimitedToPasswordOperations(t *testing.T) {
	good := Response{Code: foundation.Unauthenticated, ErrorDetail: &ErrorDetail{Reason: access.WrongPasswordReason}}
	for _, op := range []string{"access.change-password", "access.renew-recovery"} {
		if !validErrorDetailForOperation(op, good) {
			t.Fatal("valid reason rejected", op)
		}
	}
	if validErrorDetailForOperation("access.login", good) {
		t.Fatal("reason accepted on another operation")
	}
	bad := good
	bad.Code = foundation.OK
	if validErrorDetailForOperation("access.change-password", bad) {
		t.Fatal("reason accepted on success")
	}
	bad = good
	bad.ErrorDetail = &ErrorDetail{Reason: access.WrongPasswordReason, Entity: "synthetic"}
	if validErrorDetailForOperation("access.change-password", bad) {
		t.Fatal("foreign entity accepted")
	}
	bad = good
	bad.ErrorDetail = &ErrorDetail{Reason: "OTHER"}
	if validErrorDetailForOperation("access.change-password", bad) {
		t.Fatal("unknown reason accepted")
	}
}
