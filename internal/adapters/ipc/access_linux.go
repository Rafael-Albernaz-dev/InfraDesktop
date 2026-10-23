package ipc

import (
	"context"
	"crypto/subtle"
	"github.com/google/uuid"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/foundation"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func ListenAccess(dir string, uid uint32, svc *access.Service, identity foundation.Status, closeState func() error) (*Server, error) {
	s, e := Listen(dir, uid, nil, identity)
	if e != nil {
		return nil, e
	}
	s.account = svc
	s.closeState = closeState
	return s, nil
}
func (s *Server) dispatchAccess(ctx context.Context, uid uint32, q Request, owner string) Response {
	r := Response{Version: 1, RequestID: q.RequestID, Code: foundation.Invalid}
	if !strings.HasPrefix(q.Op, "access.") && !strings.HasPrefix(q.Op, "company.") && q.Op != "preferences.theme" {
		return r
	}
	if subtle.ConstantTimeCompare([]byte(q.Capability), []byte(s.capability)) != 1 {
		r.Code = foundation.Unauthenticated
		return r
	}
	s.account.Reap(ownerAlive)
	var brand access.Branding
	if q.Brand != nil {
		brand = access.Branding{Name: q.Brand.Name, Logo: q.Brand.Logo, Accent: q.Brand.Accent}
	}
	v, c := s.account.Handle(ctx, uid, access.Command{Owner: owner, Op: q.Op, Session: q.Session, Grant: q.Grant, Name: q.Name, Password: q.Password, Recovery: q.Recovery, Remember: q.Remember, Profile: q.Profile, Activation: q.Activation, Theme: q.Theme, Operation: q.Operation, Replace: q.Replace, ExpectedProfile: q.ExpectedProfile, ExpectedActivation: q.ExpectedActivation, Brand: brand})
	r.Code = foundation.Code(c)
	if c != access.OK {
		return r
	}
	d := &Data{ContractRevision: ProductRevision, ProductVersion: "0.4.0", Setup: v.Setup, Authenticated: v.Authenticated, Name: v.Name, Theme: v.Theme, Profile: v.Profile, Activation: v.Activation, Session: v.Session, Grant: v.Grant, Recovery: v.Recovery, Instance: s.identity.Instance, Heartbeat: int64(time.Since(s.started) / time.Second)}
	if v.Brand != nil {
		d.Brand = &Branding{Name: v.Brand.Name, Logo: v.Brand.Logo, Accent: v.Brand.Accent}
	}
	for _, company := range v.Companies {
		d.Companies = append(d.Companies, Company{company.ID, company.Name})
	}
	r.Data = d
	return r
}

// AccountClient serializes client state, never exposes authorization tokens to JS.
type AccountClient struct {
	transport                         *Client
	dir, session, profile, activation string
	pendingCreation, pendingName      string
}

func NewAccountClient(runtime, dir string, uid uint32) *AccountClient {
	return &AccountClient{transport: NewClient(runtime, uid), dir: dir}
}
func (c *AccountClient) exchange(ctx context.Context, q Request) Response {
	cap, e := readCapability(c.transport.dir, c.transport.uid)
	if e != nil {
		return Response{Code: foundation.Unavailable}
	}
	id, e := RandomID()
	if e != nil {
		return Response{Code: foundation.Unavailable}
	}
	q.ContractRevision = ProductRevision
	q.Version = 1
	q.RequestID = id
	q.Capability = cap
	r := c.transport.exchange(ctx, q)
	if r.Code == foundation.OK && (r.Data == nil || r.Data.ContractRevision != ProductRevision) {
		return Response{Code: foundation.Version}
	}
	if q.Op == "access.bootstrap" && r.Code == foundation.Invalid {
		return Response{Code: foundation.Version}
	}
	return r
}
func (c *AccountClient) Call(ctx context.Context, q Request) Response {
	ctx, cancel := context.WithTimeout(ctx, Budget)
	defer cancel()
	if q.Op == "access.bootstrap" {
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
	if q.Op == "company.create" && q.Operation == "" {
		if c.pendingCreation == "" || c.pendingName != q.Name {
			id, e := uuid.NewRandom()
			if e != nil {
				return Response{Code: foundation.Unavailable}
			}
			c.pendingCreation = id.String()
			c.pendingName = q.Name
		}
		q.Operation = c.pendingCreation
	}
	q.Session = c.session
	q.Profile = c.profile
	q.Activation = c.activation
	// Pre-auth fields are operation specific: do not leak a prior session into them.
	if q.Op == "access.login" || q.Op == "access.register" || q.Op == "access.recover" || q.Op == "access.check-issued" || q.Op == "access.ack-issued" {
		q.Session = ""
		q.Profile = ""
		q.Activation = ""
	}
	r := c.exchange(ctx, q)
	if r.Code != foundation.OK {
		return r
	}
	if q.Op == "access.logout" || q.Op == "access.recover" {
		c.session = ""
		c.profile = ""
		c.activation = ""
		if e := c.removeGrant(); e != nil && q.Op != "access.recover" {
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
func (c *AccountClient) accept(r Response) {
	if r.Data == nil {
		return
	}
	if r.Data.Session != "" {
		c.session = r.Data.Session
	}
	if r.Data.Authenticated {
		c.profile = r.Data.Profile
		c.activation = r.Data.Activation
	}
}
func readPrivateToken(dir string, uid uint32) (string, error) {
	p := filepath.Join(dir, "remembered")
	if _, e := os.Lstat(p); os.IsNotExist(e) {
		return "", e
	}
	st, e := privateFile(p, uid, 0)
	if e != nil {
		return "", e
	}
	f, e := os.OpenFile(p, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if e != nil {
		return "", e
	}
	defer f.Close()
	actual, e := f.Stat()
	if e != nil || !os.SameFile(st, actual) || actual.Size() != 64 {
		return "", errUnsafe
	}
	b := make([]byte, 64)
	n, e := f.Read(b)
	if e != nil || n != 64 {
		return "", errUnsafe
	}
	return string(b), nil
}
func (c *AccountClient) saveGrant(token string) error {
	if e := os.MkdirAll(c.dir, 0700); e != nil {
		return e
	}
	if e := safeDir(c.dir, c.transport.uid); e != nil {
		return e
	}
	f, e := os.CreateTemp(c.dir, ".remembered-")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if e = f.Chmod(0600); e == nil {
		_, e = f.WriteString(token)
	}
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e != nil {
		return e
	}
	if ce != nil {
		return ce
	}
	if e = os.Rename(f.Name(), filepath.Join(c.dir, "remembered")); e != nil {
		return e
	}
	return syncDirectory(c.dir)
}
func syncDirectory(dir string) error {
	f, e := os.Open(dir)
	if e != nil {
		return e
	}
	defer f.Close()
	return f.Sync()
}
func (c *AccountClient) removeGrant() error {
	if _, e := os.Lstat(c.dir); os.IsNotExist(e) {
		return nil
	}
	if e := safeDir(c.dir, c.transport.uid); e != nil {
		return e
	}
	e := os.Remove(filepath.Join(c.dir, "remembered"))
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	return syncDirectory(c.dir)
}
