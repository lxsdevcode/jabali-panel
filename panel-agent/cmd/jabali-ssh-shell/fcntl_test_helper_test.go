package main

import "syscall"

// syscallFcntlGetFD reads the FD flags (F_GETFD) for the test.
func syscallFcntlGetFD(fd uintptr) (int, uintptr, syscall.Errno) {
	r, _, errno := syscall.Syscall(syscall.SYS_FCNTL, fd, syscall.F_GETFD, 0)
	return int(r), 0, errno
}
