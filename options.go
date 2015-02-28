package stomp

import "fmt"

// Option represents a function that can modify a STOMP frame before it is sent
// to the STOMP server. Option is typically used to set headers on the frame.
type Option func(f Frame)

// Host sets the name of a virtual host that the client wishes to connect to.
func Host(host string) Option {
	return func(f Frame) {
		f.Header["host"] = host
	}
}

// Heartbeat sets the heart beating headers on the frame. Heart-beating is
// used to test the healthiness of the underlying TCP connection and to make
// sure that the remote end is alive and kicking.
//
// In order to enable heart-beating, each party has to declare what it can do
// and what it would like the other party to do.
//
// The first number represents what the sender of the frame can do (outgoing
// heart-beats):
//  * 0 means it cannot send heart-beats.
//  * otherwise it is the smallest number of milliseconds between heart-beats
//    that it can guarantee.
//
// The second number represents what the sender of the frame would like to get
// (incoming heart-beats):
//  * 0 means it does not want to receive heart-beats.
//  * otherwise it is the desired number of milliseconds between heart-beats.
func Heartbeat(client, server int) Option {
	return func(f Frame) {
		heartbeat := fmt.Sprintf("%d,%d", client, server)
		f.Header["heart-beat"] = heartbeat
	}
}

// Authenticate add the required headers to authenticate against
// a secured STOMP server.
func Authenticate(username, password string) Option {
	return func(f Frame) {
		f.Header["login"] = username
		f.Header["passcode"] = password
	}
}

// Ack sets the acknowledgment mode of a subscription to the desired value.
func Ack(ack AckMode) Option {
	return func(f Frame) {
		f.Header["ack"] = string(ack)
	}
}

// Persist marks the STOMP frame as persistent. This tells the server to enable
// reliable messaging by allowing messages to be persisted so that they can be
// recovered if there is failure which kills the broker. Processing persistent
// messages has orders of magnitude more overhead than non-persistent variety.
// You should only use it if your application really needs it.
//
// Note that reliable messaging is not supported by each server or may need
// different header. This function enables reliable messaging for at least
// Apache ActiveMQ and Apache Apollo
func Persist() Option {
	return func(f Frame) {
		f.Header["persistent"] = "true"
	}
}
