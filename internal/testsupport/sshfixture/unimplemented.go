package sshfixture

import (
	"golang.org/x/crypto/ssh"
	"reflect"
	"unsafe"
)

// The public server API can only reply SUCCESS/FAILURE. This fixture-only hook
// sends the actual RFC4253 UNIMPLEMENTED packet through the encrypted transport.
// Kept outside production adapters; pinned to x/crypto v0.53.0's transport.
//
//go:linkname writePacket golang.org/x/crypto/ssh.(*handshakeTransport).writePacket
func writePacket(unsafe.Pointer, []byte) error

func sendUnimplemented(peer *ssh.ServerConn) error {
	transport := reflect.ValueOf(peer.Conn).Elem().FieldByName("transport").Elem()
	return writePacket(unsafe.Pointer(transport.Pointer()), []byte{3, 0, 0, 0, 0})
}
