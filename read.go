package stomp

import (
	"strings"
	"time"
)

var newline = []byte{}
var null = []byte{0x0}
var heartbeat = []byte{'\n'}

func (c *Conn) safeRead() chan *Frame {
	ch := make(chan *Frame, 1)

	go func() {
		select {
		case <-c.closeC:
			return

		case c.readC <- 1:
			f, err := c.unsafeRead()
			if err != nil {
				c.error(err)
				return
			}

			<-c.readC
			ch <- f
		}
	}()

	return ch
}

// unsafeRead reads the next frame. This function is not thread safe!
func (c *Conn) unsafeRead() (*Frame, error) {
	if c.rhb > 0 {
		c.conn.SetReadDeadline(time.Now().Add(2 * c.rhb))
	} else {
		c.conn.SetReadDeadline(time.Time{})
	}

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
		return "", err
	}

	// strip CR LF
	if len(line) > 0 && line[len(line)-1] == '\n' {
		line = line[:len(line)-1]
	}
	if len(line) > 0 && line[len(line)-1] == '\r' {
		line = line[:len(line)-1]
	}

	return line, nil
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

	return data, nil
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
