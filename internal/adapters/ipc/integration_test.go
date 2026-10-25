package ipc

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"infradesktop/internal/adapters/synthetic"
	"infradesktop/internal/modules/foundation"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type running struct {
	s      *Server
	dir    string
	cancel context.CancelFunc
	done   chan error
}

func start(t *testing.T, source foundation.Source, now func() time.Time, dir string, instance string) *running {
	t.Helper()
	if dir == "" {
		dir = t.TempDir()
	}
	if e := os.Chmod(dir, 0700); e != nil {
		t.Fatal(e)
	}
	svc := foundation.New(uint32(os.Getuid()), instance, instance+"-activation", now, source)
	s, e := Listen(dir, uint32(os.Getuid()), svc, foundation.Status{Instance: instance, Activation: instance + "-activation"})
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := &running{s, dir, cancel, make(chan error, 1)}
	go func() { r.done <- s.Serve(ctx) }()
	t.Cleanup(func() { r.stop(t) })
	return r
}
func (r *running) stop(t *testing.T) {
	t.Helper()
	if r.cancel == nil {
		return
	}
	r.cancel()
	select {
	case e := <-r.done:
		if e != nil {
			t.Error(e)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown exceeded 5s")
	}
	r.cancel = nil
}
func dial(t *testing.T, r *running) *net.UnixConn {
	t.Helper()
	c, e := net.DialUnix("unix", nil, &net.UnixAddr{Name: filepath.Join(r.dir, "service.sock"), Net: "unix"})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { c.Close() })
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	return c
}
func raw(t *testing.T, r *running, b []byte) Response {
	t.Helper()
	c := dial(t, r)
	var h [4]byte
	binary.BigEndian.PutUint32(h[:], uint32(len(b)))
	if _, e := c.Write(append(h[:], b...)); e != nil {
		t.Fatal(e)
	}
	b, e := readFrame(c)
	if e != nil {
		t.Fatal(e)
	}
	var v Response
	if e = json.Unmarshal(b, &v); e != nil {
		t.Fatal(e)
	}
	return v
}
func req(t *testing.T, r *running, q Request) Response {
	t.Helper()
	b, e := json.Marshal(q)
	if e != nil {
		t.Fatal(e)
	}
	return raw(t, r, b)
}
func handshake(t *testing.T, r *running) Request {
	t.Helper()
	cap, e := readCapability(r.dir, uint32(os.Getuid()))
	if e != nil {
		t.Fatal(e)
	}
	v := req(t, r, Request{Version: 1, RequestID: "hello", Op: "handshake", Capability: cap, Profile: foundation.Profile})
	if v.Code != foundation.OK || v.Data == nil || len(v.Data.Session) != 64 {
		t.Fatal(v)
	}
	return Request{Version: 1, RequestID: "test", Op: "foundation.status", Session: v.Data.Session, Profile: foundation.Profile, Activation: v.Data.Activation}
}
func TestIntegrationRoundtripAuthAndRestart(t *testing.T) {
	now := time.Date(2026, 10, 22, 16, 0, 0, 0, time.UTC)
	// Clock mutation is serialized by request/response completion (not a concurrent TTL ticker).
	var clock atomic.Int64
	clock.Store(now.UnixNano())
	r := start(t, synthetic.Source{}, func() time.Time { return time.Unix(0, clock.Load()).UTC() }, "", "first")
	q := handshake(t, r)
	cap, _ := readCapability(r.dir, uint32(os.Getuid()))
	for _, name := range []string{"capability", "service.sock"} {
		st, e := os.Lstat(filepath.Join(r.dir, name))
		if e != nil || st.Mode().Perm() != 0600 {
			t.Fatal(name, e)
		}
	}
	for _, tc := range []struct {
		name   string
		change func(*Request)
		want   foundation.Code
	}{
		{"missing", func(q *Request) { q.Session = "" }, foundation.Unauthenticated}, {"wrong", func(q *Request) { q.Session = strings.Repeat("0", 64) }, foundation.Unauthenticated}, {"profile", func(q *Request) { q.Profile = "other" }, foundation.ProfileChanged}, {"activation", func(q *Request) { q.Activation = "old" }, foundation.ProfileChanged}, {"version", func(q *Request) { q.Version = 2 }, foundation.Version}, {"operation", func(q *Request) { q.Op = "exec" }, foundation.Invalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			x := q
			tc.change(&x)
			if v := req(t, r, x); v.Code != tc.want || v.Data != nil {
				t.Fatal(v)
			}
		})
	}
	if v := req(t, r, Request{Version: 1, RequestID: "wrongcap", Op: "handshake", Profile: foundation.Profile, Capability: strings.Repeat("0", 64)}); v.Code != foundation.Unauthenticated {
		t.Fatal(v)
	}
	if v := req(t, r, Request{Version: 1, RequestID: "wrongprofile", Op: "handshake", Profile: "other", Capability: cap}); v.Code != foundation.ProfileChanged {
		t.Fatal(v)
	}
	q.Op = "foundation.observe"
	q.Label = "L01"
	v := req(t, r, q)
	if v.Code != foundation.OK || v.Data == nil || v.Data.Value == nil || *v.Data.Value != 173 || v.Data.Source != "synthetic" || v.Data.Quality != "synthetic" || v.Data.At != "2026-10-22T16:00:00Z" || v.RequestID != "test" {
		t.Fatal(v)
	}
	q.Label = "/bin/sh"
	if v = req(t, r, q); v.Code != foundation.Invalid {
		t.Fatal(v)
	}
	q.Label = ""
	q.Op = "foundation.status"
	if v = req(t, r, q); v.Code != foundation.OK || v.Data.Observations != 1 {
		t.Fatal(v)
	}
	clock.Add(int64(foundation.TTL))
	if v = req(t, r, q); v.Code != foundation.Unauthenticated {
		t.Fatal(v)
	}
	client := NewClient(r.dir, uint32(os.Getuid()))
	if v = client.Call(context.Background(), "foundation.status", ""); v.Code != foundation.OK {
		t.Fatal(v)
	}
	r.stop(t)
	for _, name := range []string{"capability", "service.sock"} {
		if _, e := os.Lstat(filepath.Join(r.dir, name)); !os.IsNotExist(e) {
			t.Fatal(name, e)
		}
	}
	if v = client.Call(context.Background(), "foundation.status", ""); v.Code != foundation.Unavailable {
		t.Fatal(v)
	}
	r2 := start(t, synthetic.Source{}, time.Now, r.dir, "second")
	if v = req(t, r2, q); v.Code != foundation.Unauthenticated {
		t.Fatal(v)
	}
	if v = client.Call(context.Background(), "foundation.status", ""); v.Code != foundation.OK || v.Data.Instance != "second" {
		t.Fatal(v)
	}
}
func TestIntegrationStrictFraming(t *testing.T) {
	r := start(t, synthetic.Source{}, time.Now, "", "frames")
	for _, b := range []string{`{}`, `null`, `[]`, `{`, `{"version":1,"request_id":"x","op":"foundation.status","extra":1}`, `{"version":1,"request_id":"x","op":"foundation.status","version":1}`, `{"version":1,"request_id":"x","op":"foundation.status","session":null}`, `{"version":1,"request_id":"x","op":"foundation.status","label":""}`, `{"version":1,"request_id":"x","op":"foundation.status"} {}`, `{"version":1,"request_id":"x","op":"foundation.status","session":123}`, string([]byte{255})} {
		if v := raw(t, r, []byte(b)); v.Code != foundation.Invalid {
			t.Fatalf("%q: %+v", b, v)
		}
	}
	for _, n := range []uint32{0, MaxFrame + 1, 0xffffffff} {
		c := dial(t, r)
		var h [4]byte
		binary.BigEndian.PutUint32(h[:], n)
		c.Write(h[:])
		b, e := readFrame(c)
		if e != nil {
			t.Fatal(e)
		}
		var v Response
		json.Unmarshal(b, &v)
		if v.Code != foundation.Invalid {
			t.Fatal(v)
		}
	}
	c := dial(t, r)
	c.Write([]byte{0, 0, 0, 20, '{'})
	c.CloseWrite()
	b, e := readFrame(c)
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Contains(b, []byte("INVALID_REQUEST")) {
		t.Fatal(string(b))
	}
	c = dial(t, r)
	c.Write([]byte{0, 0})
	c.CloseWrite()
	if b, e = readFrame(c); e != nil || !bytes.Contains(b, []byte("INVALID_REQUEST")) {
		t.Fatal(string(b), e)
	}
	// Maximum declared payload is bounded and parsed; over-limit is rejected before body allocation.
	if v := raw(t, r, bytes.Repeat([]byte{' '}, MaxFrame)); v.Code != foundation.Invalid {
		t.Fatal(v)
	}
}

