package main

import (
	"database/sql"

	types "github.com/distributeddesigns/shared_types"
	"github.com/garyburd/redigo/redis"
	_ "github.com/lib/pq"
)

// Re-used by quoteCatcher
const insertLogQuery = `
  INSERT INTO Logs(user_id, tx_id, event_type, content) VALUES($1, $2, $3, $4)
`

func logInsertWorker(id int) {
	// Get redis connection
	redisConn := redisPool.Get()
	defer redisConn.Close()

	db, err := sql.Open("postgres", dbConnAddr)
	failOnError(err, "Could not open DB connection")
	defer db.Close()

	// Prepare insert statement
	insertLogStmt, err := db.Prepare(insertLogQuery)
	failOnError(err, "Could not prepare log insert statement")
	defer insertLogStmt.Close()

	// Loop and BLPOP on new redis tx
	// pass off to insertLog() on hit
	for {
		select {
		case <-done:
			consoleLog.Noticef(" [x] %d: Finished inserting logs", id)
			return
		default:
			// Block until something is returned by redis, or timeout
			r, err := redis.Values(redisConn.Do("BLPOP", auditEventKey, 5))
			if err == redis.ErrNil {
				consoleLog.Debug("No new entries in", auditEventKey)
				break
			} else if err != nil {
				failOnError(err, "Could not get log from redis")
			}

			// Convert reply to a string
			// BLPOP returns [k, v] pairs but we only want value
			var csv string
			_, err = redis.Scan(r, nil, &csv)
			failOnError(err, "Could not convert log to string")

			// Convert to log object
			ae, err := types.ParseAuditEvent(csv)
			if err != nil {
				// Fail and wait for next :(
				consoleLog.Errorf("Could not parse Audit Event: %s | %s", csv, err.Error())
				break
			}

			// Insert into DB
			_, err = insertLogStmt.Exec(ae.UserID, ae.ID, ae.EventType, ae.Content)
			if err != nil {
				// Log error and wait for next
				consoleLog.Error("Problem inserting log:", err)
				break
			}

			consoleLog.Debugf(" [↑] Inserted %s: %d, %s", ae.UserID, ae.ID, ae.EventType)
		}
	}
}
