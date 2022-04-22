// SPDX-FileCopyrightText: 2022 SoftIron Limited <info@softiron.com>
// SPDX-License-Identifier: GNU General Public License v2.0 only WITH Classpath exception 2.0

/* The JSON encoder.

This is an encoder for use in MessageConnections. It implements the Encoder interface.

The encoder packs messages as JSON. A simple top level JSON object is added to wrap the command ID and data. Messages
look like this:

{
    ID: "command ID",
    Data: {
        ...
    }
}

Encoded messages are sent and received with a framer object that implements the Framer interface.

The data to use when decoding a message is specified by a type map, provided when the encoder is created. This map keys
off the command IDs. If an unrecognised command ID is received, the data is decoded as an interface{}.

*/

package comms

import "encoding/json"
import "fmt"


// Encoder Factory external API.

// MakeJSONEncoderFactory - Make a JSON encoder factory.
func MakeJSONEncoderFactory() EncoderFactory {
    var factory jsonEncoderFactory
    return &factory
}


// Make - Make a new JSON encoder that sits on top of the given byte connection.
func (me *jsonEncoderFactory) Make(connection ByteConnection) Encoder {
    framer := makePreLengthFramer(connection)
    encoder := makeJSONEncoder(framer)
    return encoder
}


// Encoder external API.

// Send - Encode the given message and send it.
func (me *jsonEncoder) Send(messageID uint8, data interface{}) error {
    // First build the packet to send.
    var message TCPMessageFmt
    message.ID = messageID
    message.Data = data

    dataBytes, err := json.Marshal(&message)
    if err != nil { return fmt.Errorf("Could not encode TCP message, %v", err) }

    // Now send the packet.
    return me.framer.Send(dataBytes)
}


// Receive - Blocking call to receive, and decode, the next message.
func (me *jsonEncoder) Receive() (ReceivedMessage, error) {
    // First get the next frame.
    messageBytes, err := me.framer.Receive()

    if err != nil { return nil, err }  // Propogate error.

    // Parse the JSON to see what message it is.
    // We only need the ID, but we parse the whole thing to ensure it's all valid JSON.
    var header TCPMessageFmt
    err = json.Unmarshal(messageBytes, &header)
    if err != nil {
        return nil, fmt.Errorf("Error processing received message, %v", err)
    }

    id := header.ID
    return makeJSONReceivedMessage(id, messageBytes), nil
}


// Received message external API.

// ID - Report our message ID.
func (me *jsonReceivedMessage) ID() uint8 {
    return me.id
}


// Data - Unpack the message data into the given struct of the appropriate type.
func (me *jsonReceivedMessage) Data(data interface{}) {
    // Now we have the concrete type of the data we can fully decode the message.
    var message TCPMessageFmt
    message.Data = data

    // We've already fully parsed this, so it shouldn't be able to return an error.
    json.Unmarshal(me.messageBytes, &message)
}


// Internals.

// jsonEncoderFactory - A factory that makes JSON encoders.
type jsonEncoderFactory struct {
}

// jsonEncoder - An encoder that packs everything in JSON.
type jsonEncoder struct {
    framer Framer
}

// jsonReceivedMessage - A message received by a JSON encoder.
type jsonReceivedMessage struct {
    id uint8
    messageBytes []byte
}


// makeJSONEncoder - Make a JSON encoder that sits on top of the given framer.
func makeJSONEncoder(framer Framer) *jsonEncoder {
    var encoder jsonEncoder
    encoder.framer = framer
    return &encoder
}


//makeJSONReceviedMessage - Make a JSON received message.
func makeJSONReceivedMessage(id uint8, messageBytes []byte) *jsonReceivedMessage {
    var j jsonReceivedMessage
    j.id = id
    j.messageBytes = messageBytes
    return &j
}

