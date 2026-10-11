# Instructions

## Creating a Migration

```shell
goose create init_tag sql
```

## Applying Migrations

```shell
The server applies the legacy public chain, the Runtime chain and the
Engineering chain on startup. For a standalone migration, run the repository's
migration command so the legacy Tag values can be copied through RuntimePort:

```shell
DATABASE_DSN="postgres://growscada:growscada@localhost:5432/growscada?sslmode=disable" \
RUNTIME_DATABASE_DSN="postgres://growscada:growscada@localhost:5432/growscada?sslmode=disable" \
go run ./cmd/migrate
```

`RUNTIME_DATABASE_DSN` is optional and defaults to `DATABASE_DSN`; use a
separate database URL to store Runtime state independently.
```

## During Debugging

Migrations are applied by the server when starting via Docker Compose. The
legacy state copy is repeat-safe and does not replace already-written Runtime
values.
