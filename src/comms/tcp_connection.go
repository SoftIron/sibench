/* MessageConnection.

A MessageConnection encapsulates a connection over TCP to some other machine, over which messages can be sent and
received.

Messages consist of:
1. A string ID to identify the type of the message.
2. An optional struct of data.

The type of the data struct is implied by the message ID.

Message connections can be created either by listening on a TCP port or by connecting to another machine that is
listening. The resulting MessageConnection is identifcal in both cases.

When a message connection is now longer needed, Close() must be called on it.

Each message connection uses an encoder object to encode and decode messages, which in turn uses a framer to break up
the TCP byte stream. The encoder and framer are both created by an ecoder factory when the message connection is
created.

Once a message connection is established, messages can be received in either of 2 ways:

1. By providing a channel to which received messages are sent.
   This is expected to be used for server type code, waiting for any random message to come in.

2. By calling the blocking Receive() method.
   This is expected this to be used for client type code, sending messages and expecting a response.

It is not possible to use both of these on a single connection. Once a receive channel has been providing, Receive()
must not be called.

Whichever method is used for receiving, messages are sent with the Send() method.

*/

package comms

import "fmt"
import "io"
import "net"
import "time"

// TCPMessageFmt - Format of TCP messages.
type TCPMessageFmt struct {
    ID string `json:"command"`
    IsError bool `json:"is_error,omitempty"`
    Data interface{} `json:"data"`
}


// External API.

// MakeEncoderFactory - Make a factory for our default encoder.
func MakeEncoderFactory() EncoderFactory {
    return MakeJSONEncoderFactory()
}


// ListenTCP - Listen on the specified TCP port. New connections are reported via the given channel.
// New connections are created by the given factory.
func ListenTCP(address string, encoders EncoderFactory, notify chan<- *MessageConnection) (*Listener, error) {
    listener, err := net.Listen("tcp", address)
    if err != nil { return nil, err }    // Propogate error.

    fmt.Printf("Listening for TCP on %s\n", address)

    // Kick off background Goroutine to wait for accepts.
    go acceptTCP(listener, encoders, notify)

    // Wrap our TCP connection in a Listener, so we can hand it back to the caller.
    l := Listener{listener: listener}
    return &l, nil
}


// ListenTCPAll - Listen on the specified TCP port on any local address.
// All arguemnts other than port are as for ListenTCP.
func ListenTCPAll(port uint16, encoders EncoderFactory, notify chan<- *MessageConnection) (*Listener, error) {
    address := fmt.Sprintf(":%d", port)
    /*listener, err :=*/ return ListenTCP(address, encoders, notify)
}


// StopListening - Stop listening on our port and accepting new connections.
func (me *Listener) StopListening() {
    me.listener.Close()
}


// Listener - Handle to a listening connection.
type Listener struct {
    listener net.Listener
}


// ConnectTCP - Open a TCP message connection to the given address.
// The timeout is optional, pass to 0 for no timeout.
func ConnectTCP(address string, encoder EncoderFactory, timeout time.Duration) (*MessageConnection, error) {
    var dialer net.Dialer
    if timeout != 0 {
        dialer.Timeout = timeout
    }

    conn, err := dialer.Dial("tcp", address)

    if err != nil {
        return nil, fmt.Errorf("Failure to connect to %s, %v", address, err)
    }

    // We have a TCP connection, wrap it up in a MessageConnection.
    return makeMessageConn(conn, encoder), nil
}


// Close - Close this connection.
func (me *MessageConnection) Close() {
    // Tell our underlying connection to close.
    me.conn.Close()

    // If we have a receive channel, send nil to it.
    if me.rxChannel != nil {
        me.rxChannel<- nil
    }
}


// RemoteIP - Report the address of the machine at the other end of this connection, in IP:port form.
func (me *MessageConnection) RemoteIP() string {
    return me.conn.RemoteAddr().String()
}


// Send - Send the given message.
func (me* MessageConnection) Send(MessageID string, data interface{}) error {
    return me.encoder.Send(MessageID, data)
}


// Receive - Receive a single message, blocking until one is available.
// May not be called after a receive channel has been provided.
func (me *MessageConnection) Receive(timeout time.Duration) (ReceivedMessage, error) {
    if me.rxChannel != nil {
        return nil, fmt.Errorf("Cannot call Receive() on a MessageConnection that has a receive channel")
    }

    // TODO: Handle timeout.

    return me.encoder.Receive()
}


// SendReceive - Send the given command and wait for a response.
// Equivalent to calling Send() and then Receive(), but simplifies error handling slightly.
// The timeout is optional, pass to 0 for no timeout.
// May not be called after a receive channel has been provided.
func (me *MessageConnection) SendReceive(MessageID string, data interface{}, timeout time.Duration) (
    replyID string, replyData interface{}, err error) {
    // TODO
    return "", nil, nil
}


// ReceiveToChannel - Start receiving messages in the background and send them to the given channel.
// Kicks off a Goroutine to handle receiving.
// Once this has been called, messages may not be received by calling Receive() or SendReceive().
func (me *MessageConnection) ReceiveToChannel(notify chan<- *ReceivedMessageInfo) {
    if me.rxChannel != nil {
        // Cannot set the receive channel twice, do nothing.
        return
    }

    me.rxChannel = notify

    // Kick off a Goroutine to receive messages.
    go me.processReceives()
}

// ReceivedMessageInfo - A trivial wrapper around a received message so we can send it via a channel.
type ReceivedMessageInfo struct {
    Message ReceivedMessage
    Connection *MessageConnection
    Error error // Will equal io.EOF when the connection is closed.
}


// MessageConnection - A message based connection.
type MessageConnection struct {
    conn net.Conn  // Underlying TCP connection.
    rxChannel chan<- *ReceivedMessageInfo
    encoder Encoder
}


// Internals.

// makeMessageConn - Make a message connection based on the given TCP connection.
func makeMessageConn(conn net.Conn, encoderFactory EncoderFactory) *MessageConnection {
    var mc MessageConnection
    mc.conn = conn
    mc.encoder = encoderFactory.Make(conn)
    return &mc
}



// acceptTCP - Accept TCP connections.
// Only returns when accepting fails.
// Should be called as a Goroutine.
func acceptTCP(listener net.Listener, encoders EncoderFactory, notify chan<- *MessageConnection) {
    // Keep going round indefinitely.
    for {
        // Listen for an incoming connection.
        conn, err := listener.Accept()
        if err != nil {
            // Something went wrong, close connection.
            fmt.Errorf("Error accepting: %v\n", err)
            return
        }

        notify<- makeMessageConn(conn, encoders)
    }
}


// processReceives - Process messages received on the given connection and send them via the given channel.
// Only returns on connection failure.
// Should be called as a Goroutine.
func (me *MessageConnection) processReceives() {
    for {
        // Try to get a packet.
        message, err := me.encoder.Receive()

        // TODO: Handle connection closing.
        // TODO: Should we exit on error?

        // Wrap up the message so we can put in on the channel.
        var info ReceivedMessageInfo
        info.Message = message
        info.Connection = me
        info.Error = err

        me.rxChannel<- &info

        if err != nil {
            // Something's gone wrong with the connection, give up and close it.
            if err != io.EOF {
                me.conn.Close()
            }

            return
        }
    }
}

