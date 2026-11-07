package session

import "slices"

// WhyMounts: a container setup step mounts a drive or folder, which could
// reach past the container's lockdown (decision 0028).
const WhyMounts = "mounts"

// A container's build installs software, needs admin rights and changes
// files all over its own disk: that's what it's for, so those reasons say
// nothing about a setup step (decision 0028).
var buildReasons = []string{WhyInstalls, WhySystem, WhyOutside, WhyDeletes, WhyDiscards, WhyStops}

// SetupStepRisks is why UNCLI's risk rules would stop to ask about a
// container's setup step, less what every build does: sending data away,
// naming secrets, running code fetched from elsewhere, mounting drives.
// ok is false when the step can't be read, which the caller flags too.
func SetupStepRisks(step string) (whys []string, ok bool) {
	parts, ok := workingParts(step, DialectBash, "/")
	if !ok {
		return nil, false
	}
	add := func(r Risk) {
		if r.Level == RiskRisky && !slices.Contains(buildReasons, r.Why) && !slices.Contains(whys, r.Why) {
			whys = append(whys, r.Why)
		}
	}
	for _, p := range parts {
		prog, ctx := partRisk(p, place{workdir: "/"})
		add(prog)
		add(ctx)
		if p.words[0] == "mount" && !slices.Contains(whys, WhyMounts) {
			whys = append(whys, WhyMounts)
		}
	}
	return whys, true
}
