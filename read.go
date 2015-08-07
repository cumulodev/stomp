package stomp

import (
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
			c.Close()
			return
		}

		switch frame.Command {
		case "MESSAGE":
			c.dispatchMessage(frame)

		case "ERROR":
			c.Err = NewError(frame)
			c.Close()
		}

		//NOTE: if a default case is created, do not forget
		// about an empty frame (=heartbeat)
	}
}

func (c *Conn) dispatchMessage(frame *Frame) {
	c.subsMu.Lock()
	defer c.subsMu.Unlock()

	msg := &Message{*frame}
	if ch, ok := c.subs[msg.Subscription()]; ok {
		ch <- msg
		return
	}

	c.Err = fmt.Errorf("message received for unknown subscription on %q", msg.Destination())
	c.Close()
}

func (c *Conn) readFrame() (*Frame, error) {
	// get stomp command
	command, err := c.readLine()
	if err != nil {
		return nil, err
	}

	if len(command) == 0 {
		// received heartbeat, return empty frame
		return &Frame{}, nil
	}

	// get stomp headers
	header := make(Header)
	for {
		line, err := c.readLine()
		if err != nil {
			return nil, err
		}

		if len(line) == 0 {
			break
		}

		key, value := decodeHeader(line)
		header[key] = value
	}

	// get stomp body
	body, err := c.readBody()
	if err != nil {
		return nil, err
	}

	return &Frame{
		Command: command,
		Header:  header,
		Body:    body,
	}, nil
}

func (c *Conn) readLine() (string, error) {
	line, err := c.reader.ReadString('\n')
	if err != nil {
		return nil, err
	}

	// strip CR LF
	if len(line) > 0 && line[len(line)-1] == '\n' {
		line = line[:len(line)-1]
	}
	if len(line) > 0 && line[len(line)-1] == '\r' {
		line = line[:len(line)-1]
	}

	return line
}

func (c *Conn) readBody() ([]byte, error) {
	data, err := c.reader.ReadBytes('\x00')
	if err != nil {
		return nil, err
	}

	// strip NULL
	if len(data) > 0 && data[len(data)-1] == '\x00' {
		data = data[:len(data)-1]
	}

	return data
}

func decodeHeader(header string) (string, string) {
	decode := func(v string) string {
		v = strings.Replace(v, "\\r", "\r", -1)
		v = strings.Replace(v, "\\n", "\n", -1)
		v = strings.Replace(v, "\\c", ":", -1)
		v = strings.Replace(v, "\\\\", "\\", -1)
		return v
	}

	kv := strings.SplitN(header, ":", 2)
	return decode(kv[0]), decode(kv[1])
}
