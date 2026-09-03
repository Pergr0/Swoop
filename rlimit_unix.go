//go:build unix

package main

import (
	"syscall"
)

// raiseFileLimit raises the soft RLIMIT_NOFILE toward the hard cap.
// Finder-launched macOS apps often start with a soft limit of 256; opening
// every file in a folder for transfer then exhausts FDs and can kill WebKit.
func raiseFileLimit() {
	var lim syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &lim); err != nil {
		return
	}
	want := lim.Max
	const target = 10240
	if want > target {
		want = target
	}
	if lim.Cur >= want {
		return
	}
	lim.Cur = want
	_ = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &lim)
}
