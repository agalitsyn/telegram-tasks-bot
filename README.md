# Telegram Tasks Bot

Simple task management tool for organizing work in telegram chats.

- Each chat is `Project`.
- Chat members are `Project`'s `User`'s.
- `Project` have `Tasks` assigned to `User`'s.

## Configuraton

- Open [Botfather](https://t.me/botfather), register bot and get token
- Copy config template
  ```sh
  cp .env.example .env
  ```
- Fill config variables in `.env`

## Running locally

```sh
make run
```
