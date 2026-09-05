# Agents.md — JSTS-SaleBot

> This file provides guidance for AI coding agents (e.g. GitHub Copilot, Cursor, Claude) working on this repository.

---

## Project Overview

**JSTS-SaleBot** is a Telegram-based marketplace bot written in **Go**. It allows users to create sale posts directly inside Telegram, which are then routed through a moderation group before being published to an approved sales channel/group. It supports post lifecycle management (pending → approved/rejected → sold/bumped) and admin runtime configuration.

It is a rewrite of the TypeScript version of the same bot. The commands, MongoDB collections (`posts`, `users`), `config.json` and locale files are identical, so both versions can run against the same data.

**Stack:**
- Runtime: Go 1.26+ (single static binary, locales embedded)
- Telegram library: `github.com/go-telegram/bot` (Bot API 10.2, Rich Messages)
- Database: MongoDB via `go.mongodb.org/mongo-driver/v2`
- Lint/format: `gofmt`, `go vet`
- Infrastructure: Docker + Docker Compose (bot + MongoDB + mongo-express)

---

## Repository Structure

```text
/
├── main.go                       # Entry point: env, config, DB, bot, graceful shutdown
├── env.go                        # .env loader (no dependency)
├── internal/
│   ├── config/
│   │   ├── config.go             # Config struct (mirrors config.json), live Store, Save
│   │   └── schema.go             # Per-key /config validation rules
│   ├── controller/
│   │   ├── controller.go         # HandleUpdate: routing, sessions, wizard lifecycle, startup sync
│   │   ├── commands.go           # Command handlers that live on the controller
│   │   └── callbacks.go          # Inline-button routing
│   ├── db/db.go                  # MongoDB connection with retry
│   ├── listen/listen.go          # Registry: wait for a user's next message / button press
│   ├── locale/locale.go          # Locale discovery, T(), FAQ trees
│   ├── models/models.go          # Post & User documents (bson tags = Mongoose field names)
│   ├── repository/               # The ONLY place that talks to MongoDB
│   ├── rich/rich.go              # Rich Message constructors (Paragraph, Heading, List, Photo...)
│   ├── services/                 # Business logic, one file per concern
│   ├── testcases/testcases.go    # In-bot /test scenarios (admin-only)
│   └── tgutil/                   # Send/Answer/ClearButtons helpers, raw rich-message edit
├── locales/
│   ├── embed.go                  # //go:embed of the locale files
│   └── <lang>/common.json, faq.json
├── config.json.example           # Runtime config template (copy to config.json)
├── .env.example                  # Environment variable template (copy to .env)
├── Dockerfile                    # Multi-stage build (development + production targets)
├── docker-compose.yaml           # Bot + MongoDB + mongo-express services
└── go.mod
```

---

## Configuration Files

### `.env`
Copy from `.env.example`. Variables:
| Variable       | Description                                              |
|----------------|----------------------------------------------------------|
| `BOT_TOKEN`    | Telegram Bot API token from @BotFather (required)        |
| `MONGO_URI`    | MongoDB connection string; the path is the database name |
| `CONFIG_PATH`  | Optional, defaults to `config.json`                      |
| `LOCALES_DIR`  | Optional, read locales from disk instead of the embedded copy |

### `config.json`
Copy from `config.json.example`. Editable at runtime via `/config` (admin-only):

| Key                 | Type    | Description                                              |
|---------------------|---------|----------------------------------------------------------|
| `lang`              | string  | Default locale key from `locales/` (e.g. `"en"`)          |
| `moderationGroupId` | number  | Telegram group ID for the moderation channel             |
| `approvedGroupId`   | number  | Telegram group ID where approved posts are published     |
| `moderationTopicId` | number \| null | Forum topic ID within the moderation group        |
| `approvedTopicId`   | number \| null | Forum topic ID within the approved group          |
| `broadcastTopicId`  | number \| null | Forum topic for `/broadcast` (`null` = General)   |
| `timeOut`           | number  | Unused, kept for compatibility                           |
| `validatePrice`     | boolean | Whether to enforce numeric price input                   |
| `minimumPhotos`     | number  | Minimum number of photos/videos required per post (0 = optional) |
| `mediaLayout`       | string  | How multiple photos render: `"slideshow"` or `"collage"`  |
| `dailyBumpLimit`    | number  | Max number of bumps a user can perform per day per post  |
| `donationsEnabled`  | boolean | Enable `/donate` (default true)                          |
| `enableFaq`         | boolean | Enable `/faq` (default true)                             |

