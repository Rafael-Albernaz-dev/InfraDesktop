package routerssh

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net"
	"reflect"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestL07LegacyNegotiation(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprint("legacy=", legacy), func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			server := &ssh.ServerConfig{NoClientAuth: true}
			serverSigner := signer
			if legacy {
				server.KeyExchanges = []string{ssh.InsecureKeyExchangeDH14SHA1}
				serverSigner, err = ssh.NewSignerWithAlgorithms(signer.(ssh.AlgorithmSigner), []string{ssh.KeyAlgoRSA})
				if err != nil {
					t.Fatal(err)
				}
			}
			server.AddHostKey(serverSigner)
			done := make(chan error, 1)
			go func() {
				conn, err := listener.Accept()
				if err != nil {
					done <- err
					return
				}
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
				peer, _, _, err := ssh.NewServerConn(conn, server)
				if err == nil {
					peer.Close()
				}
				done <- err
			}()
			cfg := config("synthetic")
			cfg.HostKeyCallback = func(_ string, _ net.Addr, key ssh.PublicKey) error {
				if ssh.FingerprintSHA256(key) != ssh.FingerprintSHA256(signer.PublicKey()) {
					return fmt.Errorf("pin mismatch")
				}
				return nil
			}
			conn, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
			peer, _, _, err := ssh.NewClientConn(conn, listener.Addr().String(), cfg)
			if err != nil {
				t.Fatalf("negotiation: %v", err)
			}
			defer peer.Close()
			a := peer.(ssh.AlgorithmsConnMetadata).Algorithms()
			if legacy {
				if a.KeyExchange != ssh.InsecureKeyExchangeDH14SHA1 || a.HostKey != ssh.KeyAlgoRSA {
					t.Fatalf("legacy peer negotiated %+v", a)
				}
			} else if a.KeyExchange == ssh.InsecureKeyExchangeDH14SHA1 || a.HostKey == ssh.KeyAlgoRSA {
				t.Fatalf("modern peer downgraded: %+v", a)
			}
			if err := <-done; err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestL07OnlyObservedLegacyAlgorithmsFollowModern(t *testing.T) {
	safe, unsafe := ssh.SupportedAlgorithms(), ssh.InsecureAlgorithms()
	c := config("synthetic")
	if !reflect.DeepEqual(c.KeyExchanges, append(safe.KeyExchanges, ssh.InsecureKeyExchangeDH14SHA1)) || !reflect.DeepEqual(c.HostKeyAlgorithms, append(safe.HostKeys, ssh.KeyAlgoRSA)) || !reflect.DeepEqual(c.Ciphers, safe.Ciphers) || !reflect.DeepEqual(c.MACs, safe.MACs) {
		t.Fatal("client must retain modern order and append only the two observed requirements", unsafe)
	}
}
