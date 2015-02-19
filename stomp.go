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

func Dial(network, addr string, options ...Option) (*Conn, error) {
	conn, err := net.Dial(network, addr)
	if err != nil {
		return nil, err
	}

	return Connect(conn, options...)
}

func Connect(conn io.ReadWriteCloser, options ...Option) (*Conn, error) {
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
	}, options...)

	f, err := c.readFrame()
	if err != nil {
		return nil, err
	}

	if f.Command == "ERROR" {
		return nil, NewError(f)
	}

	//TODO parse CONNECTED frame
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

func (c *Conn) Subscribe(queue string, options ...Option) (*Subscription, error) {
	id := randID()
	frame := Frame{
		Command: "SUBSCRIBE",
		Header: Header{
			"id":          id,
			"destination": queue,
			"ack":         "auto",
		},
	}

	if err := c.writeFrame(frame, options...); err != nil {
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

func (c *Conn) Unsubscribe(s *Subscription, options ...Option) error {
	frame := Frame{
		Command: "UNSUBSCRIBE",
		Header:  Header{"id": s.Id},
	}

	if err := c.writeFrame(frame, options...); err != nil {
		return err
	}

	c.subsMu.Lock()
	defer c.subsMu.Unlock()
	delete(c.subs, s.Id)
	return nil
}

func (c *Conn) Send(destination, contentType string, body []byte, options ...Option) error {
	frame := Frame{
		Command: "SEND",
		Header: Header{
			"destination":  destination,
			"content-type": contentType,
		},
		Body: body,
	}

	if len(body) > 0 {
		frame.Header["content-length"] = fmt.Sprintf("%d", len(body))
	}

	return c.writeFrame(frame, options...)
}

func (c *Conn) Ack(m *Message, options ...Option) error {
	id := m.Ack()
	if id != "" {
		return c.writeFrame(Frame{
			Command: "ACK",
			Header:  Header{"id": id},
		}, options...)
	}
	return nil
}

func (c *Conn) Nack(m *Message, options ...Option) error {
	id := m.Ack()
	if id != "" {
		return c.writeFrame(Frame{
			Command: "NACK",
			Header:  Header{"id": id},
		}, options...)
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
