// +build darwin linux

package main

import (
	"syscall"
)

type FileDescriptor int

func (fd FileDescriptor) Seek(offset int64, whence int) (int64, error) {
	return syscall.Seek(int(fd), offset, whence)
}

func (fd FileDescriptor) Size() (int64, error) {
	var stat syscall.Stat_t

	if err := syscall.Fstat(int(fd), &stat); err != nil {
		return 0, err
	}

	return stat.Size, nil
}

func (fd FileDescriptor) Read(p []byte) (int, error) {
	return syscall.Read(int(fd), p)
}

func (fd FileDescriptor) Pread(p []byte, offset int64) (int, error) {
	return syscall.Pread(int(fd), p, offset)
}

func (fd FileDescriptor) Write(p []byte) (int, error) {
	return syscall.Write(int(fd), p)
}

func (fd FileDescriptor) Pwrite(p []byte, offset int64) (int, error) {
	return syscall.Pwrite(int(fd), p, offset)
}

func (fd FileDescriptor) Close() error {
	if fd != 0 {
		return syscall.Close(int(fd))
	}

	return nil
}

func Unmount(path string, flags int) error {
	return syscall.Unmount(path, flags)
}
