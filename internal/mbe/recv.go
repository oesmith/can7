package mbe

import (
	"fmt"
)

type recv struct {
	buf  [4095]byte // Maximum message size is 4096 (2 ** 12) bytes
	size int        // Expected data length
	next int        // Expected next frame index
	recv int        // Bytes received so far
}

func (r *recv) update(d [8]byte) error {
	switch d[0] & 0xf0 {
	case 0x0: // Single-frame sequence.
		r.size = int(d[0] & 0xf)
		r.recv = r.size
		r.next = 0
		copy(r.buf[:r.size], d[1:])
	case 0x10: // New multi-frame sequence.
		r.size = int(d[0]&0xf)*256 + int(d[1])
		r.recv = copy(r.buf[:r.size], d[2:])
		r.next = 1
	case 0x20: // Continuation of a multi-frame sequence.
		idx := int(d[0] & 0x0f)
		if idx == r.next {
			r.next = (r.next + 1) & 0xf
			r.recv += copy(r.buf[r.recv:r.size], d[1:])
		}
	default:
		return fmt.Errorf("Bad isotp frame type: %d", d[0]>>4)
	}
	return nil
}

func (r recv) isComplete() bool {
	return r.recv >= r.size
}

func (r recv) bytes() []byte {
	return r.buf[:r.recv]
}
