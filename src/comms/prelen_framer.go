/* The pre length framer.

This is a simple framer for use in MessageConnections. It implements the Framer interface.

The framer prepends a length field onto messages. The length field is always 4 bytes and little endian.

*/

package comms

import "fmt"


// External API.

// makePreLengthFramer - Make a pre length framer that sits on top of the given byte connection.
func makePreLengthFramer(conn ByteConnection) Framer {
    var framer preLengthFramer
    framer.conn = conn
    return &framer
}


// Send - Send the given message.
func (me *preLengthFramer) Send(message []byte) error {
    // First build the header. This is simply a 4 byte, little endian, length field.
    messageLen := len(message)
    var header [4]byte
    header[0] = uint8(messageLen & 0xFF)
    header[1] = uint8((messageLen >> 8) & 0xFF)
    header[2] = uint8((messageLen >> 16) & 0xFF)
    header[3] = uint8((messageLen >> 24) & 0xFF)

    // Now we can send the header and the body.
    _, err := me.conn.Write(header[:])
    if err != nil { return err }  // Propogate error.

    _, err = me.conn.Write(message)
    if err != nil { return err }  // Propogate error.

    // And we're done.
    return nil
}


// Receive - Blocking call to receive the next message.
func (me *preLengthFramer) Receive() (message []byte, err error) {
    // First we need a message header, which is always 4 bytes.
    header, err := me.receiveBytes(4)
    if err != nil { return nil, err }  // Propogate error.

    messageLen := uint(header[0]) | (uint(header[1]) << 8) | (uint(header[2]) << 16) | (uint(header[3]) << 24)
    // TODO: Do we want to impose any limits on this length as a sanity check?

    // Now we can get the message body.
    message, err = me.receiveBytes(messageLen)
    if err != nil { return nil, err }  // Propogate error.

    // Just return the message body as is.
    return message, nil
}


// Internals.

// preLengthFramer - A framer that prefixes a 4 byte length field onto each message.
type preLengthFramer struct {
    conn ByteConnection
}


// receiveBytes - Receive exactly the specified number of bytes from our connection.
func (me *preLengthFramer) receiveBytes(byteCount uint) (data []byte, err error) {
    // We can ask the connection for the correct number of bytes, but it might not give us all of them at once. So we
    // need to keep trying until we have enough.
    // We allocate a buffer at the start of the right size, so we don't add any copying.
    buffer := make([]byte, byteCount)
    index := uint(0)
    remaining := byteCount

    for remaining > 0 {
        // Get send bytes.
        //fmt.Printf("Getting %d bytes\n", len(buffer[index:]))
        count, err := me.conn.Read(buffer[index:])
        //fmt.Printf("Got %d\n", count)
        if count < 0 { return nil, fmt.Errorf("TCP connection return <0 bytes (%d)", count) }
        if err != nil { return nil, err }  // Propogate error.

        index += uint(count)
        remaining -= uint(count)
    }

    // We've got all we need.
    return buffer, nil
}

