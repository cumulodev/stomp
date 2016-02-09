package stomp

import (
	"bufio"
	"fmt"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"
)

const (
	// When the ack mode is auto, then the client does not need to send the server
	// ACK frames for the messages it receives. The server will assume the client
	// has received the message as soon as it sends it to the client. This
	// acknowledgment mode can cause messages being transmitted to the client to
	// get dropped.
	AckAuto AckMode = "auto"

	// When the ack mode is client, then the client MUST send the server ACK frames
	// for the messages it processes. If the connection fails before a client sends
	// an ACK frame for the message the server will assume the message has not been
	// processed and MAY redeliver the message to another client. The ACK frames
	// sent by the client will be treated as a cumulative acknowledgment. This means
	// the acknowledgment operates on the message specified in the ACK frame and all
	// messages sent to the subscription before the ACK'ed message.
	//
	// In case the client did not process some messages, it SHOULD send NACK frames
	// to tell the server it did not consume these messages.
	AckClient AckMode = "client"

	// When the ack mode is client-individual, the acknowledgment operates just like
	// the client acknowledgment mode except that the ACK or NACK frames sent by the
	// client are not cumulative. This means that an ACK or NACK frame for a
	// subsequent message MUST NOT cause a previous message to get acknowledged.
	AckIndividual AckMode = "client-individual"
)

// Valid values for the "ack" subscription header entry. If the header is not
// set, it defaults to auto.
type AckMode string

// A Conn represents a STOMP connection.
type Conn struct {
	// Err contains the last received error of an read operation.
	Err error

	Reconnect        func(n int, d time.Duration, err error) (bool, time.Duration)
	ReconnectSuccess func(n int)

	reader *bufio.Reader

	network string
	addr    string
	options []Option
	conn    net.Conn

	subsMu sync.Mutex
	subs   map[string]*Subscription

	rhb time.Duration
	whb time.Duration

	closeC chan struct{}
	writeC chan frame
	readC  chan int
}

// A Subscription represents a subscription on a STOMP server to
// a specified destination.
type Subscription struct {
	// Destination is the destination on the STOMP server this subscription
	// listens on.
	Destination string

	// C is the channel where messages received for this subscription
	// are sent to.
	C chan *Message

	id          string
	destination string
	options     []Option
}

// Dial connects to the given network address using net.Dial an then initializes
// a STOMP connection. Additional header and options can be given via the
// options parameter.
func Dial(network, addr string, options ...Option) (*Conn, error) {
	conn, err := net.Dial(network, addr)
	if err != nil {
		return nil, err
	}

	c := &Conn{
		Err:              nil,
		Reconnect:        ExponentialBackoffReconnect,
		ReconnectSuccess: nil,

		conn:    conn,
		network: network,
		addr:    addr,
		options: options,

		reader: bufio.NewReader(conn),
		subs:   make(map[string]*Subscription),

		rhb: 5 * time.Second,
		whb: 5 * time.Second,

		closeC: make(chan struct{}),
		writeC: make(chan frame),
		readC:  make(chan int, 1),
	}

	err = c.connect(options)
	if err != nil {
		return nil, err
	}

	go c.readLoop()
	go c.writeLoop()
	return c, nil
}

func (c *Conn) connect(options []Option) error {
	err := c.unsafeWrite(&Frame{
		Command: "CONNECT",
		Header: Header{
			"host":           "localhost",
			"accept-version": "1.2",
			"heart-beat":     fmt.Sprintf("%d,%d", c.whb/time.Millisecond, c.rhb/time.Millisecond),
		},
	}, options...)
	if err != nil {
		return err
	}

	f, err := c.unsafeRead()
	if err != nil {
		return err
	}

	if f.Command == "ERROR" {
		return NewError(f)
	}

	// parse connected frame and store heart beat
	connected := &Connected{*f}
	c.rhb = time.Duration(connected.ReadHeartBeat()) * time.Millisecond
	c.whb = time.Duration(connected.WriteHeartBeat()) * time.Millisecond
	return nil
}

// Close closes the connection and all associated subscription channels.
func (c *Conn) Close() error {
	log.Printf("DEBUG: closing ...")
	close(c.closeC)
	c.closeSubscriptions()
	return c.conn.Close()
}

func (c *Conn) closeSubscriptions() {
	c.subsMu.Lock()
	for _, sub := range c.subs {
		close(sub.C)
	}

	c.subs = make(map[string]*Subscription)
	c.subsMu.Unlock()
}

// Subscribe is used to register to listen to the given destination. Messages
// received for this subscription can be received via the C channel on the
// returned Subscription.
func (c *Conn) Subscribe(destination string, options ...Option) (*Subscription, error) {
	id := randID()
	frame := &Frame{
		Command: "SUBSCRIBE",
		Header: Header{
			"id":          id,
			"destination": destination,
			"ack":         "auto",
		},
	}
	sub := &Subscription{
		C:           make(chan *Message, 10),
		id:          id,
		destination: destination,
		options:     options,
	}

	if err := c.safeWrite(frame, options...); err != nil {
		return nil, err
	}

	c.subsMu.Lock()
	c.subs[id] = sub
	c.subsMu.Unlock()

	return sub, nil
}

// Unsubscribe removes an existing subscription and closes the receiver channel.
// Once the subscription is removed the STOMP connections will no longer receive
// messages from that subscription.
func (c *Conn) Unsubscribe(s *Subscription, options ...Option) error {
	frame := &Frame{
		Command: "UNSUBSCRIBE",
		Header:  Header{"id": s.id},
	}

	if err := c.safeWrite(frame, options...); err != nil {
		return err
	}

	c.subsMu.Lock()
	delete(c.subs, s.id)
	c.subsMu.Unlock()

	return nil
}

// Send sends a message to a destination in the messaging system.
//
// Note that STOMP treats this destination as an opaque string and no delivery
// semantics are assumed by the name of a destination. You should consult your
// STOMP server's documentation to find out how to construct a destination name
// which gives you the delivery semantics that your application needs.
//
// The reliability semantics of the message are also server specific and will
// depend on the destination value being used and the other message headers
// such as the "transaction" or "persist" header or other server specific
// message headers.
func (c *Conn) Send(destination, contentType string, body []byte, options ...Option) error {
	frame := &Frame{
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

	return c.safeWrite(frame, options...)
}

// Ack acknowledges the consumption of a message from a subscription using the
// client or client-individual acknowledgment modes. Any messages received from
// such a subscription will not be considered to have been consumed until the
// message has been acknowledged via Ack.
func (c *Conn) Ack(m *Message, options ...Option) error {
	id := m.Ack()
	if id != "" {
		return c.safeWrite(&Frame{
			Command: "ACK",
			Header:  Header{"id": id},
		}, options...)
	}
	return nil
}

// Nack is the opposite of Ack. It tells the server that the client did not
// consume the message. The server can then either send the message to a
// different client, discard it, or put it in a dead letter queue. The exact
// behavior is server specific.
//
// Nack applies either to one single message (if the subscription's ack mode is
// client-individual) or to all messages sent before and not yet Ack'ed or
// Nack'ed (if the subscription's ack mode is client).
func (c *Conn) Nack(m *Message, options ...Option) error {
	id := m.Ack()
	if id != "" {
		return c.safeWrite(&Frame{
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
