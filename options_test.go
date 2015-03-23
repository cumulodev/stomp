package stomp

// Apollo supports expiring old messages. Unconsumed expired messages are
// automatically removed from the queue. There's two way to specify when the
// message will be expired. The first way to configure the expiration is by
// setting the expires message header. The expiration time must be specified
// as the number of milliseconds since the Unix epoch.
//
// To set this header, specify a expiry Option like below or as a normal
// function.
func ExampleOption_customHeaders() {
	var expires Option = func(f *Frame) {
		f.Header["expires"] = "1308690148000"
	}

	conn, _ := Dial("tcp", "localhost:61613")
	conn.Send("/queue/test", "text/plain",
		[]body{"this message will expire on Tue Jun 21 17:02:28 EDT 2011"},
		expires)
}
