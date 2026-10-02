// Package clipfiles reads the files on the OS clipboard: what the file
// manager puts there when files are copied (Explorer's Ctrl+C). A web view
// only sees their names as text, so pasting them as paths needs the OS list.
package clipfiles

// Paths returns the full paths of the files on the clipboard, or none when
// the clipboard holds no files (or the OS isn't supported yet).
func Paths() ([]string, error) { return paths() }
