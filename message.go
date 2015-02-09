package stomp

import "strconv"

type Header map[string]string

type Frame struct {
	Command string
	Header  Header
	Body    []byte
}

func (f *Frame) get(key string) string {
	if val, ok := f.Header[key]; ok {
		return val
	}
	return ""
}

type Error struct {
	Frame
}

func (m *Error) Error() string {
	if msg := m.get("message"); msg != "" {
		return msg
	}

	return "client received ERROR frame"
}

type Message struct {
	Frame
}

func (c *Conn) dispatchMessage(frame *Frame) {
	c.subsMu.Lock()
	defer c.subsMu.Unlock()

	msg := Message{*frame}
	if ch, ok := c.subs[msg.Subscription()]; ok {
		ch <- msg
	}
}

func (m *Message) Id() string {
	return m.get("message-id")
}

func (m *Message) Destination() string {
	return m.get("destination")
}

func (m *Message) Subscription() string {
	return m.get("subscription")
}

func (m *Message) Ack() string {
	return m.get("ack")
}

func (m *Message) ContentType() string {
	return m.get("content-type")
}

func (m *Message) ContentLength() int {
	length, _ := strconv.Atoi(m.get("content-length"))
	return length
}
