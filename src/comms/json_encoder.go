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
func MakeJSONEncoderFactory(/*typeMap MsgTypeMap*/) EncoderFactory {
    var factory jsonEncoderFactory
    //factory.typeMap = typeMap
    return &factory
}


// Make - Make a new JSON encoder that sits on top of the given byte connection.
func (me *jsonEncoderFactory) Make(connection ByteConnection) Encoder {
    framer := makePreLengthFramer(connection)
    encoder := makeJSONEncoder(framer)//, me.typeMap)
    return encoder
}


// Encoder external API.

// Send - Encode the given message and send it.
func (me *jsonEncoder) Send(messageID string, data interface{}) error {
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

    // We know the command ID, look it up to find the expected data type.
    id := header.ID
    //dataType, ok := me.typeMap[id]
    //var message TCPMessageFmt

    /*
    if ok {
        if id == "node_info" {
            _ = dataType.(RequestNodeParams)
            fmt.Printf("Type OK 1\n")
        }
        message.Data = &dataType
    } else {
        // We haven't been told about the received command ID. That's OK, process data as interface{}.
    }
    */

    //err = json.Unmarshal(messageBytes, &message)
    //if err != nil {
    //    return makeReceivedMessageError(fmt.Errorf("Error processing received message \"%s\", %v", id, err))
    //}

    // TODO: Handle nil dataType
    //fmt.Printf("Data: %v\n", dataType)

    return makeJSONReceivedMessage(id, messageBytes), nil
}


// Received message external API.

// ID - Report our message ID.
func (me *jsonReceivedMessage) ID() string {
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


/*
// Type map external API.

// MakeTypeMap - Trivial utility function to make a message type map.
func MakeTypeMap() MsgTypeMap {
    return make(map[string]interface{})
}


// Add - Trivial utility function to add a message type to a map.
// A struct of the desired type for the message data should be passed, NOT a pointer to it.
func (me MsgTypeMap) Add(commandID string, dataType interface{}) {
    me[commandID] = dataType
}


// MsgTypeMap - A dictionary specifying the expected type of the data for each commandID.
type MsgTypeMap map[string]interface{}
*/


// Internals.

// jsonEncoderFactory - A factory that makes JSON encoders.
type jsonEncoderFactory struct {
    //typeMap MsgTypeMap
}

// jsonEncoder - An encoder that packs everything in JSON.
type jsonEncoder struct {
    framer Framer
    //typeMap MsgTypeMap
}

// jsonReceivedMessage - A message received by a JSON encoder.
type jsonReceivedMessage struct {
    id string
    messageBytes []byte
}


/*
// tcpHeader - Received message header fields only.
type tcpHeader struct {
    ID string
}
*/


// makeJSONEncoder - Make a JSON encoder that sits on top of the given framer.
func makeJSONEncoder(framer Framer) *jsonEncoder {
    var encoder jsonEncoder
    encoder.framer = framer
    return &encoder
}


//makeJSONReceviedMessage - Make a JSON received message.
func makeJSONReceivedMessage(id string, messageBytes []byte) *jsonReceivedMessage {
    var j jsonReceivedMessage
    j.id = id
    j.messageBytes = messageBytes
    return &j
}

