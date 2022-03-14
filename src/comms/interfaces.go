/* Interfaces used by MessageConnections.

See tcp_connection.go for details.

*/

package comms


// ReceivedMessage - A message that we have received and partially decoded.
type ReceivedMessage interface {
    // ID - Report our message ID.
    ID() uint8

    // Data - Unpack the message data into the given struct of the appropriate type.
    Data(data interface{})
}


// EncoderFactory - Makes an encoder, including its framer and/or any other required objects.
type EncoderFactory interface {
    // Make - Make a new encoder that sits on top of the given byte connection.
    Make(connection ByteConnection) Encoder
}


// Encoder - Encodes and decodes messages with struct data, sending and receiving via a framer.
type Encoder interface {
    // Send - Encode the given message and send it.
    Send(messageID uint8, data interface{}) error

    // Receive - Blocking call to receive, and decode, the next message.
    Receive() (ReceivedMessage, error)
}


// Framer - Frames and unframes messages to be sent and received over a stream.
type Framer interface {
    // Send - Send the given message.
    Send(message []byte) error

    // Receive - Blocking call to receive the next message.
    Receive() (message []byte, err error)
}


// ByteConnection - Provides a byte oriented read/write stream.
// Note that net.Conn implements this interface.
type ByteConnection interface {
    // Read - Reads data from the connection.
    Read(buffer []byte) (byteCount int, err error)

    // Write - Writes data to the connection.
    Write(buffer []byte) (byteCount int, err error)
}

