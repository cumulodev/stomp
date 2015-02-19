package stomp

import (
	"bytes"
	"fmt"
	"strings"
)

func encodeHeader(key, value string) string {
	encode := func(v string) string {
		v = strings.Replace(v, "\r", "\\r", -1)
		v = strings.Replace(v, "\n", "\\n", -1)
		v = strings.Replace(v, ":", "\\c", -1)
		v = strings.Replace(v, "\\", "\\\\", -1)
		return v
	}

	return fmt.Sprintf("%s:%s", encode(key), encode(value))
}

func encodeFrame(f Frame) []byte {
	buf := bytes.Buffer{}

	// encode command
	buf.WriteString(f.Command)
	buf.WriteString("\n")

	// encode header
	for key, value := range f.Header {
		buf.WriteString(encodeHeader(key, value))
		buf.WriteString("\n")
	}
	buf.WriteString("\n")

	// encode body
	buf.Write(f.Body)
	// terminate frame
	buf.WriteString("\x00\n")

	return buf.Bytes()
}

func (c *Conn) writeFrame(f Frame, options ...Option) error {
	c.closeMu.Lock()
	closed := c.closed
	c.closeMu.Unlock()
	if closed {
		return fmt.Errorf("writing to closed connection")
	}

	for _, fn := range options {
		fn(f)
	}

	data := encodeFrame(f)

	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_, err := c.conn.Write(data)
	return err
}

func (c *Conn) writeHeartBeat() error {
	c.closeMu.Lock()
	closed := c.closed
	c.closeMu.Unlock()
	if closed {
		return fmt.Errorf("writing to closed connection")
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_, err := c.conn.Write([]byte{'\n'})
	return err
}
