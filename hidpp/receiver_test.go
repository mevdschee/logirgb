package hidpp

import (
	"testing"

	"golang.org/x/sys/unix"
)

// fakeReceiver answers like a receiver at index 0xFF: device slots answer
// pings, the others send the given HID++ 1.0 error.
func fakeReceiver(t *testing.T, slots map[byte]byte) {
	t.Helper()
	old := openFD
	t.Cleanup(func() { openFD = old })
	openFD = func(string) (int, error) {
		fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_SEQPACKET, 0)
		if err != nil {
			return -1, err
		}
		go func(fd int) {
			defer unix.Close(fd)
			buf := make([]byte, 64)
			for {
				n, err := unix.Read(fd, buf)
				if err != nil || n == 0 {
					return
				}
				req := buf[:n]
				idx, fnsw := req[1], req[3]
				code, ok := slots[idx]
				if idx == DeviceIndexDirect {
					code, ok = 0x01, true
				}
				var reply []byte
				switch {
				case ok && code == 0:
					reply = make([]byte, longLen)
					copy(reply, []byte{reportLong, idx, 0, fnsw, 4, 2, req[6]})
				case ok:
					reply = []byte{reportShort, idx, 0x8F, 0, fnsw, code, 0}
				default:
					reply = []byte{reportShort, idx, 0x8F, 0, fnsw, 0x08, 0}
				}
				unix.Write(fd, reply)
			}
		}(fds[1])
		return fds[0], nil
	}
}

func TestOpenAllReceiver(t *testing.T) {
	// Slot 1 and 3 answer, slot 2 is paired but asleep.
	fakeReceiver(t, map[byte]byte{1: 0, 2: err10ResourceError, 3: 0})
	slots, err := OpenAll(Info{Path: "fake"})
	if err != nil {
		t.Fatal(err)
	}
	defer closeSlots(slots)
	want := []struct {
		idx byte
		dev bool
	}{{1, true}, {2, false}, {3, true}}
	if len(slots) != len(want) {
		t.Fatalf("got %d slots, want %d", len(slots), len(want))
	}
	for i, w := range want {
		if slots[i].Index != w.idx || (slots[i].Dev != nil) != w.dev {
			t.Errorf("slot %d: index %d, device %v; want index %d, device %v", i, slots[i].Index, slots[i].Dev != nil, w.idx, w.dev)
		}
		if d := slots[i].Dev; d != nil && d.Index != w.idx {
			t.Errorf("slot %d: device index %d", i, d.Index)
		}
	}
	if slots[0].Dev.fd == slots[2].Dev.fd {
		t.Error("devices on one receiver share a file descriptor")
	}
}
