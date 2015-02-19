package stomp

import "fmt"

type Option func(f Frame)

func Host(host string) Option {
	return func(f Frame) {
		f.Header["host"] = host
	}
}

func Heartbeat(client, server int) Option {
	return func(f Frame) {
		heartbeat := fmt.Sprintf("%d,%d", client, server)
		f.Header["heart-beat"] = heartbeat
	}
}

func Authenticate(username, password string) Option {
	return func(f Frame) {
		f.Header["login"] = username
		f.Header["passcode"] = password
	}
}

func Ack(ack AckMode) Option {
	return func(f Frame) {
		f.Header["ack"] = string(ack)
	}
}

func Persist(f Frame) {
	f.Header["persistent"] = "true"
}
