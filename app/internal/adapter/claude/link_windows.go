package claude

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

// link makes a directory junction at path pointing to target. Unlike a
// symbolic link, a junction needs no administrator rights or Developer
// Mode.
func link(target, path string) error {
	target, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		return err
	}
	if err := setJunction(path, target); err != nil {
		_ = os.Remove(path)
		return &os.LinkError{Op: "junction", Old: target, New: path, Err: err}
	}
	return nil
}

// setJunction writes a mount-point reparse point (REPARSE_DATA_BUFFER) on
// the empty folder at path.
func setJunction(path, target string) error {
	sub := utf16.Encode([]rune(`\??\` + target))
	printName := utf16.Encode([]rune(target))
	names := make([]uint16, 0, len(sub)+len(printName)+2)
	names = append(append(append(append(names, sub...), 0), printName...), 0)
	const header = 8 // offsets and lengths of the two names
	dataLen := header + 2*len(names)
	buf := make([]byte, 8+dataLen)
	binary.LittleEndian.PutUint32(buf[0:], windows.IO_REPARSE_TAG_MOUNT_POINT)
	binary.LittleEndian.PutUint16(buf[4:], uint16(dataLen))
	binary.LittleEndian.PutUint16(buf[8:], 0)                         // substitute name offset
	binary.LittleEndian.PutUint16(buf[10:], uint16(2*len(sub)))       // and length
	binary.LittleEndian.PutUint16(buf[12:], uint16(2*(len(sub)+1)))   // print name offset
	binary.LittleEndian.PutUint16(buf[14:], uint16(2*len(printName))) // and length
	for i, c := range names {
		binary.LittleEndian.PutUint16(buf[16+2*i:], c)
	}
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	h, err := windows.CreateFile(p, windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	var n uint32
	return windows.DeviceIoControl(h, windows.FSCTL_SET_REPARSE_POINT, &buf[0], uint32(len(buf)), nil, 0, &n, nil)
}
