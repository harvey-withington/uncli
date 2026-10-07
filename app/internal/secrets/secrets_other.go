//go:build !windows

package secrets

func get(string) (string, error) { return "", ErrUnsupported }
func set(string, string) error   { return ErrUnsupported }
func del(string) error           { return ErrUnsupported }
