Audit Logger
=====
[![Build Status](https://travis-ci.org/DistributedDesigns/audit_logger.svg?branch=master)](https://travis-ci.org/DistributedDesigns/audit_logger)

Listens for fresh quotes and audit events on RMQ and writes them to an xml log file.

## Installing
```sh
git clone https://github.com/DistributedDesigns/audit_logger.git

.scripts/install

# Start dependent services
# Assumes there's an RMQ running somewhere
docker-compose up

# Run with one of
$GOPATH/bin/audit_logger
go run *.go
```

### Monitored Queues
#### Active
Other services intentionally create messages to send to the logger on these queues.
- `audit_event` -> Something to be written to the audit log, like a `<userCommand>`. Message is a serialized `AuditEvent`.
-  `dumplog` -> Requests for user activity logs. Message is a serialized `DumplogRequest`.

#### Passive
The logger snoops on these messages and records them.
- `quote_broadcast` -> `#.fresh` quotes are logged as `<quoteServer>` events.

### Sending RMQ messages
Sending messages to control / debug the logger is useful. You can do this through the [Management interface](http://localhost:8080/#/) or with the [`rabbitmqadmin` CLI](http://localhost:8080/cli). You can also generate quote traffic for snooping with the `fake_quote_server`, `quote_manager` and `rmq_proto` repos.

### Notes
- Insertion is slow! With a high volume of logs it takes a while for PG to chew through the backlog of events in Redis.
- Validate logs with `./script/validate logs/<your-log>.xml`
