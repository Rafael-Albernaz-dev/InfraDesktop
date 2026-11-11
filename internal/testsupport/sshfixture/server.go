// Package sshfixture is a controlled loopback peer used only by isolated tests.
package sshfixture

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"golang.org/x/crypto/ssh"
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/adapters/routerssh/deviceparse"
	"infradesktop/internal/adapters/routerssh/pppoeparse"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

const Password = "local-fixture-only"

type Config struct {
	OnHandshake                 func()
	OnPassword                  func()
	Family, Mode                string
	ServerVersion               string
	Identity, InterfaceName     string
	InterfaceMode               func() string
	PPPoE                       func() string
	Command                     func(string) (string, bool)
	DisconnectCommand           func(string) bool
	PPRows                      int
	RealTime                    bool
	IgnoreGlobalRequests        bool
	UnimplementedGlobalRequests bool
	Stderr                      string
	ExitStatus                  uint32
	Hold                        <-chan struct{}
}
type Server struct {
	listener       net.Listener
	config         Config
	signer         ssh.Signer
	mu             sync.Mutex
	conns          map[net.Conn]bool
	closed         chan struct{}
	wg             sync.WaitGroup
	Auth           atomic.Int32
	Handshakes     atomic.Int32
	Commands       atomic.Int32
	PPCalls        atomic.Int32
	PingCalls      atomic.Int32
	GlobalRequests atomic.Int32
	started        time.Time
}

func Start(config Config) (*Server, error) {
	_, key, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		return nil, e
	}
	signer, e := ssh.NewSignerFromKey(key)
	if e != nil {
		return nil, e
	}
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		return nil, e
	}
	s := &Server{listener: l, config: config, signer: signer, conns: map[net.Conn]bool{}, closed: make(chan struct{}), started: time.Now()}
	s.wg.Add(1)
	go s.serve()
	return s, nil
}
func (s *Server) Port() int              { return s.listener.Addr().(*net.TCPAddr).Port }
func (s *Server) ActiveConnections() int { s.mu.Lock(); defer s.mu.Unlock(); return len(s.conns) }
func (s *Server) Fingerprint() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return ssh.FingerprintSHA256(s.signer.PublicKey())
}

