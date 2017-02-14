package main

func auditEventCatcher(events chan<- string, done <-chan bool) {
	ch, err := rmqConn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	msgs, err := ch.Consume(
		config.Rabbit.Queues.Audit, // queue
		"",    // consumer
		true,  // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	failOnError(err, "Failed to register a consumer")

	go func() {
		consoleLog.Info(" [-] Monitoring", config.Rabbit.Queues.Audit)

		for d := range msgs {
			consoleLog.Info(" [↓] Audit event for TxID:", d.Headers["transactionID"])
			events <- string(d.Body)
		}
	}()

	<-done
}
