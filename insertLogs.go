package main

type logItem struct {
	userID  string
	logType string
	content string
}

func insertLogs(done <-chan struct{}) {
	// Prepare the insertion statement so we can reuse the connection
	insertLogStmt, err := db.Prepare(`
    INSERT INTO Logs(user_id, type, content) VALUES(?, ?, ?)
  `)
	failOnError(err, "Could not prepare log insert statement")
	defer insertLogStmt.Close()

	consoleLog.Info(" [-] Waiting for logs to insert")

	for {
		select {
		case log := <-logs:
			consoleLog.Debugf(" [↑] Inserting %s: %s", log.logType, log.userID)
			_, err := insertLogStmt.Exec(log.userID, log.logType, log.content)
			if err != nil {
				// continue execution but log the error
				consoleLog.Error("Problem inserting log:", err)
			}
		case <-done:
			consoleLog.Notice(" [x] Finished inserting logs")
			break
		}
	}
}
