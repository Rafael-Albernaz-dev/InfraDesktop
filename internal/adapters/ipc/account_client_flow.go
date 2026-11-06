package ipc

import (
	"context"
	"infradesktop/internal/modules/foundation"
	"os"
)

func (c *AccountClient) bootstrap(ctx context.Context, q Request) Response {
	if c.session != "" {
		r := c.exchange(ctx, Request{Op: "access.snapshot", Session: c.session, Profile: c.profile, Activation: c.activation})
		if r.Code == foundation.OK {
			return r
		}
		if r.Code != foundation.Unauthenticated && r.Code != foundation.ProfileChanged {
			return r
		}
		c.session = ""
	}
	// A token is read only from an owner-only regular file after directory validation.
	if safeDir(c.dir, c.transport.uid) == nil {
		if token, e := readPrivateToken(c.dir, c.transport.uid); e == nil {
			r := c.exchange(ctx, Request{Op: "access.resume", Grant: token})
			if r.Code == foundation.OK {
				c.accept(r)
				return r
			}
			if r.Code != foundation.Unauthenticated {
				return r
			}
		} else if !os.IsNotExist(e) {
			return Response{Code: foundation.Unavailable}
		}
	}
	return c.exchange(ctx, q)
}
func preAuthOperation(op string) bool {
	switch op {
	case "access.login", "access.register", "access.recover", "access.reset-begin", "access.reset-complete",
		"access.check-issued", "access.ack-issued":
		return true
	default:
		return false
	}
}
func (c *AccountClient) finishCall(ctx context.Context, q Request, r Response) Response {
	if q.Op == "access.logout" || q.Op == "access.recover" || q.Op == "access.reset-complete" {
		c.session = ""
		c.profile = ""
		c.activation = ""
		if e := c.removeGrant(); e != nil && q.Op != "access.recover" && q.Op != "access.reset-complete" {
			return Response{Code: foundation.Unavailable}
		}
	}
	if r.Data != nil && r.Data.Grant != "" {
		if e := c.saveGrant(r.Data.Grant); e != nil {
			// Durable token storage failed. Revoke the newly created grant, report failure.
			c.accept(r)
			_ = c.exchange(ctx, Request{Op: "access.logout", Session: c.session, Profile: c.profile, Activation: c.activation})
			c.session = ""
			return Response{Code: foundation.Unavailable}
		}
	}
	if q.Op == "access.login" && !q.Remember {
		if e := c.removeGrant(); e != nil {
			return Response{Code: foundation.Unavailable}
		}
	}
	if q.Op == "company.create" {
		c.pendingCreation = ""
		c.pendingName = ""
	}
	c.accept(r)
	return r
}
