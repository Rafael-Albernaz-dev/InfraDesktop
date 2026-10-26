package ipc

import (
	"context"
	"crypto/subtle"
	"errors"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/foundation"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

type Server struct {
	ppOnce                sync.Once
	ppKey                 [32]byte
	ppKeyError            error
	dir, capability       string
	started               time.Time
	uid                   uint32
	service               *foundation.Service
	account               *access.Service
	equipment             *equipment.Service
	closeState            func() error
	listener              *net.UnixListener
	lock                  *os.File
	socketInfo, tokenInfo os.FileInfo
	mu                    sync.Mutex
	connections           map[*net.UnixConn]bool
	wg                    sync.WaitGroup
	slots                 chan struct{}
	once                  sync.Once
	cleanup               sync.Once
	closing               bool
	identity              foundation.Status
}

func Listen(dir string, uid uint32, svc *foundation.Service, identity foundation.Status) (*Server, error) {
	if e := safeDir(dir, uid); e != nil {
		return nil, e
	}
	lockpath := filepath.Join(dir, "service.lock")
	fd, e := syscall.Open(lockpath, syscall.O_CREAT|syscall.O_RDWR|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0600)
	if e != nil {
		return nil, errUnsafe
	}
	lock := os.NewFile(uintptr(fd), lockpath)
	fail := func() (*Server, error) { lock.Close(); return nil, errUnsafe }
	if _, e = privateFile(lockpath, uid, 0); e != nil {
		return fail()
	}
	if e = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		return fail()
	}
	for _, name := range []string{"service.sock", "capability"} {
		if _, e = os.Lstat(filepath.Join(dir, name)); !errors.Is(e, os.ErrNotExist) {
			return fail()
		}
	}
	cap, e := RandomID()
	if e != nil {
		return fail()
	}
	f, e := os.OpenFile(filepath.Join(dir, "capability"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return fail()
	}
	tokenInfo, _ := f.Stat()
	_, writeErr := f.WriteString(cap)
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil {
		removeOwned(filepath.Join(dir, "capability"), tokenInfo)
		return fail()
	}
	l, e := net.ListenUnix("unix", &net.UnixAddr{Name: filepath.Join(dir, "service.sock"), Net: "unix"})
	if e != nil {
		removeOwned(filepath.Join(dir, "capability"), tokenInfo)
		return fail()
	}
	l.SetUnlinkOnClose(false)
	socketInfo, _ := os.Lstat(filepath.Join(dir, "service.sock"))
	if e = os.Chmod(filepath.Join(dir, "service.sock"), 0600); e != nil {
		l.Close()
		removeOwned(filepath.Join(dir, "service.sock"), socketInfo)
		removeOwned(filepath.Join(dir, "capability"), tokenInfo)
		return fail()
	}
	return &Server{started: time.Now(), identity: identity, dir: dir, uid: uid, capability: cap, service: svc, listener: l, lock: lock, socketInfo: socketInfo, tokenInfo: tokenInfo, connections: make(map[*net.UnixConn]bool), slots: make(chan struct{}, 8)}, nil
}
func (s *Server) Serve(ctx context.Context) error {
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			s.Close()
		case <-done:
		}
	}()
	defer s.Close()
	for {
		c, e := s.listener.AcceptUnix()
		if e != nil {
			if ctx.Err() != nil || errors.Is(e, net.ErrClosed) {
				return nil
			}
			return errors.New("IPC listener failed")
		}
		select {
		case s.slots <- struct{}{}:
		default:
			c.Close()
			continue
		}
		s.mu.Lock()
		if s.closing {
			s.mu.Unlock()
			c.Close()
			<-s.slots
			continue
		}
		s.connections[c] = true
		s.wg.Add(1)
		s.mu.Unlock()
		deadline := time.Now().Add(Budget)
		go func() {
			defer s.wg.Done()
			defer func() { c.Close(); s.mu.Lock(); delete(s.connections, c); s.mu.Unlock(); <-s.slots }()
			_ = c.SetDeadline(deadline)
			requestCtx, cancel := context.WithDeadline(ctx, deadline)
			defer cancel()
			s.handle(requestCtx, c)
		}()
	}
}
func (s *Server) Close() {
	s.once.Do(func() {
		s.listener.Close()
		s.mu.Lock()
		s.closing = true
		for c := range s.connections {
			c.Close()
		}
		s.mu.Unlock()
	})
	s.wg.Wait()
	s.cleanup.Do(func() {
		removeOwned(filepath.Join(s.dir, "service.sock"), s.socketInfo)
		removeOwned(filepath.Join(s.dir, "capability"), s.tokenInfo)
		s.lock.Close()
		if s.equipment != nil {
			s.equipment.Close()
		}
		if s.closeState != nil {
			s.closeState()
		}
	})
}
func (s *Server) handle(ctx context.Context, c *net.UnixConn) {
	uid, e := peerUID(c)
	if e != nil || uid != s.uid {
		_ = writeFrame(c, Response{Version: 1, Code: foundation.Denied})
		return
	}
	b, e := readFrameLimit(c, MaxBrandFrame)
	if e != nil {
		_ = writeFrame(c, Response{Version: 1, Code: foundation.Invalid})
		return
	}
	q, code := validate(b)
	r := Response{Version: 1, RequestID: q.RequestID, Code: code}
	if !validID(q.RequestID) {
		r.RequestID = ""
	}
	if code == foundation.OK {
		if s.account != nil {
			owner, err := peerOwner(c)
			if err != nil {
				r.Code = foundation.Unavailable
			} else {
				r = s.dispatchAccess(ctx, uid, q, owner)
			}
		} else {
			r = s.dispatch(ctx, uid, q)
		}
	}
	if ctx.Err() != nil {
		r = Response{Version: 1, RequestID: r.RequestID, Code: foundation.Deadline}
	}
	limit := MaxFrame
	if q.Op == "company.brand-get" || q.Op == "company.brand-set" {
		limit = MaxBrandFrame
	}
	_ = writeFrameLimit(c, r, limit)
}
func (s *Server) dispatch(ctx context.Context, uid uint32, q Request) Response {
	r := Response{Version: 1, RequestID: q.RequestID, Code: foundation.OK}
	if s.account != nil {
		return s.dispatchAccess(ctx, uid, q, "")
	}
	if s.service == nil {
		r.Code = foundation.Invalid
		return r
	}
	if q.Op == "handshake" {
		if subtle.ConstantTimeCompare([]byte(q.Capability), []byte(s.capability)) != 1 {
			r.Code = foundation.Unauthenticated
			return r
		}
		id, e := RandomID()
		if e != nil {
			r.Code = foundation.Unavailable
			return r
		}
		v, code := s.service.Open(uid, q.Profile, id)
		r.Code = code
		if code != foundation.OK {
			return r
		}
		// Get the activation snapshot via the composition-owned service identity.
		r.Data = &Data{Session: v.ID, Expires: v.Expires.UTC().Format(time.RFC3339Nano), Profile: foundation.Profile, Instance: s.identity.Instance, Activation: s.identity.Activation}
		return r
	}
	scope := foundation.Scope{Session: q.Session, UID: uid, Profile: q.Profile, Activation: q.Activation}
	if q.Op == "foundation.observe" {
		v, code := s.service.Observe(ctx, scope, q.Label)
		r.Code = code
		if code != foundation.OK {
			return r
		}
		r.Data = &Data{Value: &v.Value, At: v.At.Format(time.RFC3339Nano), Label: q.Label, Source: "synthetic", Quality: "synthetic", Instance: s.identity.Instance, Activation: s.identity.Activation, Profile: foundation.Profile}
		return r
	}
	v, code := s.service.Snapshot(scope)
	r.Code = code
	if code == foundation.OK {
		r.Data = &Data{Instance: v.Instance, Activation: v.Activation, Profile: foundation.Profile, Heartbeat: v.Heartbeat, Observations: v.Observations}
	}
	return r
}
