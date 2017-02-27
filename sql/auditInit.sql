CREATE TABLE Logs (
  id SERIAL PRIMARY KEY,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  user_id VARCHAR(50) NOT NULL,
  type ENUM('quote', 'command', 'account_action') NOT NULL,
  content TEXT NOT NULL
);
