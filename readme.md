Audit Logger
=====
[![Build Status](https://travis-ci.org/DistributedDesigns/audit_logger.svg?branch=master)](https://travis-ci.org/DistributedDesigns/audit_logger)

Listens for fresh quotes and audit events on RMQ and writes them to an xml log file.

## Installing
```sh
git clone https://github.com/DistributedDesigns/audit_logger.git

.scripts/install

# Start MySQL
make run

# Assumes there's a RMQ instance running in docker
# Run with one of
$GOPATH/bin/audit_logger
go run *.go
```

### Monitored Queues
#### Active
Other services intentionally create messages to send to the logger on these queues.
- `audit_event` -> Something to be written to the audit log, like a `<userCommand>`. Message body is pre-formatted XML.
-  `dumplog` -> Requests for user activity logs. User is passed in message `header["userID"]`. If the user is `admin` then a system dump is performed.

#### Passive
The logger snoops on these messages and records them.
- `quote_broadcast` -> `#.fresh` quotes are logged as `<quoteServer>` events.

### Sending RMQ messages
Sending messages to control / debug the logger is useful. You can do this through the [Management interface](http://localhost:8080/#/) or with the [`rabbitmqadmin` CLI](http://localhost:8080/cli). You can also generate quote traffic for snooping with the `fake_quote_server`, `quote_manager` and `rmq_proto` repos.

### Notes
- Logger will write until it receives a message on the `dumplog` queue with the header `userID == admin`. When it receives this message it writes the footer and closes the session log file.
- Validate logs with `./script/validate logs/<your-log>.xml`
- The MySQL root password is randomly generated. If you want to see what it is: `docker logs mysql-trading 2>/dev/null | grep GENERATED`
