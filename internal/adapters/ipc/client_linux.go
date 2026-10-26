package ipc

import (
	"context"
	"infradesktop/internal/modules/foundation"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Client keeps all capability/session material outside JS and serializes only handshakes.
type Client struct {
	dir                           string
	uid                           uint32
	mu                            sync.Mutex
	auth                          chan struct{}
	session, activation, instance string
}

func NewClient(dir string, uid uint32) *Client {
	return &Client{dir: dir, uid: uid, auth: make(chan struct{}, 1)}
}
func (c *Client) reset() {
	c.mu.Lock()
	c.session = ""
	c.activation = ""
	c.instance = ""
	c.mu.Unlock()
}
func (c *Client) Call(ctx context.Context, op, label string) Response {
	ctx, cancel := context.WithTimeout(ctx, Budget)
	defer cancel()
	if safeDir(c.dir, c.uid) != nil {
		return Response{Version: 1, Code: foundation.Unavailable}
	}
	select {
	case c.auth <- struct{}{}:
	case <-ctx.Done():
		return Response{Version: 1, Code: foundation.Deadline}
	}
	c.mu.Lock()
	needs := c.session == ""
	c.mu.Unlock()
	if needs {
		cap, e := readCapability(c.dir, c.uid)
		if e != nil {
			<-c.auth
			return Response{Version: 1, Code: foundation.Unavailable}
		}
		id, e := RandomID()
		if e != nil {
			<-c.auth
			return Response{Version: 1, Code: foundation.Unavailable}
		}
		r := c.exchange(ctx, Request{Version: 1, RequestID: id, Op: "handshake", Capability: cap, Profile: foundation.Profile})
		if r.Code != foundation.OK || r.Data == nil || r.Data.Session == "" {
			<-c.auth
			return r
		}
		c.mu.Lock()
		c.session = r.Data.Session
		c.activation = r.Data.Activation
		c.instance = r.Data.Instance
		c.mu.Unlock()
	}
	<-c.auth
	id, e := RandomID()
	if e != nil {
		return Response{Version: 1, Code: foundation.Unavailable}
	}
	c.mu.Lock()
	q := Request{Version: 1, RequestID: id, Op: op, Session: c.session, Activation: c.activation, Profile: foundation.Profile, Label: label}
	c.mu.Unlock()
	r := c.exchange(ctx, q)
	if r.Code == foundation.Unavailable || r.Code == foundation.Unauthenticated || r.Code == foundation.ProfileChanged {
		c.reset()
	}
	return r
}
func (c *Client) exchange(parent context.Context, q Request) Response {
	ctx, cancel := context.WithTimeout(parent, Budget)
	defer cancel()
	failure := func() Response {
		code := foundation.Unavailable
		deadline, _ := ctx.Deadline()
		if ctx.Err() != nil || !time.Now().Before(deadline) {
			code = foundation.Deadline
		}
		return Response{Version: 1, RequestID: q.RequestID, Code: code}
	}
	if safeDir(c.dir, c.uid) != nil {
		return failure()
	}
	socket := filepath.Join(c.dir, "service.sock")
	if _, e := privateFile(socket, c.uid, os.ModeSocket); e != nil {
		return failure()
	}
	conn, e := (&net.Dialer{}).DialContext(ctx, "unix", socket)
	if e != nil {
		return failure()
	}
	defer conn.Close()
	uc, ok := conn.(*net.UnixConn)
	if !ok {
		return failure()
	}
	uid, e := peerUID(uc)
	if e != nil || uid != c.uid {
		return failure()
	}
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	deadline, _ := ctx.Deadline()
	_ = conn.SetDeadline(deadline)
	limit := MaxFrame
	if q.Op == "company.brand-set" {
		limit = MaxBrandFrame
	}
	if writeFrameLimit(conn, q, limit) != nil {
		return failure()
	}
	limit = MaxFrame
	if q.Op == "company.brand-get" || q.Op == "company.brand-set" {
		limit = MaxBrandFrame
	}
	b, e := readFrameLimit(conn, uint32(limit))
	if e != nil {
		return failure()
	}
	var r Response
	if strict(b, &r) != nil || r.Version != 1 || r.RequestID != q.RequestID {
		return failure()
	}
	if !validErrorDetail(r) {
		return failure()
	}
	switch r.Code {
	case foundation.OK:
		if r.Data == nil {
			return failure()
		}
	case foundation.Invalid, foundation.Version, foundation.Unauthenticated, foundation.Denied, foundation.ProfileChanged, foundation.Deadline, foundation.Busy, foundation.Unavailable, "ACCOUNT_EXISTS", "TRY_LATER", "BRAND_NAME_INVALID", "BRAND_LOGO_INVALID", "BRAND_ACCENT_INVALID", "REVISION_CONFLICT", "VAULT_UNAVAILABLE", "CREDENTIAL_NOT_FOUND", "DEVICE_INACTIVE", "TRUST_REQUIRED", "TRUST_CHANGED", "LIMIT_EXCEEDED", "CANCELLED", "PARTIAL", "JOB_INTERRUPTED", "REMOTE_AUTH_FAILED", "REMOTE_PERMISSION_DENIED", "UNSUPPORTED", "CONNECT_FAILED", "SSH_NEGOTIATION_FAILED", "SSH_CONNECTION_LOST", "PERSISTENCE_FAILED", "DEVICE_NAME_INVALID", "DEVICE_ADDRESS_INVALID", "DEVICE_PORT_INVALID", "CREDENTIAL_ALIAS_INVALID", "CREDENTIAL_USER_INVALID", "CREDENTIAL_SECRET_INVALID", "OBSERVATION_UNAVAILABLE", "OBSERVATION_INCOMPATIBLE", "CURSOR_INVALID":
		r.Data = nil
	default:
		return failure()
	}
	return r
}
