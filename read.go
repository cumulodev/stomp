package stomp

import (
	"bytes"
	"fmt"
	"strings"
)

var newline = []byte{}
var null = []byte{0x0}

func (c *Conn) readLoop() {
	for {
		frame, err := c.readFrame()

		c.closeMu.Lock()
		closed := c.closed
		c.closeMu.Unlock()

		// if connection was closed, the above received
		// error (if any) is most likely due to the
		// closed connection.
		if closed {
			return
		}

		if err != nil {
			c.Err = err
			return
		}

		switch frame.Command {
		case "MESSAGE":
			c.dispatchMessage(frame)

		case "ERROR":
			c.Err = NewError(frame)
			c.Close()
		}
	}
}

func (c *Conn) dispatchMessage(frame *Frame) {
	c.subsMu.Lock()
	defer c.subsMu.Unlock()

	msg := Message{*frame}
	if ch, ok := c.subs[msg.Subscription()]; ok {
		ch <- msg
		return
	}

	c.Err = fmt.Errorf("message received for unknown subscription on %q", msg.Destination())
	c.Close()
}

func (c *Conn) readFrame() (*Frame, error) {
	frame := &Frame{Header: Header{}}
	fn := readCommand

loop:
	for c.scanner.Scan() {
		data := c.scanner.Bytes()
		fn = fn(frame, data)
		if fn == nil {
			break loop
		}
	}

	if err := c.scanner.Err(); err != nil {
		return nil, err
	}

	return frame, nil
}

type frameFn func(frame *Frame, data []byte) frameFn

func readCommand(frame *Frame, data []byte) frameFn {
	if bytes.Equal(data, null) || bytes.Equal(data, newline) {
		// EOL instead of command -> heart beat
		return readCommand
	}

	frame.Command = string(data)
	return readHeader
}

func readHeader(frame *Frame, data []byte) frameFn {
	if bytes.Equal(data, newline) {
		// single EOL -> end of header
		return readBody
	}

	key, value := decodeHeader(data)
	if _, ok := frame.Header[key]; !ok {
		frame.Header[key] = value
	}

	return readHeader
}

func readBody(frame *Frame, data []byte) frameFn {
	frame.Body = append(frame.Body, data...)
	if bytes.HasSuffix(data, null) {
		frame.Body = stripNull(frame.Body)
		return nil
	}

	return readBody
}

func decodeHeader(header []byte) (string, string) {
	decode := func(v string) string {
		v = strings.Replace(v, "\\r", "\r", -1)
		v = strings.Replace(v, "\\n", "\n", -1)
		v = strings.Replace(v, "\\c", ":", -1)
		v = strings.Replace(v, "\\\\", "\\", -1)
		return v
	}

	kv := strings.SplitN(string(header), ":", 2)
	return decode(kv[0]), decode(kv[1])
}

func stripNull(data []byte) []byte {
	if len(data) > 0 && data[len(data)-1] == '\x00' {
		return data[0 : len(data)-1]
	}

	return data
}
