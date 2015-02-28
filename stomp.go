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
	AckAuto       AckMode = "auto"
	AckClient     AckMode = "client"
	AckIndividual AckMode = "client-individual"
)

type AckMode string

// A Conn represents a STOMP connection.
type Conn struct {
	// Err contains the last received error of an read operation.
	Err     error
	scanner *bufio.Scanner

	writeMu sync.Mutex // protects the Writer part of conn
	conn    io.ReadWriteCloser

	subsMu sync.Mutex
	subs   map[string]chan Message

	closeMu sync.Mutex
	closed  bool
}

// A Subscription represents a subscription on a STOMP server to
// a specified destination.
type Subscription struct {
	// Destination is the destination on the STOMP server this subscription
	// listens on.
	Destination string

	// C is the channel where messages received for this subscription
	// are sent to.
	C chan Message

	id string
}

// Dial connects to the given network address using net.Dial an then initializes
// a STOMP connection. Additional header and options can be given via the options parameter.
func Dial(network, addr string, options ...Option) (*Conn, error) {
	conn, err := net.Dial(network, addr)
	if err != nil {
		return nil, err
	}

	return Connect(conn, options...)
}

// Connects uses the given ReadWriteCloser to initialize a STOMP connection. Additional header
// and options can be given via the options parameter. See Dial for examples on how to use
// options.
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

// Close closes the connection and all associated subscription
// channels.
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

// Subscribe is used to register to listen to the given destination. Messages received
// for this subscription can be received via the C channel on the returned Subscription.
func (c *Conn) Subscribe(destination string, options ...Option) (*Subscription, error) {
	id := randID()
	frame := Frame{
		Command: "SUBSCRIBE",
		Header: Header{
			"id":          id,
			"destination": destination,
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
		id: id,
		C:  ch,
	}, nil
}

func (c *Conn) Unsubscribe(s *Subscription, options ...Option) error {
	frame := Frame{
		Command: "UNSUBSCRIBE",
		Header:  Header{"id": s.id},
	}

	if err := c.writeFrame(frame, options...); err != nil {
		return err
	}

	c.subsMu.Lock()
	defer c.subsMu.Unlock()
	delete(c.subs, s.id)
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
