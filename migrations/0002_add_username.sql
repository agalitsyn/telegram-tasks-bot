ALTER TABLE users ADD COLUMN username TEXT;
CREATE INDEX idx_users_username ON users(username);