// RotateHostKey simulates replacement at the same endpoint, without changing
// the client's stored pin or allowing a password to precede pin verification.
func (s *Server) RotateHostKey() error {
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.signer = signer
	s.mu.Unlock()
	return nil
}
func (s *Server) serve() {
	defer s.wg.Done()
	for {
		conn, e := s.listener.Accept()
		if e != nil {
			return
		}
		s.mu.Lock()
		select {
		case <-s.closed:
			s.mu.Unlock()
			conn.Close()
			return
		default:
		}
		s.conns[conn] = true
		s.wg.Add(1)
		s.mu.Unlock()
		go func() {
			defer s.wg.Done()
			defer func() { conn.Close(); s.mu.Lock(); delete(s.conns, conn); s.mu.Unlock() }()
			s.handle(conn)
		}()
	}
}
func (s *Server) handle(conn net.Conn) {
	if s.config.Mode == "handshake" {
		<-s.closed
		return
	}
	config := &ssh.ServerConfig{ServerVersion: s.config.ServerVersion, PasswordCallback: func(meta ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
		s.Auth.Add(1)
		if s.config.OnPassword != nil {
			s.config.OnPassword()
		}
		if s.config.Mode == "disconnect-auth" {
			conn.Close()
			return nil, nil
		}
		if meta.User() != "reader" || string(password) != Password || s.config.Mode == "auth" {
			return nil, fmt.Errorf("denied")
		}
		return nil, nil
	}}
	s.mu.Lock()
	signer := s.signer
	s.mu.Unlock()
	config.AddHostKey(signer)
	if s.config.OnHandshake != nil {
		s.config.OnHandshake()
	}
	s.Handshakes.Add(1)
	peer, channels, requests, e := ssh.NewServerConn(conn, config)
	if e != nil {
		return
	}
	defer peer.Close()
	go func() {
		for request := range requests {
			s.GlobalRequests.Add(1)
			if s.config.UnimplementedGlobalRequests {
				_ = sendUnimplemented(peer)
			} else if !s.config.IgnoreGlobalRequests {
				_ = request.Reply(false, nil)
			}
		}
	}()

	for channel := range channels {
		if channel.ChannelType() != "session" {
			channel.Reject(ssh.Prohibited, "only session")
			continue
		}
		ch, reqs, e := channel.Accept()
		if e != nil {
			continue
		}
		func() {
			defer ch.Close()
			for req := range reqs {
				if req.Type != "exec" {
					req.Reply(false, nil)
					continue
				}
				var exec struct{ Command string }
				if ssh.Unmarshal(req.Payload, &exec) != nil {
					req.Reply(false, nil)
					return
				}
				s.Commands.Add(1)
				req.Reply(true, nil)
				if s.config.Hold != nil {
					select {
					case <-s.config.Hold:
					case <-s.closed:
						return
					}
				}
				if s.config.DisconnectCommand != nil && s.config.DisconnectCommand(exec.Command) {
					return
				}
				out, ok := s.output(exec.Command)
				status := s.config.ExitStatus
				if !ok {
					status = 1
				}
				_, _ = ch.Stderr().Write([]byte(s.config.Stderr))
				_, _ = ch.Write([]byte(out))
				_, _ = ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{status}))
				return
			}
		}()
	}
}
func (s *Server) output(command string) (string, bool) {
	if s.config.Command != nil {
		if output, ok := s.config.Command(command); ok {
			return output, true
		}
	}
	if s.config.Mode == "permission" {
		return "failure: not enough permissions\r\n", true
	}
	if s.config.Mode == "large" {
		return string(make([]byte, routerssh.MaxOutput+1)), true
	}
	if s.config.Mode == "malformed" {
		return "unexpected\n", true
	}
	if result, ok := s.diagnosticOutput(command); ok {
		return result, true
	}
	switch command {
	case pppoeparse.PPPoECommand:
		if s.config.PPPoE != nil {
			return s.config.PPPoE(), true
		}
		return s.ppOutput(), true
	case deviceparse.IdentityCommand:
		if s.config.Identity != "" {
			return "identity=" + s.config.Identity + "\r\n", true
		}
		return "identity=Fixture router\r\n", true
	case deviceparse.ResourceCommand:
		version := "7.26 (stable)"
		if s.config.Family == "v6" {
			version = "6.49.19 (long-term)"
		}
		if s.config.Mode == "unsupported" {
			version = "5.26"
		}
		out := "version=" + version + "\r\nboard=Test board\r\nuptime=1d2h\r\ncpu=12\r\nfree=1024\r\ntotal=2048\r\ndone\r\n"
		if s.config.Mode == "partial" {
			out = out[:len(out)-6]
		}
		return out, true
	case deviceparse.InterfacesCommand:
		mode := s.config.Mode
		if s.config.InterfaceMode != nil {
			mode = s.config.InterfaceMode()
		}
		if mode == "interfaces-permission" {
			return "failure: not enough permissions\n", true
		}
		if mode == "interfaces-malformed" {
			return "unexpected\n", true
		}
		if mode == "interfaces-large" {
			return string(make([]byte, routerssh.MaxOutput+1)), true
		}
		count, eligible, dynamic := 1, 1, 0
		if mode == "interfaces" {
			count, eligible, dynamic = 128, 129, 4096
		}
		if mode == "dynamic" {
			dynamic = 10000
		}
		out := fmt.Sprintf("eligible=%d\ndynamic=%d\n", eligible, dynamic)
		for i := 0; i < count; i++ {
			name := "ether" + strconv.Itoa(i+1)
			if s.config.InterfaceName != "" {
				name = s.config.InterfaceName
			}
			out += "name=" + name + "\ntype=ether\nrunning=true\ndisabled=false\nend\n"
		}
		return out + "done\n", true

	}
	return "unknown fixed command", false
}
func (s *Server) Close() {
	s.mu.Lock()
	select {
	case <-s.closed:
		s.mu.Unlock()
		return
	default:
		close(s.closed)
	}
	s.listener.Close()
	for c := range s.conns {
		c.Close()
	}
	s.mu.Unlock()
	s.wg.Wait()
}
