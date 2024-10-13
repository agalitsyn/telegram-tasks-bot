CREATE TABLE projects (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tg_chat_id INTEGER NOT NULL
);
CREATE INDEX idx_projects_chat_id ON projects(tg_chat_id);

CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tg_user_id INTEGER NOT NULL UNIQUE
);
CREATE INDEX idx_users_telegram_user_id ON users(tg_user_id);

CREATE TABLE user_projects (
    project_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    user_role TEXT NOT NULL,
    PRIMARY KEY (project_id, user_id),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX idx_user_projects_user_id ON user_projects(user_id);
