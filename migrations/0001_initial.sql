CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    tg_user_id INTEGER NOT NULL UNIQUE,
    full_name TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT 1
);
CREATE INDEX idx_users_tg_user_id ON users(tg_user_id);

CREATE TABLE projects (
    id INTEGER PRIMARY KEY,
    tg_chat_id INTEGER NOT NULL UNIQUE,
    title TEXT NOT NULL,
    archived BOOLEAN NOT NULL DEFAULT 0
);
CREATE INDEX idx_projects_tg_chat_id ON projects(tg_chat_id);

CREATE TABLE user_projects (
    project_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    user_role TEXT NOT NULL,
    PRIMARY KEY (project_id, user_id),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX idx_user_projects_user_id ON user_projects(user_id);

CREATE TABLE tasks (
    id INTEGER PRIMARY KEY,
    project_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL,
    deadline DATETIME,
    created_by INTEGER NOT NULL,
    updated_by INTEGER NOT NULL,
    assignee INTEGER,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (updated_by) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (assignee) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX idx_tasks_project_id ON tasks(project_id);
CREATE INDEX idx_tasks_assignee ON tasks(assignee);
