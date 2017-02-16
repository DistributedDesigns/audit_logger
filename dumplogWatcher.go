package main

func dumplogWatcher(dump chan<- struct{}, done <-chan struct{}) {
	ch, err := rmqConn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	msgs, err := ch.Consume(
		dumplogQ, // queue
		"",       // consumer
		true,     // auto-ack
		false,    // exclusive
		false,    // no-local
		false,    // no-wait
		nil,      // args
	)
	failOnError(err, "Failed to register a consumer")

	go func() {
		consoleLog.Info(" [-] Monitoring", dumplogQ)

		for d := range msgs {
			userID := d.Headers["userID"].(string)
			if userID == "admin" {
				consoleLog.Notice(" [!] Admin dump triggered")
				// TODO: the dump
				dump <- struct{}{}
			} else {
				consoleLog.Info(" [x] Dump triggered for", userID)
				// TODO: the dump
			}
		}
	}()

	<-done
}
