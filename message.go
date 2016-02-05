package stomp

import (
	"strconv"
	"strings"
)

type Header map[string]string

// Frame represents any STOMP frame. STOMP is a frame based protocol which
// assumes a reliable 2-way streaming network protocol (such as TCP) underneath.
// The client and server will communicate using STOMP frames sent over the
// stream.
type Frame struct {
	// Command specified what type this frame is.
	Command string

	// Header contains a collection of key -> value headers
	Header Header

	// Body contains the data of the optional body. Only the SEND, MESSAGE, and
	// ERROR frames MAY have a body. All other frames MUST NOT have a body.
	Body []byte

	ch chan error
}

func (f *Frame) get(key string) string {
	if val, ok := f.Header[key]; ok {
		return val
	}
	return ""
}

// Error represents an STOMP ERROR frame. The server MAY send ERROR frames if
// something goes wrong. In this case, it MUST then close the connection just
// after sending the ERROR frame. The ERROR frame SHOULD contain a message
// header with a short description of the error, and the body MAY contain
// more detailed information (or MAY be empty).
//
// This type implements also the error interface and is used as such. For
// example if the client receives an ERROR frame from the server, the frame
// is stored in the Err field of the Conn structure.
type Error struct {
	Frame
}

func NewError(f *Frame) *Error {
	return &Error{*f}
}

// Error returns the error message of the frame. The ERROR frame SHOULD contain
// a message header with a short description of the error, and the body MAY
// contain more detailed information (or MAY be empty).
func (m *Error) Error() string {
	if msg := m.get("message"); msg != "" {
		return msg
	}

	return "client received ERROR frame"
}

// Message represents a STOMP MESSAGE frame. MESSAGE frames are used to convey
// messages from subscriptions to the client.
type Message struct {
	Frame
}

// Id returns the unique identifier for that message.
func (m *Message) Id() string {
	return m.get("message-id")
}

// Destination returns the destination this message was sent to.
func (m *Message) Destination() string {
	return m.get("destination")
}

// Subscription returns the identifier of the subscription that is receiving
// the message.
func (m *Message) Subscription() string {
	return m.get("subscription")
}

// Ack returns the value of the "ack" header of a message. If the message is
// received from a subscription that requires explicit acknowledgment (either
// client or client-individual mode) then the MESSAGE frame MUST also contain an
// ack header with an arbitrary value. This header will be used to relate the
// message to a subsequent ACK or NACK frame.
func (m *Message) Ack() string {
	return m.get("ack")
}

// ContentType returns the content type of the frame body if set by the  server.
// If the content type is set, its value MUST be a MIME type which describes the
// format of the body. Otherwise, the receiver SHOULD consider the body to be a
// binary blob.
func (m *Message) ContentType() string {
	return m.get("content-type")
}

// ContentLength returns  an octet count for the length of the message body.
func (m *Message) ContentLength() int {
	length, err := strconv.Atoi(m.get("content-length"))
	if err != nil {
		return len(m.Body)
	}

	return length
}

type Connected struct {
	Frame
}

func (m *Connected) ReadHeartBeat() int {
	beats := strings.Split(m.get("heart-beat"), ",")
	if len(beats) != 2 {
		return 0
	}

	beat, _ := strconv.Atoi(beats[0])
	return beat
}

func (m *Connected) WriteHeartBeat() int {
	beats := strings.Split(m.get("heart-beat"), ",")
	if len(beats) != 2 {
		return 0
	}

	beat, _ := strconv.Atoi(beats[1])
	return beat
}
