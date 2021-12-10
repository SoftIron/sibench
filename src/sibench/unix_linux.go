package main

import "syscall"

func Open(path string, mode int, perm uint32) (FileDescriptor, error) {
	fd, err := syscall.Open(path, mode|syscall.O_DIRECT|syscall.O_SYNC, perm)

	return FileDescriptor(fd), err
}

func Mount(source string, target string, fstype string, flags uintptr, data string) error {
	return syscall.Mount(source, target, fstype, flags, data)
}
