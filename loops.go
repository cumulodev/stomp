package stomp

import (
	"errors"
	"time"
)

func (c *Conn) writeLoop() {
	for {
		select {
		case <-c.closeC:
			return

		case <-timeout(c.whb):
			err := c.unsafeWrite(&Frame{})
			if err != nil {
				c.error(err)
			}

		case frame := <-c.writeC:
			err := c.unsafeWrite(frame.body, frame.options...)
			if err != nil {
				c.error(err)
			}

			frame.ch <- err
		}
	}
}

func (c *Conn) readLoop() {
	for {
		select {
		case <-c.closeC:
			return

		case <-timeout(2 * c.rhb):
			c.error(errors.New("no heartbeat received"))

		case frame := <-c.safeRead():
			switch frame.Command {
			case "MESSAGE":
				c.dispatchMessage(frame)

			case "ERROR":
				c.error(NewError(frame))
			}

			//NOTE: if a default case is created, do not forget
			// about an empty frame (=heartbeat)
		}
	}
}

func timeout(d time.Duration) <-chan time.Time {
	if d == 0 {
		return make(chan time.Time)
	}

	return time.After(d)
}

func (c *Conn) dispatchMessage(frame *Frame) {
	c.subsMu.Lock()
	msg := &Message{*frame}
	if sub, ok := c.subs[msg.Subscription()]; ok {
		sub.C <- msg
	}
	c.subsMu.Unlock()
}
