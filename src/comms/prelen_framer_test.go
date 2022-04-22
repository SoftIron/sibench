// SPDX-FileCopyrightText: 2022 SoftIron Limited <info@softiron.com>
// SPDX-License-Identifier: GNU General Public License v2.0 only WITH Classpath exception 2.0

// Tests for pre length framing protocol.

package comms

import "testing"
import "silib/testutil"


// Test functions.

// Encode a small frame.
func TestPrelenFramerEncodeSmall(t *testing.T) {
    payload := []byte{4, 5}
    expected := []byte{2, 0, 0, 0, 4, 5}

    conn := makeTestByteConn(nil)
    framer := makePreLengthFramer(conn)

    err := framer.Send(payload)

    testutil.CheckNoError(t, err)
    testutil.CheckBool(t, false, conn.ReadCalled())
    testutil.CheckBytes(t, expected, conn.WriteBytes())
}


// Encode a larger frame.
func TestPrelenFramerEncodeLarge(t *testing.T) {
    payload := []byte{
        0x45, 0x00, 0x00, 0x73, 0x00, 0x00, 0x40, 0x00,
        0x40, 0x11, 0x00, 0x00, 0xc0, 0xa8, 0x00, 0x01,
        0xc0, 0xa8, 0x00, 0xc7,
    }
    expected := []byte{
        0x14, 0x00, 0x00, 0x00,
        0x45, 0x00, 0x00, 0x73, 0x00, 0x00, 0x40, 0x00,
        0x40, 0x11, 0x00, 0x00, 0xc0, 0xa8, 0x00, 0x01,
        0xc0, 0xa8, 0x00, 0xc7,
    }
    conn := makeTestByteConn(nil)
    framer := makePreLengthFramer(conn)

    err := framer.Send(payload)

    testutil.CheckNoError(t, err)
    testutil.CheckBool(t, false, conn.ReadCalled())
    testutil.CheckBytes(t, expected, conn.WriteBytes())
}


// Decode a small message.
func TestPrelenFramerDecodeSmall(t *testing.T) {
    readBytes := []byte{3, 0, 0, 0, 4, 5, 6}
    expected := []byte{4, 5, 6}

    conn := makeTestByteConn(readBytes)
    framer := makePreLengthFramer(conn)

    message, err := framer.Receive()

    testutil.CheckNoError(t, err)
    testutil.CheckBool(t, false, conn.WriteCalled())
    testutil.CheckBytes(t, expected, message)
    testutil.CheckInt(t, 0, conn.UnreadByteCount())
}


// Decode a message spanning 2 reads.
func TestPrelenFramerDecodeSplit(t *testing.T) {
    // Our test connection only returns 8 at a time.
    readBytes := []byte{10, 0, 0, 0, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}
    expected := []byte{10, 11, 12, 13, 14, 15, 16, 17, 18, 19}

    conn := makeTestByteConn(readBytes)
    framer := makePreLengthFramer(conn)

    message, err := framer.Receive()

    testutil.CheckNoError(t, err)
    testutil.CheckBool(t, false, conn.WriteCalled())
    testutil.CheckBytes(t, expected, message)
    testutil.CheckInt(t, 0, conn.UnreadByteCount())
}


// Decode a message with data left over.
func TestPrelenFramerDecodeExcessData(t *testing.T) {
    readBytes := []byte{3, 0, 0, 0, 4, 5, 6, 7, 8}
    expected := []byte{4, 5, 6}

    conn := makeTestByteConn(readBytes)
    framer := makePreLengthFramer(conn)

    message, err := framer.Receive()

    testutil.CheckNoError(t, err)
    testutil.CheckBool(t, false, conn.WriteCalled())
    testutil.CheckBytes(t, expected, message)
    testutil.CheckInt(t, 2, conn.UnreadByteCount())
}


// Decode 2 message from a single stream.
func TestPrelenFramerDecode2(t *testing.T) {
    readBytes := []byte{3, 0, 0, 0, 4, 5, 6, 2, 0, 0, 0, 7, 8}
    expected1 := []byte{4, 5, 6}
    expected2 := []byte{7, 8}

    conn := makeTestByteConn(readBytes)
    framer := makePreLengthFramer(conn)

    message1, err1 := framer.Receive()
    message2, err2 := framer.Receive()

    testutil.CheckNoError(t, err1)
    testutil.CheckNoError(t, err2)
    testutil.CheckBool(t, false, conn.WriteCalled())
    testutil.CheckBytes(t, expected1, message1)
    testutil.CheckBytes(t, expected2, message2)
    testutil.CheckInt(t, 0, conn.UnreadByteCount())
}


// Worker type.

// makeTestByteConn - Make a test byte connection claiming to have received the given data.
func makeTestByteConn(received []byte) *testByteConn {
    var t testByteConn
    t.readBytes = received
    return &t
}


// Read - Supply fake data. Never supplies more than 8 bytes at a time.
func (me *testByteConn) Read(buffer []byte) (byteCount int, err error) {
    me.readCalled = true
    length := len(buffer)
    if length > 8 { length = 8 }
    if length > len(me.readBytes) { length = len(me.readBytes) }

    // Hand out the first length bytes and keep the rest.
    copy(buffer, me.readBytes[0:length])
    me.readBytes = me.readBytes[length:]

    return length, nil
}


// ReadCalled - Report whether read was called.
func (me *testByteConn) ReadCalled() bool {
    return me.readCalled
}


// UnreadByteCount -  Report the number of bytes that remained unread, if any.
func (me *testByteConn) UnreadByteCount() int {
    return len(me.readBytes)
}


// Write writes data to the connection.
func (me *testByteConn) Write(buffer []byte) (byteCount int, err error) {
    me.writeCalled = true

    // Just save the given data.
    me.writtenBytes = append(me.writtenBytes, buffer...)

    return len(buffer), nil
}


// WriteCalled - Report whether write was called.
func (me *testByteConn) WriteCalled() bool {
    return me.writeCalled
}


// WriteBytes - Report the bytes that were written to the connection.
func (me *testByteConn) WriteBytes() []byte {
    return me.writtenBytes
}


// testByteConn - Byte connection for testing that reports preset fake receive data and records sent data.
type testByteConn struct {
    writeCalled bool
    writtenBytes []byte
    readCalled bool
    readBytes []byte
}

