# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Telegram bot for task management within Telegram chats. The bot maps Telegram chats to Projects, chat members to Users, and allows creating Tasks assigned to users within projects.

## Architecture

The codebase follows Clean Architecture principles:
- `internal/model/` - Domain entities (Project, Task, User) and repository interfaces
- `internal/storage/sqlite/` - SQLite implementations of repository interfaces
- `internal/app/` - Bot business logic and command handlers
- `cmd/bot/` - Application entry point and configuration

Key patterns:
- Repository pattern for data access
- Dependency injection via constructor
- Interface-based design for easy testing/swapping implementations

## Development Commands

```bash
# Build the bot binary
make build

# Run the bot locally (requires .env file)
make run

# Run tests
make test          # full test suite
make test-short    # quick tests only

# Code quality
make fmt           # format code
make lint          # run linter

# Database operations
make db-reset      # backup current db and create fresh with migrations
make db-populate   # load fixture data from fixtures/fixtures.sql

# Dependency management
make vendor        # update and vendor dependencies
```

## Configuration

The bot uses environment variables with `TG_TASKS_BOT_` prefix:
- `TG_TASKS_BOT_TOKEN` - Telegram bot token (required)
- `TG_TASKS_BOT_DEBUG` - Enable debug logging

Configuration can be set via:
1. `.env` file (copy `.env.example` to `.env`)
2. Environment variables
3. Command-line flags (override environment)

## Database Schema

SQLite database with these tables:
- `users` - Telegram users (id, telegram_id, first_name, last_name, username)
- `projects` - Projects mapped to chat IDs (id, telegram_chat_id, title)
- `user_projects` - User-project relationships with roles (user_id, project_id, role)
- `tasks` - Tasks within projects (id, project_id, assignee_id, status, title, etc.)

Migrations are in `migrations/` and run automatically with `--migrate` flag.

## Bot Commands

Current commands:
- `/start` - Initialize project in current chat
- `/status` - Show project status and members
- `/home` - Display help message

Commands can be sent as:
- Direct: `/command`
- Mentioned: `@botname /command`

## Testing Approach

The project uses Go's standard testing package. Task storage implementation is currently commented out in tests, indicating work in progress on the task management features.
