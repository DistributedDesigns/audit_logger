Create Type event_type_enum As Enum(
  'quote',
  'command',
  'account_action'
);

Create Table Logs (
  id Serial Primary Key,
  created_at TimestampTZ Default CURRENT_TIMESTAMP,
  user_id Varchar(50) Not Null,
  tx_id Bigint Not Null,
  event_type event_type_enum Not Null,
  content Text Not Null
);

Create Index uid_idx On Logs(user_id);
