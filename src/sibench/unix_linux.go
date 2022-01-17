package main

import "syscall"


func Open(path string, mode int, perm uint32) (FileDescriptor, error) {
	fd, err := syscall.Open(path, mode|syscall.O_DIRECT|syscall.O_SYNC, perm)

	return FileDescriptor(fd), err
}


func Mount(source string, target string, fstype string, flags uintptr, data string) error {
	return syscall.Mount(source, target, fstype, flags, data)
}


/*
 * Returns the number of bytes of physical memory in the system, or 0 if we are unable to determine it.
 */
func GetPhysicalMemorySize() uint64 {
	info := &syscall.Sysinfo_t{}
	err := syscall.Sysinfo(info)
	if err != nil {
		return 0
	}

	return info.Totalram
}

