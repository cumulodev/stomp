package main

import (
	"fmt"
	"time"

	"github.com/cumulodev/stomp"
)

func main() {
	conn, err := stomp.Dial("tcp", "localhost:61613")
	if err != nil {
		fmt.Printf("connect: %v\n", err)
		return
	}

	sub, _ := conn.Subscribe("queue.test", stomp.AckAuto)
	conn.Send("queue.test", "application/json", []byte("NEVER EVER JSON!"), nil)

	go func() {
		time.Sleep(3 * time.Second)
		fmt.Printf("TIME TO KILL!\n")
		conn.Close()
	}()

	for msg := range sub.C {
		fmt.Printf("<- %+v\n", msg)
	}

	time.Sleep(1 * time.Second)
	if conn.Err != nil {
		fmt.Printf("ups: %+v\n", conn.Err)
	}
}
