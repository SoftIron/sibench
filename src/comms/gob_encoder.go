// SPDX-FileCopyrightText: 2022 SoftIron Limited <info@softiron.com>
// SPDX-License-Identifier: GNU General Public License v2.0 only WITH Classpath exception 2.0

/* The Gob encoder.

This is an encoder for use in MessageConnections. It implements the Encoder interface.

The encoder uses the gpb stuff from the Go standard library
*/

package comms

import "bytes"
import "encoding/gob"
import "fmt"


// Encoder Factory external API.

// MakeGobEncoderFactory - Make a Gob encoder factory.
func MakeGobEncoderFactory() EncoderFactory {
    var factory gobEncoderFactory
    return &factory
}


// Make - Make a new Gob encoder that sits on top of the given byte connection.
func (me *gobEncoderFactory) Make(connection ByteConnection) Encoder {
    framer := makePreLengthFramer(connection)
    encoder := makeGobEncoder(framer)
    return encoder
}


// Encoder external API.

// Send - Encode the given message and send it.
func (me *gobEncoder) Send(messageID uint8, data interface{}) error {
    // First build the packet to send.
    var buf bytes.Buffer
    buf.WriteByte(byte(messageID))

    if data != nil {
        enc := gob.NewEncoder(&buf)
        err := enc.Encode(data)
        if err != nil {
            return fmt.Errorf("Could not encode TCP message, %v", err)
        }
    }

    // Now send the packet.
    return me.framer.Send(buf.Bytes())
}


// Receive - Blocking call to receive, and decode, the next message.
func (me *gobEncoder) Receive() (ReceivedMessage, error) {
    // First get the next frame.
    messageBytes, err := me.framer.Receive()
    if err != nil { return nil, err }

    // We know the command ID, look it up to find the expected data type.
    id := uint8(messageBytes[0])
    return makeGobReceivedMessage(id, messageBytes[1:]), nil
}


// Received message external API.

// ID - Report our message ID.
func (me *gobReceivedMessage) ID() uint8 {
    return me.id
}


// Data - Unpack the message data into the given struct of the appropriate type.
func (me *gobReceivedMessage) Data(data interface{}) {
    buf := bytes.NewBuffer(me.messageBytes)
    dec := gob.NewDecoder(buf)
    dec.Decode(data) // XXX err
}


// Internals.

// gobEncoderFactory - A factory that makes Gob encoders.
type gobEncoderFactory struct {
}

// gobEncoder - An encoder that packs everything in Gob.
type gobEncoder struct {
    framer Framer

}

// gobReceivedMessage - A message received by a Gob encoder.
type gobReceivedMessage struct {
    id uint8
    messageBytes []byte
}



// makeGobEncoder - Make a Gob encoder that sits on top of the given framer.
func makeGobEncoder(framer Framer) *gobEncoder {
    var encoder gobEncoder
    encoder.framer = framer
    return &encoder
}


//makeGobReceviedMessage - Make a Gob received message.
func makeGobReceivedMessage(id uint8, messageBytes []byte) *gobReceivedMessage {
    var j gobReceivedMessage
    j.id = id
    j.messageBytes = messageBytes
    return &j
}

