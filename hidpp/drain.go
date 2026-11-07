package hidpp

import (
	"errors"

	"golang.org/x/sys/unix"
)

// drain discards input already waiting on the hidraw node, such as replies
// mirrored from a sibling node, so they cannot be mistaken for the answer to
// the next request. Events are kept via queue().
func (d *Device) drain() {
	buf := make([]byte, 64)
	for {
		fds := []unix.PollFd{{Fd: int32(d.fd), Events: unix.POLLIN}}
		n, err := unix.Poll(fds, 0)
		if err != nil {
			if errors.Is(err, unix.EINTR) {
				continue
			}
			return
		}
		if n == 0 {
			return
		}
		m, err := unix.Read(d.fd, buf)
		if err != nil {
			return
		}
		if msg := buf[:m]; len(msg) >= 4 && msg[1] == d.Index && (msg[0] == reportShort || msg[0] == reportLong) {
			d.queue(msg)
		}
	}
}
