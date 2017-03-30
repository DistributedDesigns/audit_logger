package main

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	types "github.com/distributeddesigns/shared_types"
)

func dumplogWatcher(id int) {
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

	db, err := sql.Open("postgres", dbConnAddr)
	failOnError(err, "Could not open DB connection")
	defer db.Close()

	go func() {
		consoleLog.Infof(" [-] %d: Monitoring %s", id, dumplogQ)

		for d := range msgs {
			generateDumplog(db, string(d.Body))
		}
	}()

	<-done
}

func generateDumplog(db *sql.DB, csv string) {
	// Deserialize message
	dr, err := types.ParseDumplogRequest(csv)
	if err != nil {
		// Fail and wait for next
		consoleLog.Errorf("Could not parse Dumplog Request: %s | %s", csv, err.Error())
		return
	}

	consoleLog.Notice(" [!] Starting Dumplog for", dr.UserID)

	// Filter so users only see their messages but admins see everyone's
	var userFilter string // default value is ''
	if dr.UserID != "admin" {
		userFilter = fmt.Sprintf("And user_id = '%s'", dr.UserID)
	}

	dumplogQuery := fmt.Sprintf(`
		Select content
		From Logs
		Where event_type In ('quote', 'command') %s
		Order By created_at`, userFilter)

	// Run the query
	rows, err := db.Query(dumplogQuery)
	if err != nil {
		consoleLog.Error("Could not execute query for", dr.UserID)
		return
	}
	defer rows.Close()

	// Make a file to store the results
	logfile := createNewLogfile(dr)

	// Start with the header
	logfile.WriteString("<?xml version=\"1.0\"?>\n<log>")
	// Write the footer and close when this function terminates.
	// This ensures the file is left in a "clean" state if an error occurs
	// and the function terminates before it reaches the end.
	defer func() {
		logfile.WriteString("\n</log>\n")
		logfile.Close()
	}()

	// Read the results and write to logfile
	var content string
	for rows.Next() {
		err = rows.Scan(&content)
		if err != nil {
			consoleLog.Error("Problem reading dumplog row", err)
			return
		}

		logfile.WriteString(content)
	}

	if rows.Err() != nil {
		consoleLog.Error("Problem reading dumplog results", rows.Err())
		return
	}

	consoleLog.Noticef(" [✔] Dumplog for %s at %s", dr.UserID, logfile.Name())
}

func createNewLogfile(dr types.DumplogRequest) *os.File {
	// Prepend the current time and username to the filename to de-dupe
	// Structure is `logdir/$userID_$time_$filename.xml`
	now := time.Now()
	filename := fmt.Sprintf("%s/%s_%d-%02d-%02dT%02d%02d%02d_%s.xml",
		config.env.logFileDir, dr.UserID, now.Year(), now.Month(), now.Day(),
		now.Hour(), now.Minute(), now.Second(), dr.Filename,
	)

	auditLogFile, err := os.Create(filename)
	failOnError(err, "Could not create logfile")

	return auditLogFile
}
