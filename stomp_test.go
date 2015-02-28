package stomp

// To connect to a STOMP server that requires authentication, the Authenticate options
// adds the required headers.
func ExampleDial_authenticate() {
	Dial("tcp", "localhost:61613", Authenticate("username", "password"))
}

// To override the default heart-beat settings, add an Heartbeat option to Dial:
func ExampleDial_heartbeat() {
	Dial("tcp", "localhost:61613", Heartbeat(0, 1000))
}

func ExampleConn_Subscribe() {
	conn, _ := Dial("tcp", "localhost:61613")
	sub, _ := conn.Subscribe("/queue/test")
	for frame := range sub.C {
	}
}

// To override the default acknowledge mode for messages, add an Ack option to Subscribe:
func ExampleConn_Subscribe_ackClient() {
	conn, _ := Dial("tcp", "localhost:61613")
	sub, _ := conn.Subscribe("/queue/test", Ack(AckClient))
	for frame := range sub.C {
	}
}
