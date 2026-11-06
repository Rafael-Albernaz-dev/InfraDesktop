package ipc

import (
	"infradesktop/internal/modules/access"
	"strings"
)

func isAccessOperation(op string) bool {
	return strings.HasPrefix(op, "access.") || strings.HasPrefix(op, "company.") ||
		op == "preferences.theme" || op == "preferences.columns"
}

func accessCommand(q Request, owner string, brand access.Branding) access.Command {
	return access.Command{Owner: owner, Op: q.Op, Session: q.Session, Grant: q.Grant, Name: q.Name,
		Password: q.Password, Recovery: q.Recovery, ResetNonce: q.ResetNonce, Remember: q.Remember, Profile: q.Profile, Activation: q.Activation,
		Theme: q.Theme, Columns: &q.Columns, Operation: q.Operation, Replace: q.Replace,
		ExpectedProfile: q.ExpectedProfile, ExpectedActivation: q.ExpectedActivation, Brand: brand}
}

func accessData(v access.View, instance string, heartbeat int64) *Data {
	return &Data{ContractRevision: ProductRevision, ProductVersion: ProductVersion, Setup: v.Setup,
		Authenticated: v.Authenticated, Name: v.Name, Theme: v.Theme, Columns: v.Columns, Profile: v.Profile,
		Activation: v.Activation, Session: v.Session, Grant: v.Grant, Recovery: v.Recovery, ResetNonce: v.ResetNonce,
		Instance: instance, Heartbeat: heartbeat}
}
