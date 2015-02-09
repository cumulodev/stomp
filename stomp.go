package stomp

import (
	"bufio"
	"fmt"
	"io"
	"math/rand"
	"net"
	"sync"
)

// Valid values for the "ack" subscription header entry.
const (
	AckAuto       = AckMode("auto")
	AckClient     = AckMode("client")
	AckIndividual = AckMode("client-individual")
)

type AckMode string

type Conn struct {
	Err     error
	scanner *bufio.Scanner

	writeMu sync.Mutex // protects the Writer part of conn
	conn    io.ReadWriteCloser

	subsMu sync.Mutex
	subs   map[string]chan Message

	closeMu sync.Mutex
	closed  bool
}

type Subscription struct {
	Id string
	C  chan Message
}

func Dial(network, addr string) (*Conn, error) {
	conn, err := net.Dial(network, addr)
	if err != nil {
		return nil, err
	}

	return Connect(conn)
}

func Connect(conn io.ReadWriteCloser) (*Conn, error) {
	c := &Conn{
		conn:    conn,
		scanner: bufio.NewScanner(conn),
		subs:    make(map[string]chan Message),
	}

	c.writeFrame(Frame{
		Command: "CONNECT",
		Header: Header{
			"host":           "localhost",
			"accept-version": "1.2",
			"heart-beat":     "0,1000",
		},
	})

	//TODO parse CONNECTED frame
	_, err := c.readFrame()
	if err != nil {
		return nil, err
	}

	go c.readLoop()
	return c, nil
}

func (c *Conn) Close() error {
	c.closeMu.Lock()
	c.closed = true
	c.closeMu.Unlock()

	c.closeSubscriptions()

	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.conn.Close()
}

func (c *Conn) closeSubscriptions() {
	c.subsMu.Lock()
	defer c.subsMu.Unlock()

	for _, ch := range c.subs {
		close(ch)
	}

	c.subs = make(map[string]chan Message)
}

func (c *Conn) Subscribe(queue string, ack AckMode) (*Subscription, error) {
	id := randID()
	frame := Frame{
		Command: "SUBSCRIBE",
		Header: Header{
			"id":          id,
			"destination": queue,
			"ack":         string(ack),
		},
	}

	if err := c.writeFrame(frame); err != nil {
		return nil, err
	}

	c.subsMu.Lock()
	defer c.subsMu.Unlock()
	ch := make(chan Message, 10)
	c.subs[id] = ch

	return &Subscription{
		Id: id,
		C:  ch,
	}, nil
}

func (c *Conn) Unsubscribe(s *Subscription) error {
	frame := Frame{
		Command: "UNSUBSCRIBE",
		Header:  Header{"id": s.Id},
	}
	err := c.writeFrame(frame)
	if err != nil {
		return err
	}

	c.subsMu.Lock()
	defer c.subsMu.Unlock()
	delete(c.subs, s.Id)
	return nil
}

func (c *Conn) Send(destination, contentType string, body []byte, userDefined Header) error {
	if userDefined == nil {
		userDefined = Header{}
	}

	userDefined["destination"] = destination
	userDefined["content-type"] = contentType
	if len(body) > 0 {
		userDefined["content-length"] = fmt.Sprintf("%d", len(body))
	}

	frame := Frame{
		Command: "SEND",
		Header:  userDefined,
		Body:    body,
	}
	return c.writeFrame(frame)
}

func (c *Conn) Ack(m *Message) error {
	id := m.Ack()
	if id != "" {
		return c.writeFrame(Frame{
			Command: "ACK",
			Header:  Header{"id": id},
		})
	}
	return nil
}

func (c *Conn) Nack(m *Message) error {
	id := m.Ack()
	if id != "" {
		return c.writeFrame(Frame{
			Command: "NACK",
			Header:  Header{"id": id},
		})
	}
	return nil
}

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randID() string {
	b := make([]rune, 8)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
