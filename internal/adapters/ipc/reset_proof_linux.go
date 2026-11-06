package ipc

import (
	"crypto/subtle"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

type resetFileProof struct {
	dir string
	uid uint32
}

func (p resetFileProof) Consume(nonce string) bool {
	if len(nonce) != 64 {
		return false
	}
	decoded, err := hex.DecodeString(nonce)
	if err != nil || hex.EncodeToString(decoded) != nonce {
		return false
	}
	dirfd, err := syscall.Open(p.dir, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return false
	}
	defer syscall.Close(dirfd)
	var directory syscall.Stat_t
	if syscall.Fstat(dirfd, &directory) != nil || directory.Uid != p.uid || directory.Mode&0777 != 0700 {
		return false
	}
	name := "reset-" + nonce
	fd, err := syscall.Openat(dirfd, name, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC|syscall.O_NONBLOCK, 0)
	if err != nil {
		return false
	}
	file := os.NewFile(uintptr(fd), name)
	defer file.Close()
	var st syscall.Stat_t
	if syscall.Fstat(fd, &st) != nil || !validResetProofStat(st) {
		return false
	}
	data := make([]byte, 65)
	n, err := io.ReadFull(file, data)
	if err != io.ErrUnexpectedEOF || n != 64 || subtle.ConstantTimeCompare(data[:n], []byte(nonce)) != 1 {
		return false
	}
	return os.Remove(filepath.Join(p.dir, name)) == nil
}

func validResetProofStat(st syscall.Stat_t) bool {
	return st.Uid == 0 && st.Gid == 0 && st.Mode&syscall.S_IFMT == syscall.S_IFREG &&
		st.Mode&0022 == 0 && st.Nlink == 1 && st.Size == 64
}
