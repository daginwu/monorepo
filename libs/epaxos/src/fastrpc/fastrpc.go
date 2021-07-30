package fastrpc

import (
	"io"
)

type Serializable interface {
	Marshal(io.Writer)
	Unmarshal(io.Reader) error
	New() Serializable
}

func Int64ToByteArray(x int64) []byte {
	b := make([]byte, 8)
	b[0] = byte(x)
	b[1] = byte(x >> 8)
	b[2] = byte(x >> 16)
	b[3] = byte(x >> 24)
	b[4] = byte(x >> 32)
	b[5] = byte(x >> 40)
	b[6] = byte(x >> 48)
	b[7] = byte(x >> 56)
	return b
}

func Int64FromByteArray(b []byte) int64 {
	x := uint64(b[0])
	x |= uint64(b[1]) << 8
	x |= uint64(b[2]) << 16
	x |= uint64(b[3]) << 24
	x |= uint64(b[4]) << 32
	x |= uint64(b[5]) << 40
	x |= uint64(b[6]) << 48
	x |= uint64(b[7]) << 56
	return int64(x)
}
