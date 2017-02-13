Audit Logger
=====
Listens for fresh quotes and audit events on RMQ and writes them to an xml log file.

Quotes come from the `quote_broadcast` exchange with routing keys `stock.status` where status is either `cached` or `fresh`. `*.fresh` quotes represent an external service request and are logged.

Audit events from the `audit_event` queue. Queue messages are pre-formatted log entries.
