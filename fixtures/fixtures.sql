-- Insert fake projects
INSERT INTO projects (tg_chat_id) VALUES
(123456789),
(987654321),
(555555555);

-- Insert fake users
INSERT INTO users (tg_user_id) VALUES
(11111111),
(22222222),
(33333333),
(44444444),
(55555555);

-- Insert fake user_projects relationships
INSERT INTO user_projects (project_id, user_id, user_role) VALUES
(1, 1, 'manager'),
(1, 1, 'manager'),
(1, 2, 'manager'),
(1, 3, 'member'),
(2, 1, 'manager'),
(2, 3, 'member'),
(2, 4, 'member'),
(3, 4, 'manager'),
(3, 5, 'member');
