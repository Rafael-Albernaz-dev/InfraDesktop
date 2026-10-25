package ipc

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// PID plus kernel start ticks prevents PID reuse from inheriting a session.
func processOwner(pid int32) (string, error) {
	return readProcessOwner(pid, os.ReadFile)
}

func readProcessOwner(pid int32, read func(string) ([]byte, error)) (string, error) {
	b, err := read(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return "", err
	}
	end := strings.LastIndexByte(string(b), ')')
	if end < 0 {
		return "", errUnsafe
	}
	fields := strings.Fields(string(b)[end+1:])
	if len(fields) < 20 {
		return "", errUnsafe
	}
	if _, err := strconv.ParseUint(fields[19], 10, 64); err != nil {
		return "", errUnsafe
	}
	if fields[0] == "Z" || fields[0] == "X" {
		return "", os.ErrNotExist
	}
	return fmt.Sprintf("%d:%s", pid, fields[19]), nil
}
func peerOwner(c *net.UnixConn) (string, error) {
	raw, err := c.SyscallConn()
	if err != nil {
		return "", err
	}
	var cred *syscall.Ucred
	var inner error
	err = raw.Control(func(fd uintptr) {
		cred, inner = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	})
	if err != nil {
		return "", err
	}
	if inner != nil {
		return "", inner
	}
	return processOwner(cred.Pid)
}
func ownerAlive(owner string) bool {
	return inspectOwnerAlive(owner, processOwner)
}

func inspectOwnerAlive(owner string, inspect func(int32) (string, error)) bool {
	parts := strings.Split(owner, ":")
	if len(parts) != 2 {
		return false
	}
	pid, err := strconv.ParseInt(parts[0], 10, 32)
	if err != nil {
		return false
	}
	current, err := inspect(int32(pid))
	// Inaccessible /proc must not evict a potentially live window. Its TTL and
	// existing bound still apply; new peers fail closed if identity cannot be read.
	if err != nil {
		return !os.IsNotExist(err)
	}
	return current == owner
}
