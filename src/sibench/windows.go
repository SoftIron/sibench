// SPDX-FileCopyrightText: 2022 SoftIron Limited <info@softiron.com>
// SPDX-License-Identifier: GNU General Public License v2.0 only WITH Classpath exception 2.0

// +build windows

package main

import "fmt"
import"runtime"
import "unsafe"
import "golang.org/x/sys/windows"


type FileDescriptor windows.Handle


func Open(path string, mode int, perm uint32) (FileDescriptor, error) {
	// Copy stdlib windows implementation of this just to add the
	// FILE_FLAG_WRITE_THROUGH flag. See windows.Open (syscall_windows.go) for original code.
	//
	// https://docs.microsoft.com/en-US/windows/win32/api/fileapi/nf-fileapi-createfilea

	if len(path) == 0 {
		return FileDescriptor(windows.InvalidHandle), windows.ERROR_FILE_NOT_FOUND
	}

	pathp, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return FileDescriptor(windows.InvalidHandle), err
	}

	var access uint32

	switch mode & (windows.O_RDONLY | windows.O_WRONLY | windows.O_RDWR) {
        case windows.O_RDONLY:  access = windows.GENERIC_READ
        case windows.O_WRONLY:  access = windows.GENERIC_WRITE
        case windows.O_RDWR:    access = windows.GENERIC_READ | windows.GENERIC_WRITE
	}

	if mode&windows.O_CREAT != 0 {
		access |= windows.GENERIC_WRITE
	}

	if mode&windows.O_APPEND != 0 {
		access &^= windows.GENERIC_WRITE
		access |= windows.FILE_APPEND_DATA
	}

	sharemode := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE)

	var sa windows.SecurityAttributes

	if mode&windows.O_CLOEXEC == 0 {
		sa.Length = uint32(unsafe.Sizeof(sa))
		sa.InheritHandle = 1
	}

	var createmode uint32

	switch {
        case mode&(windows.O_CREAT | windows.O_EXCL) == (windows.O_CREAT | windows.O_EXCL):
            createmode = windows.CREATE_NEW

        case mode&(windows.O_CREAT | windows.O_TRUNC) == (windows.O_CREAT | windows.O_TRUNC):
            createmode = windows.CREATE_ALWAYS

        case mode&windows.O_CREAT == windows.O_CREAT:
            createmode = windows.OPEN_ALWAYS

        case mode&windows.O_TRUNC == windows.O_TRUNC:
            createmode = windows.TRUNCATE_EXISTING

        default:
            createmode = windows.OPEN_EXISTING
	}

	var attrs uint32 = windows.FILE_ATTRIBUTE_NORMAL

	if perm&windows.S_IWRITE == 0 {
		attrs = windows.FILE_ATTRIBUTE_READONLY
	}

	// mix in the O_SYNC like 0x80000000
	fd, err := windows.CreateFile(pathp, access, sharemode, &sa, createmode, attrs|windows.FILE_FLAG_WRITE_THROUGH, 0)

	return FileDescriptor(fd), err
}


func (fd FileDescriptor) Size() (int64, error) {
	var stat windows.ByHandleFileInformation

	err := windows.GetFileInformationByHandle(windows.Handle(fd), &stat)
	if err != nil {
		return 0, err
	}

	return int64(stat.FileSizeHigh)<<32 | int64(stat.FileSizeLow), nil
}


func (fd FileDescriptor) Seek(offset int64, whence int) (int64, error) {
	return windows.Seek(windows.Handle(fd), offset, whence)
}


func (fd FileDescriptor) Read(p []byte) (int, error) {
	return windows.Read(windows.Handle(fd), p)
}


func (fd FileDescriptor) Pread(p []byte, offset int64) (int, error) {
	var o windows.Overlapped

	o.OffsetHigh = uint32(offset >> 32)
	o.Offset = uint32(offset)
	n := uint32(len(p))

	err := windows.ReadFile(windows.Handle(fd), p, &n, &o)

	return int(n), err
}


func (fd FileDescriptor) Write(p []byte) (int, error) {
	return windows.Write(windows.Handle(fd), p)
}


func (fd FileDescriptor) Pwrite(p []byte, offset int64) (int, error) {
	var o windows.Overlapped
	var done uint32

	o.OffsetHigh = uint32(offset >> 32)
	o.Offset = uint32(offset)

	err := windows.WriteFile(windows.Handle(fd), p, &done, &o)

	return int(done), err
}


func (fd FileDescriptor) Close() error {
	return windows.Close(windows.Handle(fd))
}


func Mount(source string, target string, fstype string, flags uintptr, data string) error {
	return fmt.Errorf("Mount not implemented on %q", runtime.GOOS)
}


func Unmount(path string, flags int) error {
	return fmt.Errorf("Unmount not implemented on %q", runtime.GOOS)
}


func NewRadosConnection(target string, protocol ProtocolConfig, worker WorkerConnectionConfig) (Connection, error) {
	return nil, fmt.Errorf("rados not implemented on %q", runtime.GOOS)
}


func NewRbdConnection(target string, protocol ProtocolConfig, worker WorkerConnectionConfig) (Connection, error) {
	return nil, fmt.Errorf("rbd not implemented on %q", runtime.GOOS)
}


/*
 * Returns the number of bytes of physical memory in the system, or 0 if we are unable to determine it.
 */
func GetPhysicalMemorySize() uint64 {
    // XXX Need to work this out on Windows!
    return 0
}