---

## Core Concepts

### Post Lifecycle
```
User sends /newPost
  → InputService collects: title, description, price, location, media
  → PostService renders the preview (Rich Message)
  → User confirms
  → Post saved to DB (status: "pending")
  → PostService sends the card to the moderation group
      → ModerationService: moderator approves → status: "approved", posted to approved group
      → ModerationService: moderator rejects  → status: "rejected", user notified
  → User can /myposts:
      → Mark as "sold"  → edits the approved-group message with the sold rendering
      → Bump post       → re-posts (subject to dailyBumpLimit)
```

### Update dispatch & waiting for input
`Controller.HandleUpdate` is the single entry point (registered as the library's default handler; the library runs each update on its own goroutine). It first offers the update to the **listener registry** (`internal/listen`), then routes commands and callbacks. A wizard step that needs the user's next message calls `Listen.WaitMessage(ctx, accept)`; the accept function filters by chat and user and returns true to consume the message. This replaces the `bot.on("message")` add/remove-listener pattern of the TypeScript version. Every wait is context-aware: shutdown or a newer `/newPost` from the same user cancels it.

### Session Management
Each Telegram user has an in-memory `Session` on the controller (`IsIdle`, `AwaitingDonation`), guarded by a mutex. `beginWizard` marks the user busy, cancels any previous wizard, and returns an `end` func that restores idle state; always `defer end()`.

### Localization
All user-facing strings live in `locales/<lang>/common.json`. `locale.Service` resolves `preferredLocale` → `languageCode` → `config.lang` and exposes `T(locale, key, params...)`. When adding new strings, update **every** language's `common.json` and run `go test ./internal/locale/` — the test fails on missing keys and warns on keys that no Go source references.

### Rich Messages
Posts, `/help` and reports are Rich Messages built with `internal/rich`. The block and text `type` discriminators are verified by `internal/rich/rich_test.go` against the Bot API wire format. Editing a message to a rich message uses `tgutil.EditRichMessage` (a direct API call) because the library's `EditMessageText` always sends its `text` field.

---

## Commands

| Command          | Access    | Description                                      |
|------------------|-----------|--------------------------------------------------|
| `/start`         | All users | Shows a welcome greeting                          |
| `/newPost`       | All users | Begins the post creation wizard                  |
| `/myposts`       | All users | Lists user's own posts with bump/sold actions    |
| `/lang`          | All users | Change language preference                       |
| `/faq`           | All users | View airsoft FAQ                                 |
| `/donate`        | All users | Donate Stars to support the bot (optional)       |
| `/help`          | All users | Shows available commands (roles see extra items) |
| `/pending`       | Moderator | List posts awaiting approval                     |
| `/clearpending`  | Moderator | Expire all pending posts                         |
| `/auth`          | Moderator | Show a user's role                               |
| `/config`        | Admin     | View/update `config.json` keys at runtime        |
| `/activeUsers`   | Admin     | List users currently inside the wizard           |
| `/promote`, `/demote` | Admin | Change a user's role                          |
| `/broadcast`     | Admin     | Post to the approved group's broadcast topic     |
| `/broadcastUsers`| Admin     | DM active users and pending/approved authors     |
| `/test`          | Admin     | Runs in-bot test cases from `internal/testcases` |

Commands are case-insensitive and matched as whole words at the start of the message (`/cmd@BotName` works too).

---

## Development Workflow

### Setup (local)
```bash
cp .env.example .env                 # Fill in BOT_TOKEN and MONGO_URI
cp config.json.example config.json   # Adjust group IDs and settings
go run .
```

### Setup (Docker)
```bash
cp .env.example .env
cp config.json.example config.json
docker compose up --build
```
Mongo-express is available at `http://localhost:8081` for DB inspection.

### Build for production
```bash
go build -o jsts-salebot .           # or: docker build --target production -t jsts-salebot .
```

### Linting & Testing
```bash
gofmt -l .        # must print nothing
go vet ./...
go test ./...     # locale integrity, /config parsing, broadcast error mapping, unit tests
```

---

## Agent Guidelines

### Do
- **Follow the layering**: business logic in `internal/services/`, database access in `internal/repository/`, document shapes in `internal/models/`, update routing in `internal/controller/`.
- **Keep MongoDB in the repositories.** Services and the controller never import the driver; they pass `repository.Fields` maps or typed arguments.
- **Preserve the on-disk contract**: bson field names in `internal/models`, collection names (`posts`, `users`), `config.json` keys and locale keys must stay compatible with the TypeScript version and existing databases.
- **Always update `locales/<lang>/common.json`** for all languages when adding user-facing messages. Validate with `go test ./internal/locale/`.
- **Read configuration through `config.Store.Get()`** at the start of a handler; never hardcode group IDs or language strings. New keys go in the `Config` struct, `KnownKeys`, `set()` and `Schema`.
- **Use `tgutil`** for sending (`Send`, `SendLog`, `SendRich`), answering callbacks (`Answer`) and clearing buttons (`ClearButtons`) so thread ids and error logging stay consistent.
- **Pass `ctx` everywhere** and make waits cancellable (`Listen.WaitMessage`/`WaitCallback` already are).
- **Log with a `[LEVEL - context]` prefix** (`[ERROR - ModerationService.handleCallback]`), matching the existing style.
- **Avoid conflicting update operators**: in upserts never put the same field in both `$set` and `$setOnInsert` (MongoDB error 40). See `UserRepository.UpsertUserWithInsert`.
- **Only `$set` non-empty profile fields** when refreshing a user, so Telegram omitting a field never wipes stored data.
- **Preserve Docker targets** (`development`/`production`) when modifying the `Dockerfile`.
- **Update documentation**: when implementing new features, update `docs/README.md` (Features list) and `docs/CHANGELOG.md`.
- **Add tests**: unit tests next to the code (`*_test.go`), and an in-bot scenario in `internal/testcases/` for anything that needs a real Telegram round-trip.
- **Run `gofmt`** before committing; CI rejects unformatted files.

### Don't
- **Don't bypass the moderation flow**: posts must pass through `moderationGroupId` before appearing in `approvedGroupId`.
- **Don't mutate `config.json` directly** from code; go through `config.Store.Update` (used by `AdminService.HandleConfig`).
- **Don't hardcode user-facing strings** in services; all text comes from `Deps.T(locale, key)`.
- **Don't import `internal/testcases`** outside of `internal/controller`; test cases are intentionally isolated.
- **Don't block the update goroutine indefinitely** without a context; every wait must observe `ctx.Done()`.
- **Don't use `panic` for expected failures**; return errors and log them at the boundary.

### Adding a New Command
1. Add the handler to an appropriate service (or create one in `internal/services/`).
2. Route it in `Controller.route()` (`internal/controller/controller.go`); gate on `isPrivate` and the required `models.AuthLevel` as the neighbours do.
3. Add help strings to `locales/<lang>/common.json` for all languages and reference them in `showHelp()`.
4. Document it in `docs/README.md` and this file.

### Adding a New Locale
1. Create `locales/<lang>/` with `common.json` (all keys) and `faq.json`.
2. Run `go test ./internal/locale/` — it errors on any missing key.
3. Rebuild (locales are embedded) or run with `LOCALES_DIR=./locales`.
4. Set `"lang": "<lang>"` in `config.json` to make it the default (users can override with `/lang`).

---

## Key Files Quick Reference

| File | Purpose |
|------|---------|
| `main.go` | App entry point |
| `internal/controller/controller.go` | All Telegram update routing & sessions |
| `internal/listen/listen.go` | Waiting for a user's next message/button |
| `internal/services/input.go` | Step-by-step user input collection |
| `internal/services/post.go` | Post formatting & publishing |
| `internal/services/moderation.go` | Approve/reject logic |
| `internal/services/myposts.go` | User post management (bump, sold) |
| `internal/services/admin.go` | Runtime config, broadcast, roles |
| `internal/services/payment.go` | Telegram Stars donations |
| `internal/repository/post.go`, `user.go` | MongoDB access |
| `internal/models/models.go` | Document shapes |
| `internal/config/config.go`, `schema.go` | Runtime configuration |
| `locales/` | Localized UI strings and FAQ by language |
| `config.json` | Runtime bot configuration |
