//go:build windows

package notify

import (
	"os"
	"testing"
	"time"
	"unsafe"
)

// The struct must match the Windows SDK's NOTIFYICONDATAW, or Windows
// ignores every call.
func TestNotifyIconDataSize(t *testing.T) {
	want := uintptr(976)
	if unsafe.Sizeof(uintptr(0)) == 4 {
		want = 956
	}
	if got := unsafe.Sizeof(notifyIconData{}); got != want {
		t.Errorf("sizeof(NOTIFYICONDATAW) = %d, want %d", got, want)
	}
}

func TestCopyUTF16(t *testing.T) {
	var buf [5]uint16
	copyUTF16(buf[:], "abcdefgh")
	if buf[4] != 0 || buf[0] != 'a' {
		t.Errorf("buffer = %v", buf)
	}
	copyUTF16(buf[:], "a\x00b")
	if buf[0] != 'a' || buf[1] != 'b' || buf[2] != 0 {
		t.Errorf("NUL not dropped: %v", buf)
	}
}

// TestShowForReal puts an icon in the tray and shows a toast. It needs a
// desktop session, so it only runs with UNCLI_NOTIFY_LIVE=1.
func TestShowForReal(t *testing.T) {
	if os.Getenv("UNCLI_NOTIFY_LIVE") != "1" {
		t.Skip("set UNCLI_NOTIFY_LIVE=1 to show a real notification")
	}
	n := New("UNCLI test", func(key string) { t.Logf("clicked %q", key) })
	defer n.Close()
	if err := n.Show(Note{Key: "s1", Title: "UNCLI test", Body: "Finished: this is a test notification."}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(3 * time.Second)
}
