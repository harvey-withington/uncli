package artifacts

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPutStoresOnceAndReadsBack(t *testing.T) {
	dir := t.TempDir()
	st := NewStore(filepath.Join(dir, "store"))
	a := filepath.Join(dir, "a.html")
	b := filepath.Join(dir, "b.html")
	_ = os.WriteFile(a, []byte("<h1>hi</h1>"), 0o644)
	_ = os.WriteFile(b, []byte("<h1>hi</h1>"), 0o644)
	h1, n, err := st.Put(a)
	if err != nil || n != 11 || len(h1) != 64 {
		t.Fatalf("put = %q %d %v", h1, n, err)
	}
	h2, _, _ := st.Put(b)
	if h1 != h2 {
		t.Error("the same content got two hashes")
	}
	// A later change is a new version; the old one stays readable.
	_ = os.WriteFile(a, []byte("<h1>bye</h1>"), 0o644)
	h3, _, _ := st.Put(a)
	if h3 == h1 {
		t.Error("changed content kept its hash")
	}
	if got, err := st.Read(h1, 1<<20); err != nil || string(got) != "<h1>hi</h1>" {
		t.Errorf("read old = %q %v", got, err)
	}
	if _, err := st.Read("../../etc/passwd", 1<<20); err == nil {
		t.Error("read outside the store")
	}
	if _, err := st.Read(h1, 4); !errors.Is(err, ErrTooBig) {
		t.Errorf("cap: %v", err)
	}
	left, _ := filepath.Glob(filepath.Join(dir, "store", "put-*"))
	if len(left) != 0 {
		t.Errorf("temp files left: %v", left)
	}
}

func TestListAndResolve(t *testing.T) {
	root := filepath.Join(t.TempDir(), Folder)
	if files, err := List(root); err != nil || len(files) != 0 {
		t.Fatalf("missing folder: %v %v", files, err)
	}
	_ = os.MkdirAll(filepath.Join(root, "charts"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "report.html"), []byte("x"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "charts", "sales.svg"), []byte("yy"), 0o644)
	files, err := List(root)
	if err != nil || len(files) != 2 || files[0] != (File{Path: "charts/sales.svg", Size: 2}) {
		t.Fatalf("list = %v %v", files, err)
	}
	if got, err := ReadLive(root, "charts/sales.svg", 10); err != nil || string(got) != "yy" {
		t.Errorf("read = %q %v", got, err)
	}
	outside := filepath.Join(filepath.Dir(root), "secret.txt")
	_ = os.WriteFile(outside, []byte("no"), 0o644)
	for _, bad := range []string{"../secret.txt", "charts/../../secret.txt", outside, ""} {
		if _, err := ReadLive(root, bad, 10); err == nil {
			t.Errorf("read %q outside the folder", bad)
		}
	}
}
