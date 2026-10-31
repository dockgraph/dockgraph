//go:build !linux

package collector

import "syscall"

// composeChildAttr returns nil: the parent-death signal the Linux version
// sets has no counterpart here.
func composeChildAttr() *syscall.SysProcAttr {
	return nil
}

// restrictComposeParse sets no limits outside Linux: the child is only
// stopped at its parent's deadline.
func restrictComposeParse([]string) string {
	return "no memory or file limits on this system; stopped after " + composeParseTimeout.String()
}
