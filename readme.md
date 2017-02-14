Audit Logger
=====
Listens for fresh quotes and audit events on RMQ and writes them to an xml log file.

### Monitored Queues
#### Active
- `audit_event` -> Something to be written to the audit log, like a `<userCommand>`. Message body is pre-formatted XML.
-  `dumplog` -> Requests for user activity logs. User is passed in message `header["userID"]`. If the user is `admin` then a system dump is performed.

#### Passive
- `quote_broadcast` -> `#.fresh` quotes are logged as `<quoteServer>` events.