type blockingSource struct {
	entered chan struct{}
	release chan struct{}
}

func (s blockingSource) Read(ctx context.Context, _ string) (int, foundation.Code) {
	s.entered <- struct{}{}
	select {
	case <-s.release:
		return 173, foundation.OK
	case <-ctx.Done():
		return 0, foundation.Deadline
	}
}
func TestIntegrationConcurrencyAndDeadline(t *testing.T) {
	source := blockingSource{make(chan struct{}, 4), make(chan struct{})}
	r := start(t, source, time.Now, "", "slow")
	q := handshake(t, r)
	q.Op = "foundation.observe"
	q.Label = "L01"
	results := make(chan Response, 2)
	errors := make(chan error, 2)
	for range 2 {
		go func() {
			c, e := net.Dial("unix", filepath.Join(r.dir, "service.sock"))
			if e != nil {
				errors <- e
				return
			}
			defer c.Close()
			c.SetDeadline(time.Now().Add(5 * time.Second))
			if e = writeFrame(c, q); e != nil {
				errors <- e
				return
			}
			b, e := readFrame(c)
			if e != nil {
				errors <- e
				return
			}
			var v Response
			e = json.Unmarshal(b, &v)
			if e != nil {
				errors <- e
				return
			}
			results <- v
		}()
	}
	for range 2 {
		select {
		case <-source.entered:
		case e := <-errors:
			t.Fatal(e)
		case <-time.After(time.Second):
			t.Fatal("work did not start")
		}
	}
	if v := req(t, r, q); v.Code != foundation.Busy {
		t.Fatal(v)
	}
	status := q
	status.Op = "foundation.status"
	status.Label = ""
	before := time.Now()
	if v := req(t, r, status); v.Code != foundation.OK || v.Data.Observations != 0 {
		t.Fatal(v)
	}
	if time.Since(before) > time.Second {
		t.Fatal("status serialized behind observation")
	}
	close(source.release)
	for range 2 {
		select {
		case v := <-results:
			if v.Code != foundation.OK {
				t.Fatal(v)
			}
		case e := <-errors:
			t.Fatal(e)
		}
	}
	c := dial(t, r)
	before = time.Now()
	c.Write([]byte{0})
	var b [1]byte
	_, e := c.Read(b[:])
	elapsed := time.Since(before)
	if e == nil || elapsed < Budget-100*time.Millisecond || elapsed > Budget+time.Second {
		t.Fatal("slow client budget", elapsed, e)
	}
}
func TestIntegrationSaturationShutdownAndUnsafePaths(t *testing.T) {
	r := start(t, synthetic.Source{}, time.Now, "", "limits")
	_ = handshake(t, r)
	if _, e := Listen(r.dir, uint32(os.Getuid()), r.s.service, r.s.identity); e == nil {
		t.Fatal("second listener accepted")
	}
	// Fill all eight connection slots, including unauthenticated peers; wait on observable admission.
	clients := make([]*net.UnixConn, 8)
	for i := range clients {
		clients[i] = dial(t, r)
		clients[i].Write([]byte{0})
	}
	until := time.Now().Add(time.Second)
	for len(r.s.slots) != 8 {
		if time.Now().After(until) {
			t.Fatal("slots not admitted")
		}
		time.Sleep(time.Millisecond)
	}
	c := dial(t, r)
	var b [1]byte
	if _, e := c.Read(b[:]); e == nil {
		t.Fatal("ninth client was admitted")
	}
	before := time.Now()
	r.stop(t)
	if time.Since(before) > 5*time.Second {
		t.Fatal("shutdown")
	}
	for _, c := range clients {
		if _, e := c.Read(b[:]); e == nil {
			t.Fatal("shutdown left connection")
		}
	}
	for _, kind := range []string{"mode", "file", "symlink", "socket", "token", "locklink", "parentlink"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			os.Chmod(dir, 0700)
			sentinel := filepath.Join(dir, "sentinel")
			os.WriteFile(sentinel, []byte("owned fixture"), 0600)
			switch kind {
			case "mode":
				os.Chmod(dir, 0755)
			case "file":
				os.WriteFile(filepath.Join(dir, "service.sock"), []byte("preserve"), 0600)
			case "symlink":
				os.Symlink(sentinel, filepath.Join(dir, "service.sock"))
			case "token":
				os.WriteFile(filepath.Join(dir, "capability"), []byte("preserve"), 0600)
			case "locklink":
				os.Symlink(sentinel, filepath.Join(dir, "service.lock"))
			case "socket":
				l, e := net.ListenUnix("unix", &net.UnixAddr{Name: filepath.Join(dir, "service.sock"), Net: "unix"})
				if e != nil {
					t.Fatal(e)
				}
				defer l.Close()
			case "parentlink":
				link := filepath.Join(t.TempDir(), "linked")
				os.Symlink(dir, link)
				dir = link
			}
			if s, e := Listen(dir, uint32(os.Getuid()), r.s.service, r.s.identity); e == nil {
				s.Close()
				t.Fatal("unsafe path accepted")
			}
			if b, e := os.ReadFile(sentinel); e != nil || string(b) != "owned fixture" {
				t.Fatal("foreign resource changed")
			}
		})
	}
}
func TestFrameReaderBoundaries(t *testing.T) {
	for _, b := range [][]byte{{}, {0}, {0, 0, 0, 2, '{'}} {
		if _, e := readFrame(bytes.NewReader(b)); e == nil {
			t.Fatal("truncated accepted")
		}
	}
	var out bytes.Buffer
	if e := writeFrame(&out, map[string]int{"x": 1}); e != nil {
		t.Fatal(e)
	}
	if b, e := readFrame(&out); e != nil || string(b) != `{"x":1}` {
		t.Fatal(string(b), e)
	}
	if _, e := readFrame(io.LimitReader(strings.NewReader("abcd"), 0)); e == nil {
		t.Fatal("empty")
	}
}

