package bridge

import "testing"

// One UNCLI per data folder: the user's app keeps the plain id, and a copy
// on a folder of its own gets an id of its own, the same for the same folder.
func TestInstanceID(t *testing.T) {
	if got := instanceID(""); got != "app.uncli.desktop" {
		t.Errorf("default = %q", got)
	}
	a, b := instanceID(`C:\Temp\uncli-e2e\data`), instanceID(`c:\temp\uncli-e2e\data\`)
	if a == "app.uncli.desktop" || a != b {
		t.Errorf("a data folder: %q, %q", a, b)
	}
	if a == instanceID(`C:\Temp\other`) {
		t.Error("two folders share an id")
	}
}
