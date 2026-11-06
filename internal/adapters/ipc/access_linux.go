package ipc

import (
	"context"
	"crypto/subtle"
	"github.com/google/uuid"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/foundation"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

func ListenAccess(dir string, uid uint32, svc *access.Service, identity foundation.Status, closeState func() error) (*Server, error) {
	s, e := Listen(dir, uid, nil, identity)
	if e != nil {
		return nil, e
	}
	s.account = svc
	svc.SetResetProof(resetFileProof{dir: dir, uid: uid})
	s.closeState = closeState
	return s, nil
}
func (s *Server) dispatchAccess(ctx context.Context, uid uint32, q Request, owner string) Response {
	r := Response{Version: 1, RequestID: q.RequestID, Code: foundation.Invalid}
	if isRadio(q.Op) {
		return s.dispatchRadio(ctx, uid, q, owner)
	}
	if isPPPoE(q.Op) {
		return s.dispatchPPPoE(ctx, uid, q, owner)
	}
	if isEquipment(q.Op) {
		return s.dispatchEquipment(ctx, uid, q, owner)
	}
	if !isAccessOperation(q.Op) {
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
	v, c := s.account.Handle(ctx, uid, accessCommand(q, owner, brand))
	r.Code = foundation.Code(c)
	if c != access.OK {
		return r
	}
	d := accessData(v, s.identity.Instance, int64(time.Since(s.started)/time.Second))
	if s.equipment != nil {
		d.VaultAvailable = s.equipment.VaultAvailable()
	}
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
		return c.bootstrap(ctx, q)
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
	if preAuthOperation(q.Op) {
		q.Session, q.Profile, q.Activation = "", "", ""
	}
	r := c.exchange(ctx, q)
	if r.Code != foundation.OK {
		return r
	}
	return c.finishCall(ctx, q, r)
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
