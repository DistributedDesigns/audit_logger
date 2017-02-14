package main

func auditEventCatcher(events chan<- string, done <-chan struct{}) {
	ch, err := rmqConn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	msgs, err := ch.Consume(
		auditEventQ, // queue
		"",          // consumer
		true,        // auto-ack
		false,       // exclusive
		false,       // no-local
		false,       // no-wait
		nil,         // args
	)
	failOnError(err, "Failed to register a consumer")

	go func() {
		consoleLog.Info(" [-] Monitoring", auditEventQ)

		for d := range msgs {
			consoleLog.Info(" [↓] Audit event for TxID:", d.Headers["transactionID"])
			events <- string(d.Body)
		}
	}()

	<-done
}
