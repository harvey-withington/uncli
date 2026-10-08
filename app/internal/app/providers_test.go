package app

import (
	"io/fs"
	"testing"

	"uncli/config"
)

func TestProvidersHaveNames(t *testing.T) {
	defaults, _ := fs.Sub(config.Defaults, "defaults")
	ps, err := loadProviders(defaults)
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) == 0 || ps[0].ID != "claude" || ps[0].Name == "" || ps[0].Agent == "" || ps[0].Label != ps[0].Name {
		t.Errorf("providers = %+v", ps)
	}
}
