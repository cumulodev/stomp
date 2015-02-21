package stomp

import (
	"bufio"
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

		//NOTE: if a default case is created, do not forget
		// about an empty frame (=heartbeat)
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
	// get stomp command
	c.scanner.Split(bufio.ScanLines)
	if res := c.scanner.Scan(); !res {
		return nil, c.scanner.Err()
	}

	command := c.scanner.Text()

	if len(command) == 0 {
		// received heartbeat, return empty frame
		return &Frame{}, nil
	}

	// get stomp headers
	header := make(Header)
	for c.scanner.Scan() {
		raw := c.scanner.Text()
		if len(raw) == 0 {
			break
		}

		key, value := decodeHeader(raw)
		header[key] = value
	}

	// get stomp body
	c.scanner.Split(scanFrame)
	if res := c.scanner.Scan(); !res {
		return nil, c.scanner.Err()
	}

	body := c.scanner.Bytes()

	return &Frame{
		Command: command,
		Header:  header,
		Body:    body,
	}, nil
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

func scanFrame(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, '\x00'); i >= 0 {
		// we have a non-empty body.
		return i + 1, stripNull(data[0:i]), nil
	}
	if atEOF {
		return len(data), stripNull(data), nil
	}
	// request more data.
	return 0, nil, nil
}

func stripNull(data []byte) []byte {
	if len(data) > 0 && data[len(data)-1] == '\x00' {
		return data[0 : len(data)-1]
	}

	return data
}
