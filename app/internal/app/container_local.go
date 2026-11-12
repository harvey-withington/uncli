package app

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// A container profile can list local paths (Local): paths in a session's
// folder, such as node_modules, that the container keeps on its own disk,
// so the container's installs and Windows' don't overwrite each other
// (decision 0032). They stay inside the container, so they don't count as
// reaching outside it (0028).

// maxLocalPath is the longest local path a profile may give.
const maxLocalPath = 200

// localPath is a local path as it's mounted: relative to the project, with
// forward slashes and no ./ or trailing slash. It refuses what would leave
// the project or can't be a Windows folder, and .git.
func localPath(s string) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "", errors.New("is empty")
	case len(s) > maxLocalPath:
		return "", errors.New("is too long")
	case strings.HasPrefix(s, "/") || strings.HasPrefix(s, `\`) || len(s) >= 2 && s[1] == ':':
		return "", errors.New("must be relative to the project, not a full path")
	case strings.ContainsFunc(s, unicode.IsControl) || strings.ContainsAny(s, `<>:"|?*`):
		return "", errors.New("has a character a Windows folder can't")
	}
	var segs []string
	for _, seg := range strings.Split(strings.ReplaceAll(s, `\`, "/"), "/") {
		switch {
		case seg == "" || seg == ".":
			continue
		case seg == "..":
			return "", errors.New("can't go up out of the project (..)")
		case strings.EqualFold(seg, ".git"):
			return "", errors.New("can't be .git or in it")
		case strings.HasSuffix(seg, ".") || strings.HasSuffix(seg, " ") || strings.HasPrefix(seg, " "):
			return "", errors.New("has a name Windows can't keep (one starting with a space, or ending with a dot or a space)")
		}
		segs = append(segs, seg)
	}
	if len(segs) == 0 {
		return "", errors.New("is the project itself")
	}
	return strings.Join(segs, "/"), nil
}

// validLocal checks a profile's local paths (validProfile), naming the
// one that's wrong: each must be a path in the project, once, and not in
// another (Windows' folders don't tell case apart, so neither does this).
func validLocal(id string, paths []string) error {
	var clean []string
	for _, s := range paths {
		p, err := localPath(s)
		if err != nil {
			return fmt.Errorf("container %q: local path %q %s", id, s, err)
		}
		for i, q := range clean {
			lp, lq := strings.ToLower(p), strings.ToLower(q)
			switch {
			case lp == lq:
				return fmt.Errorf("container %q: local path %q is listed twice", id, s)
			case strings.HasPrefix(lp, lq+"/"):
				return fmt.Errorf("container %q: local path %q is inside %q, which is local already", id, s, paths[i])
			case strings.HasPrefix(lq, lp+"/"):
				return fmt.Errorf("container %q: local path %q holds %q, which is listed too", id, s, paths[i])
			}
		}
		clean = append(clean, p)
	}
	return nil
}

// localPaths are a profile's local paths as they're mounted. A profile is
// checked when it's read, so a wrong one isn't expected; it's left out.
func localPaths(paths []string) []string {
	out := []string{}
	for _, s := range paths {
		if p, err := localPath(s); err == nil && validLocal("", append(out[:len(out):len(out)], p)) == nil {
			out = append(out, p)
		}
	}
	return out
}

// tidyLocal is a profile's local paths as they're saved: each cleaned when
// it can be (one that can't stays, for validLocal to name), none as nil, so
// a profile without any is as it was before they existed.
func tidyLocal(paths []string) []string {
	var out []string
	for _, s := range paths {
		if p, err := localPath(s); err == nil {
			out = append(out, p)
		} else {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}
