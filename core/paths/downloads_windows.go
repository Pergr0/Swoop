//go:build windows

package paths

import "golang.org/x/sys/windows"

// FOLDERID_Downloads {374DE290-123F-4565-9164-39C4925E467B}
var folderIDDownloads = windows.KNOWNFOLDERID{
	Data1: 0x374DE290,
	Data2: 0x123F,
	Data3: 0x4565,
	Data4: [8]byte{0x91, 0x64, 0x39, 0xC4, 0x92, 0x5E, 0x46, 0x7B},
}

// Downloads queries the Windows "Downloads" known folder (which correctly
// follows a relocated/redirected folder), falling back to %USERPROFILE%\Downloads.
func Downloads() string {
	if p, err := windows.KnownFolderPath(&folderIDDownloads, 0); err == nil && p != "" {
		return p
	}
	return homeJoin("Downloads")
}
