package main

import "github.com/streadway/amqp"

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
			consoleLog.Info(" [↓]", d.Headers["name"])
			events <- string(d.Body)
			logs <- auditEventToLog(d)
		}
	}()

	<-done
}

func auditEventToLog(d amqp.Delivery) logItem {
	return logItem{
		userID:  d.Headers["userID"].(string),
		logType: "command",
		content: string(d.Body),
	}
}
