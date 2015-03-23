package main

import (
	"fmt"

	"github.com/cumulodev/stomp"
)

func main() {
	conn, _ := stomp.Dial("tcp", "localhost:61613")
	sub, _ := conn.Subscribe("queue.test", stomp.Ack(stomp.AckClient))

	for msg := range sub.C {
		fmt.Printf("<- %+v\n", msg)
		conn.Ack(msg)
	}

	if conn.Err != nil {
		fmt.Printf("ERROR: %+v\n", conn.Err)
	}
}
