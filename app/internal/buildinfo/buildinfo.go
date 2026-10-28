// Package buildinfo is what a build knows about itself: its version, which
// release builds stamp (-ldflags "-X uncli/internal/buildinfo.Version=0.4.0"),
// and the commit it was built from. Anything else is a development build.
package buildinfo

import "runtime/debug"

// Version is the release this build is ("0.4.0-alpha"), or "dev".
var Version = "dev"

// Dev says this is a development build: built by hand or by publish-build,
// not stamped by a release. Development builds don't check for updates.
func Dev() bool { return Version == "" || Version == "dev" }

// Commit is the commit the build came from and when it was made (RFC 3339);
// empty when Go didn't record them.
func Commit() (rev, date string) {
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				rev = s.Value
			case "vcs.time":
				date = s.Value
			}
		}
	}
	return rev, date
}
