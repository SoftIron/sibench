/*
SPDX-FileCopyrightText: 2014 Noah Watkins
SPDX-FileCopyrightText: 2022 SoftIron Limited
SPDX-License-Identifier: MIT

The following code is a modified fork of the go-ceph package, licensed under the MIT license
and available from https://github.com/ceph/go-ceph/

The MIT License (MIT)

Copyright (c) 2014 Noah Watkins

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package rbd

// #cgo LDFLAGS: -lrbd
// /* force XSI-complaint strerror_r() */
// #define _POSIX_C_SOURCE 200112L
// #undef _GNU_SOURCE
// #include <errno.h>
// #include <stdlib.h>
// #include <rados/librados.h>
// #include <rbd/librbd.h>
import "C"

import "io"
import "unsafe"



const (
    // fail a create operation if the object already exists
    LIBRADOS_OP_FLAG_EXCL int = 1 << iota

    // allow the transaction to succeed even if the flagged op fails
    LIBRADOS_OP_FLAG_FAILOK

    // indicate read/write op random
    LIBRADOS_OP_FLAG_FADVISE_RANDOM

    // indicate read/write op sequential
    LIBRADOS_OP_FLAG_FADVISE_SEQUENTIAL

    // indicate read/write data will be accessed in the near future (by someone)
    LIBRADOS_OP_FLAG_FADVISE_WILLNEED

    // indicate read/write data will not accessed in the near future (by anyone)
    LIBRADOS_OP_FLAG_FADVISE_DONTNEED

    // indicate read/write data will not accessed again (by *this* client)
    LIBRADOS_OP_FLAG_FADVISE_NOCACHE

    // optionally support FUA (force unit access) on write requests
    LIBRADOS_OP_FLAG_FADVISE_FUA
)


/*
 * Ceph's go bindings aren't complete.  In particular we are missing the rbd_read2 function, which
 * is the version of read that allows us to pass flags in.  This is needed by sibench as one of the 
 * flags tells it to not use cache.
 */
func (image *Image) Read2(data []byte, op_flags int) (int, error) {
	if err := image.validate(imageIsOpen); err != nil {
		return 0, err
	}

	if len(data) == 0 {
		return 0, nil
	}

	ret := int(C.rbd_read2(
		image.image,
		(C.uint64_t) (image.offset),
		(C.size_t) (len(data)),
		(*C.char) (unsafe.Pointer(&data[0])),
        (C.int) (op_flags)))

	if ret < 0 {
		return 0, rbdError(ret)
	}

	image.offset += int64(ret)
	if ret < len(data) {
		return ret, io.EOF
	}

	return ret, nil
}


func (image *Image) InvalidateCache() error {
	ret := int(C.rbd_invalidate_cache(image.image))
	if ret < 0 {
		return rbdError(ret)
	}

    return nil
}
