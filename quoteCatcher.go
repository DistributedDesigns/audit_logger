package main

import (
	"database/sql"
	"fmt"
	"time"

	types "github.com/distributeddesigns/shared_types"

	"github.com/streadway/amqp"
)

func quoteCatcher() {
	ch, err := rmqConn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"quote_logger", // name
		true,           // durable
		false,          // delete when unused
		false,          // exclusive
		false,          // no wait
		nil,            // arguments
	)
	failOnError(err, "Failed to declare a queue")

	// Receive all fresh quotes
	freshQuotes := "*.fresh"
	err = ch.QueueBind(
		q.Name,           // name
		freshQuotes,      // routing key
		quoteBroadcastEx, // exchange
		false,            // no-wait
		nil,              // args
	)
	failOnError(err, "Failed to bind a queue")

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	failOnError(err, "Failed to register a consumer")

	db, err := sql.Open("postgres", dbConnAddr)
	failOnError(err, "Could not open DB connection")
	defer db.Close()

	insertLogStmt, err := db.Prepare(insertLogQuery)
	failOnError(err, "Could not prepare log insert statement")
	defer insertLogStmt.Close()

	go func() {
		consoleLog.Infof(" [-] Watching for '%s' on %s", freshQuotes, quoteBroadcastEx)

		for d := range msgs {
			ae := quoteToAuditEvent(string(d.Body), d.Headers)
			_, err := insertLogStmt.Exec(ae.UserID, ae.ID, ae.EventType, ae.Content)
			if err != nil {
				// Log error and wait for next
				consoleLog.Error("Problem inserting quote:", err)
				break
			}
		}
	}()

	<-done
}

func quoteToAuditEvent(s string, headers amqp.Table) types.AuditEvent {
	// Optimistic conversion :/
	quote, _ := types.ParseQuote(s)

	consoleLog.Info(" [↙] Intercepted quote TxID:", quote.ID)

	// More optimistic conversion :/
	server := headers["serviceID"].(string)
	if server == "" {
		server = "UNKNOWN"
	}

	nowMillisec := time.Now().UnixNano() / 1e6
	quoteMillisec := quote.Timestamp.UnixNano() / 1e6

	xmlElement := fmt.Sprintf(`
	<quoteServer>
		<timestamp>%d</timestamp>
		<server>%s</server>
		<transactionNum>%d</transactionNum>
		<price>%.2f</price>
		<stockSymbol>%s</stockSymbol>
		<username>%s</username>
		<quoteServerTime>%d</quoteServerTime>
		<cryptokey>%s</cryptokey>
	</quoteServer>`,
		nowMillisec, server, quote.ID, quote.Price.ToFloat(),
		quote.Stock, quote.UserID, quoteMillisec, quote.Cryptokey,
	)

	return types.AuditEvent{
		UserID:    quote.UserID,
		ID:        quote.ID,
		EventType: "quote",
		Content:   xmlElement,
	}
}
