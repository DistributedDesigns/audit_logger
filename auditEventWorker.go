package main

import "fmt"

func auditEventWorker(id int) {
	ch, err := rmqConn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	consumerName := fmt.Sprintf("audit_event_worker_%d", id)

	msgs, err := ch.Consume(
		auditEventQ,  // queue
		consumerName, // consumer
		true,         // auto-ack
		false,        // exclusive
		false,        // no-local
		false,        // no-wait
		nil,          // args
	)
	failOnError(err, "Failed to register a consumer")

	consoleLog.Infof(" [-] Worker %d watching for audit events on %s", id, auditEventQ)

	// Each worker gets their own redis connection
	redisConn := redisPool.Get()
	defer redisConn.Close()

	for {
		select {
		case d := <-msgs:
			consoleLog.Infof(" [↓] %d: %s", id, d.Headers["name"])
			_, err := redisConn.Do("RPUSH", auditEventKey, string(d.Body))
			failOnError(err, "Could not insert event into redis")
		case <-done:
			consoleLog.Noticef("Worker %d finished receiving audit events", id)
			return
		}
	}
}
