-- Users
INSERT INTO users (tg_user_id, full_name, role, is_active) VALUES
(123456789, 'John Doe', 'admin', 1),
(987654321, 'Jane Smith', 'member', 1),
(456789123, 'Alice Johnson', 'member', 1),
(789123456, 'Bob Brown', 'guest', 0);

-- Projects
INSERT INTO projects (tg_chat_id, title, archived) VALUES
(100123456, 'Project Alpha', 0),
(100789012, 'Project Beta', 0),
(100345678, 'Project Gamma', 1);

-- User Projects
INSERT INTO user_projects (project_id, user_id, user_role) VALUES
(1, 1, 'manager'),
(1, 2, 'member'),
(1, 3, 'member'),
(2, 2, 'manager'),
(2, 3, 'member'),
(3, 1, 'manager'),
(3, 4, 'member');
