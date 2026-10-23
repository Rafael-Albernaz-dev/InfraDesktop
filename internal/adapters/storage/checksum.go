package storage

import (
	"crypto/sha256"
	"fmt"
)

func checksum(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }
