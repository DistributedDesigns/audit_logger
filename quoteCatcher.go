package main

import (
	"fmt"
	"time"

	types "github.com/distributeddesigns/shared_types"

	"github.com/streadway/amqp"
)

func quoteCatcher(events chan<- string, done <-chan struct{}) {
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

	go func() {
		consoleLog.Infof(" [-] Watching for '%s' on %s", freshQuotes, quoteBroadcastEx)

		for d := range msgs {
			consoleLog.Info(" [↙] Intercepted quote TxID:", d.Headers["transactionID"])
			events <- quoteToAuditLog(string(d.Body), d.Headers)
		}
	}()

	<-done
}

func quoteToAuditLog(s string, headers amqp.Table) string {
	// Optimistic conversion :/
	quote, _ := types.ParseQuote(s)

	// Will default to 0 if header is missing
	// Optimistic conversion :/
	transactionID := headers["transactionID"].(int64)

	// More optimistic conversion :/
	server := headers["serviceID"].(string)
	if server == "" {
		server = "UNKNOWN"
	}

	unixMillisec := time.Now().UnixNano() / 1e6

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
		unixMillisec, server, transactionID, quote.Price.ToFloat(),
		quote.Stock, quote.UserID, quote.Timestamp.Unix(), quote.Cryptokey,
	)

	return xmlElement
}
