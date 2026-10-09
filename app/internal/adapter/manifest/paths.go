package manifest

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// get follows a dotted path into decoded JSON; nil when it isn't there.
// A number in the path indexes a list.
func get(v any, path string) any {
	if path == "" {
		return nil
	}
	for _, k := range strings.Split(path, ".") {
		switch t := v.(type) {
		case map[string]any:
			v = t[k]
		case []any:
			i, err := strconv.Atoi(k)
			if err != nil || i < 0 || i >= len(t) {
				return nil
			}
			v = t[i]
		default:
			return nil
		}
	}
	return v
}

// text is a value as text: numbers without a needless fraction.
func text(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		if t == math.Trunc(t) && math.Abs(t) < 1e15 {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	case map[string]any, []any:
		b, _ := json.Marshal(t)
		return string(b)
	}
	return fmt.Sprint(v)
}

func str(v any, path string) string { s, _ := get(v, path).(string); return s }

// first is the first path that holds a non-empty string.
func first(v any, paths Paths) string {
	for _, p := range paths {
		if s := str(v, p); s != "" {
			return s
		}
	}
	return ""
}

func num(v any, path string) float64 {
	f, _ := get(v, path).(float64)
	return f
}

func sum(v any, paths Paths) int {
	n := 0
	for _, p := range paths {
		n += int(num(v, p))
	}
	return n
}

func has(v any, paths Paths) bool {
	for _, p := range paths {
		if get(v, p) != nil {
			return true
		}
	}
	return false
}

// matches: every path holds the value given, or one of the values listed.
// A condition that isn't set holds for nothing.
func matches(v any, cond map[string]any) bool {
	if len(cond) == 0 {
		return false
	}
	for path, want := range cond {
		got := get(v, path)
		if got == nil {
			return false
		}
		g := text(got)
		ok := false
		if list, isList := want.([]any); isList {
			for _, w := range list {
				ok = ok || g == fmt.Sprint(w)
			}
		} else {
			ok = g == fmt.Sprint(want)
		}
		if !ok {
			return false
		}
	}
	return true
}

var placeholder = regexp.MustCompile(`\{([A-Za-z0-9_.:-]+)\}`)

// expand fills {name} placeholders; lookup reports whether it knows one.
// Unknown placeholders are left as they are.
func expand(s string, lookup func(name string) (string, bool)) string {
	return placeholder.ReplaceAllStringFunc(s, func(m string) string {
		if v, ok := lookup(m[1 : len(m)-1]); ok {
			return v
		}
		return m
	})
}

// fill fills the placeholders in every string of a YAML value (a template
// for JSON), keeping its shape.
func fill(v any, lookup func(name string) (string, bool)) any {
	switch t := v.(type) {
	case string:
		return expand(t, lookup)
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, x := range t {
			out[k] = fill(x, lookup)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, x := range t {
			out[i] = fill(x, lookup)
		}
		return out
	}
	return v
}

// vars looks names up in a map, then (dotted paths) in a JSON value.
func vars(m map[string]string, line any) func(string) (string, bool) {
	return func(name string) (string, bool) {
		if v, ok := m[name]; ok {
			return v, true
		}
		if line != nil {
			if v := get(line, name); v != nil {
				return text(v), true
			}
		}
		return "", false
	}
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i] + " …"
	}
	if r := []rune(s); len(r) > 200 {
		s = string(r[:199]) + "…"
	}
	return s
}

func truncate(s string) string {
	const max = 4000
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}
