package stomp

import (
	"bufio"
	"math"
	"math/rand"
	"net"
	"time"
)

// ExponentialBackoffReconnect uses the exponential backoff algorithm that
// uses feedback to multiplicatively decrease the rate of reconnects until
// either a success or an acceptable rate has been found.
func ExponentialBackoffReconnect(n int, d time.Duration, err error) (bool, time.Duration) {
	k := math.Min(float64(n), 10)
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	ms := rnd.Intn(int(math.Pow(2, k)-1)) + 1
	slot := (100 * time.Millisecond)

	return true, time.Duration(ms) * slot
}

func (c *Conn) error(err error) {
	go func() {
		// stop read & write loop while reconnecting
		close(c.closeC)
		c.writeC = make(chan frame)
		c.readC = make(chan int, 1)
		c.closeC = make(chan struct{})

		var (
			n     = 1
			ok    bool
			sleep time.Duration
		)

		for ; err != nil; err = c.reconnect() {
			if ok, sleep = c.Reconnect(n, sleep, err); !ok {
				c.Err = err
				c.Close()
				return
			}

			n = n + 1
			time.Sleep(sleep)
		}

		go c.readLoop()
		go c.writeLoop()

		if c.ReconnectSuccess != nil {
			c.ReconnectSuccess(n)
		}
	}()
}

func (c *Conn) reconnect() error {
	conn, err := net.Dial(c.network, c.addr)
	if err != nil {
		return err
	}

	c.conn = conn
	c.reader = bufio.NewReader(conn)
	err = c.connect(c.options)
	if err != nil {
		return err
	}

	c.subsMu.Lock()
	defer c.subsMu.Unlock()

	for _, sub := range c.subs {
		frame := &Frame{
			Command: "SUBSCRIBE",
			Header: Header{
				"id":          sub.id,
				"destination": sub.destination,
				"ack":         "auto",
			},
		}

		if err := c.unsafeWrite(frame, sub.options...); err != nil {
			return err
		}
	}

	return nil
}