func TestIntegrationObservationDeadlineAndClientCancel(t *testing.T) {
	source := blockingSource{make(chan struct{}, 4), make(chan struct{})}
	r := start(t, source, time.Now, "", "deadline")
	client := NewClient(r.dir, uint32(os.Getuid()))
	before := time.Now()
	v := client.Call(context.Background(), "foundation.observe", "L01")
	elapsed := time.Since(before)
	if v.Code != foundation.Deadline || elapsed < Budget-100*time.Millisecond || elapsed > Budget+time.Second {
		t.Fatal(v.Code, elapsed)
	}
	// Status remains responsive after expired work; then verify immediate client cancellation.
	status := client.Call(context.Background(), "foundation.status", "")
	if status.Code != foundation.OK || status.Data.Observations != 0 {
		t.Fatal(status.Code)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	before = time.Now()
	v = client.Call(ctx, "foundation.status", "")
	if v.Code != foundation.Deadline || time.Since(before) > time.Second {
		t.Fatal(v.Code, time.Since(before))
	}
}

func TestIntegrationClientRejectsUnsafeEndpoint(t *testing.T) {
	r := start(t, synthetic.Source{}, time.Now, "", "client-path")
	for _, name := range []string{"capability", "service.sock"} {
		p := filepath.Join(r.dir, name)
		if e := os.Chmod(p, 0644); e != nil {
			t.Fatal(e)
		}
		c := NewClient(r.dir, uint32(os.Getuid()))
		if v := c.Call(context.Background(), "foundation.status", ""); v.Code != foundation.Unavailable {
			t.Fatal(name, v.Code)
		}
		if e := os.Chmod(p, 0600); e != nil {
			t.Fatal(e)
		}
	}
	// Client policy injection only: this does not claim a real cross-UID process test.
	if v := NewClient(r.dir, uint32(os.Getuid())+1).Call(context.Background(), "foundation.status", ""); v.Code != foundation.Unavailable {
		t.Fatal(v.Code)
	}
}

func TestIntegrationCleanupPreservesReplacement(t *testing.T) {
	r := start(t, synthetic.Source{}, time.Now, "", "cleanup")
	token := filepath.Join(r.dir, "capability")
	original := filepath.Join(r.dir, "original-owned-token")
	if e := os.Rename(token, original); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(token, []byte("replacement fixture"), 0600); e != nil {
		t.Fatal(e)
	}
	r.stop(t)
	if b, e := os.ReadFile(token); e != nil || string(b) != "replacement fixture" {
		t.Fatal("cleanup removed another resource", e)
	}
}
