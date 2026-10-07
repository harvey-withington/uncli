package secrets

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	advapi32   = windows.NewLazySystemDLL("advapi32.dll")
	credReadW  = advapi32.NewProc("CredReadW")
	credWriteW = advapi32.NewProc("CredWriteW")
	credDelete = advapi32.NewProc("CredDeleteW")
	credFree   = advapi32.NewProc("CredFree")
)

const (
	credTypeGeneric         = 1
	credPersistLocalMachine = 2
)

// credential is CREDENTIALW.
type credential struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        windows.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

func get(target string) (string, error) {
	t, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return "", err
	}
	var c *credential
	r, _, e := credReadW.Call(uintptr(unsafe.Pointer(t)), credTypeGeneric, 0, uintptr(unsafe.Pointer(&c)))
	if r == 0 {
		if errors.Is(e, windows.ERROR_NOT_FOUND) {
			return "", nil
		}
		return "", e
	}
	defer credFree.Call(uintptr(unsafe.Pointer(c)))
	if c.CredentialBlobSize == 0 || c.CredentialBlob == nil {
		return "", nil
	}
	return string(unsafe.Slice(c.CredentialBlob, c.CredentialBlobSize)), nil
}

func set(target, value string) error {
	t, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	user, _ := windows.UTF16PtrFromString("uncli")
	blob := []byte(value)
	c := credential{
		Type:               credTypeGeneric,
		TargetName:         t,
		CredentialBlobSize: uint32(len(blob)),
		CredentialBlob:     &blob[0],
		Persist:            credPersistLocalMachine,
		UserName:           user,
	}
	if r, _, e := credWriteW.Call(uintptr(unsafe.Pointer(&c)), 0); r == 0 {
		return e
	}
	return nil
}

func del(target string) error {
	t, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	if r, _, e := credDelete.Call(uintptr(unsafe.Pointer(t)), credTypeGeneric, 0); r == 0 && !errors.Is(e, windows.ERROR_NOT_FOUND) {
		return e
	}
	return nil
}
