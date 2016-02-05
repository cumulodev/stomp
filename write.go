package stomp

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"time"
)

type frame struct {
	body    *Frame
	options []Option
	ch      chan error
}

func (c *Conn) safeWrite(f *Frame, options ...Option) error {
	ch := make(chan error)
	frame := frame{
		body:    f,
		options: options,
		ch:      ch,
	}

	select {
	case <-c.closeC:
		return errors.New("connection closed")

	case c.writeC <- frame:
		return <-ch
	}
}

// unsafeWrite writes the next frame. This function is not thread safe!
func (c *Conn) unsafeWrite(f *Frame, options ...Option) error {
	if c.whb > 0 {
		c.conn.SetWriteDeadline(time.Now().Add(2 * c.whb))
	} else {
		c.conn.SetWriteDeadline(time.Time{})
	}

	_, err := c.conn.Write(encodeFrame(f, options...))
	return err
}

func encodeHeader(key, value string) string {
	encode := func(v string) string {
		v = strings.Replace(v, "\\", "\\\\", -1)
		v = strings.Replace(v, "\r", "\\r", -1)
		v = strings.Replace(v, "\n", "\\n", -1)
		v = strings.Replace(v, ":", "\\c", -1)
		return v
	}

	return fmt.Sprintf("%s:%s", encode(key), encode(value))
}

func encodeFrame(f *Frame, options ...Option) []byte {
	for _, fn := range options {
		fn(f)
	}

	// encode command
	buf := bytes.Buffer{}
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
