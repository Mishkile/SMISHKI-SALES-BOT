# 🛍️ JSTS Sale Bot
[![License: CC BY-NC 4.0](https://img.shields.io/badge/License-CC%20BY--NC%204.0-lightgrey.svg)](LICENSE)  
[![Version](https://img.shields.io/endpoint?url=https://gist.githubusercontent.com/SM-26/d0f70dbe1fa864c035f90ddf5fcce2a5/raw/version_JSTS-SaleBot.json)](VERSION.md)  
  
(pronounced as "Just Sale Bot") is a Telegram sales bot that lets users create sale listings through a guided conversational flow. Posts go through admin moderation before being published to a public sales group.

This is built with **Go**, **[go-telegram/bot](https://github.com/go-telegram/bot)** and **MongoDB**. It is the Go rewrite of the TypeScript [JSTS-SaleBot](https://github.com/SM-26/JSTS-SaleBot), which was itself a JS version of [GoSaleBot](https://github.com/SM-26/GoSaleBot/). Commands, database collections, `config.json` and the locale files are unchanged, so it runs against an existing deployment's data.

---

## ✨ Features

- **Guided Post Creation** — Step-by-step flow: title → description → price → location → photos (with a step indicator)
- **Price Validation** — Optional numeric price validation, accepting thousands separators (`2,000`)
- **Media Upload** — Multi-photo and video support
- **Rich Messages** — Posts, `/help`, and reports render as Telegram **Rich Messages** (Bot API 10.2): headings, block quotes, lists, dividers and media galleries — with inline buttons attached to the same message
- **Configurable Media Layout** — Multiple photos render as a swipeable **slideshow** or a **collage** grid (`mediaLayout`)
- **Live Preview** — Users see a formatted preview, with Confirm/Cancel on the same message
- **Admin Moderation** — Posts sent to a moderation group with approve/reject buttons
- **Pending Management** — Admins can list (`/pending`) and bulk-expire (`/clearpending`) posts
- **Rejection Reasons** — Admins can provide an optional reason when rejecting
- **Post Bumping** — Users can bump their approved posts to the top (subject to daily limits)
- **Donations** — Users can support the bot via Telegram Stars (`/donate`)
- **Auto-Publish** — Approved posts are forwarded to a public sales group
- **Broadcasts** — Announce to the channel (`/broadcast`) or DM active/pending/approved users directly (`/broadcastUsers`)
- **Forum Topics** — Moderation and approved posts target specific group topics
- **Multi Localization** — User-specific language preferences with automatic detection from Telegram language settings. Supports multiple languages in structured `locales/<lang>/common.json` files.
- **User Mentions** — Deep-links (`tg://user`) for users without a username
- **Role-Based Access Control (RBAC)** — Granular authorization with User, Moderator, and Admin roles, replacing simple `isAdmin` flags.
- **Single static binary** — Locales are embedded; deploy one file plus `config.json`.

### 🌐 How Multi Localization Works

1. Locale definition
   - Each language has its own `locales/{lang}/common.json` file (currently `en`, `he`, `ru`).
   - Translation keys are shared across locales (same keys, different values).
   - The files are embedded into the binary at build time. Set `LOCALES_DIR=/path/to/locales` to read them from disk instead, e.g. to add a language without rebuilding.

2. Locale resolution
   - When a user interacts with the bot, `UserService.EnsureUser()` writes or updates the user profile.
   - `locale.Service.ResolveUserLocale(user)` prefers:
     - `user.preferredLocale` (from `/lang` selection)
     - `user.languageCode` (Telegram user language hint)
     - bot config default (`config.lang`)

3. Message rendering
   - `locale.Service.T(locale, key, params...)` loads `common.json` for `locale`, caches it, and returns the translation with `{placeholders}` substituted.
   - Missing keys fall back to the key string with a warning log.

4. `/lang` command
   - Sends an inline keyboard with the locales discovered in `locales/`.
   - Updates `user.preferredLocale` in MongoDB.
   - Future replies use the chosen locale.

5. Configuration tests
   - `internal/locale/locale_test.go` validates matching keys across `locales/*/common.json` and reports missing entries. It runs as part of `go test ./...`.

---

## 🤖 Commands

### 👤 User Commands
| Command | Description |
| :--- | :--- |
| `/start` | Show a welcome greeting. |
| `/newPost` | Start the flow to create a new sale post (Title → Desc → Price → Location → Media). |
| `/myposts` | View your active posts, bump them to the top, or mark them as sold. |
| `/lang` | Set your preferred language for bot interactions. |
| `/faq` | View airsoft frequently asked questions and information. |
| `/donate` | Support the bot by donating Telegram Stars. |
| `/help` | Show the list of available commands. |

### 🛡️ Moderator Commands
| Command | Description |
| :--- | :--- |
| `/pending` | View a list of posts waiting for approval with inline Approve/Reject buttons. |
| `/clearpending` | Bulk expire (reject) all currently pending posts. |
| `/auth` | View a user's authorization level and role (supports self, ID, username, or reply). |

### ⚙️ Admin Commands
| Command | Description |
| :--- | :--- |
| `/promote` | Increase a user's authorization level (e.g., User to Moderator, Moderator to Admin). |
| `/demote` | Decrease a user's authorization level (e.g., Admin to Moderator, Moderator to User). |
| `/activeUsers` | List users currently in the process of creating a post. |
| `/config` | View or update bot configuration at runtime (e.g., `/config dailyBumpLimit 5`). |
| `/test` | Run built-in test scenarios to verify bot functionality. |
| `/broadcast` | Send a message to the approved channel, either by replying to an existing message or by typing a new message. |
| `/broadcastUsers` | Send a direct message (PM) to every active user and every author of a pending or approved post (de-duplicated, excluding you). Text only; reports per-recipient delivery failures. |

Commands are case-insensitive (`/newpost`, `/NewPost` and `/NEWPOST` all work) and are matched as whole words at the start of the message.

---

## ❓ FAQ System

The bot supports a **localized, hierarchical FAQ system** that allows users to view Frequently Asked Questions in their preferred language. This was implemented to provide searchable, structured information without hardcoding text into services.

### Overview

- **Location**: `locales/<lang>/faq.json` (e.g., `en/faq.json`, `he/faq.json`)
- **Access**: Users run `/faq` to view all available questions and answers
- **Localization**: Each language has its own FAQ file with identical structure but translated content
- **Hierarchy**: Questions and answers are organized using dot notation (e.g., `1`, `1.1`, `2`, `2.1.1`) to create nested topics
- **Design Reference**: See [Issue #20](https://github.com/SM-26/JSTS-SaleBot/issues/20) for architectural decisions on this feature

### FAQ File Structure

Each FAQ file follows this JSON schema:

```json
{
    "meta": {
        "locale": "en"
    },
    "nodes": {
        "1": "Main Question Title",
        "1.1": "Answer or sub-question content",
        "1.2": "Another sub-topic at same level",
        "2": "Second Main Question",
        "2.1": "Answer to second question",
        "2.1.1": "Nested deeper level answer"
    }
}
```

**Key structure elements:**
- **`meta.locale`**: Language code (must match the directory name, e.g., `en`, `he`, `de`)
- **`nodes`**: Object containing all FAQ entries as key-value pairs
  - **Keys**: Hierarchical identifiers using dot notation
    - `1`, `2`, `3` = Main topics (top level)
    - `1.1`, `1.2`, `2.1` = Sub-topics (one level deep)
    - `1.1.1`, `2.1.2` = Deeper nesting (unlimited levels)
  - **Values**: Plain text strings (no Markdown; HTML tags like `<b>`, `<i>` are supported if you manually format in code)

Top-level keys are shown in ascending numeric order; nested keys keep the order in which they appear in the file.

### Adding or Editing FAQ Content

#### 1. **Edit an existing locale's FAQ**

Open `locales/<lang>/faq.json` and modify the `nodes` object:

```json
{
    "meta": { "locale": "en" },
    "nodes": {
        "1": "What is Airsoft?",
        "1.1": "Airsoft is a recreational sport...",
        "2": "What equipment do I need?",
        "2.1": "Basic equipment includes a weapon, protective gear..."
    }
}
```

#### 2. **Create a new language's FAQ**

1. Create a new directory: `locales/<lang>/` (e.g., `locales/fr/`)
2. Copy the structure from an existing FAQ file and translate all values:

```bash
mkdir -p locales/fr
cp locales/en/faq.json locales/fr/faq.json
# Then edit locales/fr/faq.json with French translations
```

3. Update the `meta.locale` field to match the language code:

```json
{
    "meta": { "locale": "fr" },
    "nodes": { ... }
}
```

4. Run the locale validation tests to ensure the file is syntactically correct:

```bash
go test ./...
```

5. Rebuild the binary (locales are embedded), or run with `LOCALES_DIR=./locales` to pick the new directory up from disk.

### Best Practices

✅ **Do:**
- Keep FAQ entries concise and user-friendly
- Use consistent numbering (avoid gaps: use `1`, `1.1`, `1.2`, `2`; not `1`, `1.5`, `3`)
- Test the FAQ file with: `go test ./...` (validates JSON syntax and node structure)
- Use the `/faq` command in the bot to preview your FAQ before committing
- Update **all** language files when modifying structure (to keep them in sync)

❌ **Don't:**
- Leave the `meta.locale` field empty or mismatched with the directory name
- Use special characters or newlines in FAQ text—keep values as single-line strings
- Skip validation after editing—`go test ./...` ensures file integrity
- Create duplicate or out-of-order node keys (structure is for logical presentation, not sorting)

### Technical Implementation

- **Loading**: `locale.Service.GetFaqs(locale)` reads `faq.json` and returns the `nodes` map plus the ordered key list
- **Error Handling**: If a FAQ file is missing or invalid, users see: *"FAQ information not available for your language."*
- **Caching**: FAQ files are parsed once per locale and cached in memory
- **Locale Resolution**: FAQ respects user-specific locale preference (see [Multi Localization](#-how-multi-localization-works) section)

---

## 📁 Project Structure

```
.
├── main.go                       # Entry point — env, config, DB, bot, graceful shutdown
├── env.go                        # Minimal .env loader (no dependency)
├── internal/
│   ├── config/
│   │   ├── config.go             # config.json struct, live Store, persistence
│   │   └── schema.go             # Per-key type rules validating /config values
│   ├── controller/
│   │   ├── controller.go         # Update routing, sessions, startup sold-post sync
│   │   ├── commands.go           # /start, /newPost wizard, /help, /lang, /donate, admin commands
│   │   └── callbacks.go          # Inline button routing (approve/reject, sold/bump, lang, donate, test)
│   ├── db/db.go                  # MongoDB connection with retry
│   ├── listen/listen.go          # Registry for "wait for the user's next message/button" steps
│   ├── locale/locale.go          # User-specific localization & FAQ loading
│   ├── models/models.go          # Post & User document shapes (bson tags match the Mongoose schemas)
│   ├── repository/
│   │   ├── post.go               # Post CRUD + status query helpers
│   │   └── user.go               # User CRUD (upsert, isAdmin → authLevel migration)
│   ├── rich/rich.go              # Rich Message block/text constructors
│   ├── services/
│   │   ├── input.go              # Reusable input collection (text, price, media, confirm)
│   │   ├── post.go               # Post Rich Message formatting, publish to groups, sold edits
│   │   ├── myposts.go            # User post management (list, bump, mark sold)
│   │   ├── moderation.go         # Approve/reject logic & rejection reasons
│   │   ├── pending.go            # /pending listing & /clearpending bulk expire
│   │   ├── admin.go              # /config, /broadcast, promote/demote/auth
│   │   ├── broadcastusers.go     # /broadcastUsers audience resolution & throttled DM fan-out
│   │   ├── payment.go            # Donation invoice creation & payment event handling
│   │   ├── user.go               # User registration & role checks
│   │   └── faq.go                # FAQ command handler with locale-specific content
│   ├── testcases/testcases.go    # Manual test scenarios (in-bot /test)
│   └── tgutil/                   # Small helpers over the Telegram API
├── locales/
│   ├── embed.go                  # Embeds the locale files into the binary
│   ├── en/                       # English: common.json (UI strings) + faq.json
│   ├── he/                       # Hebrew
│   └── ru/                       # Russian
├── config.json.example           # Runtime config template (copy to config.json)
├── .env.example                  # Environment variable template (copy to .env)
├── Dockerfile                    # Multi-stage build (development + production targets)
└── docker-compose.yaml           # Bot + MongoDB + mongo-express services
```

---

## ⚡ Quick Start

### 🐳 Run with Docker (Recommended)

The `Dockerfile` is a multi-stage build with **two deployment targets**:

| Target | Use case | How it runs | Image contents |
| :--- | :--- | :--- | :--- |
| **`development`** | Local development & testing | `go run .` — source is bind-mounted, restart the container to pick up edits. | Go toolchain + module cache |
| **`production`** | Real deployment | A static binary on Alpine, as the non-root `bot` user | ~20 MB, no toolchain |

#### 1. Development — `docker compose`

`docker-compose.yaml` targets the **development** stage and brings up the bot, MongoDB and mongo-express together, networked out of the box.

In `.env`, point at the Mongo **service name** (not `localhost` — inside the container that would mean the bot itself):

``` env
MONGO_URI=mongodb://mongoserver:27017/SalesBotDB
```

Launch:

```bash
docker compose up -d          # add --build after changing dependencies or the Dockerfile
```

Monitor & manage:
- Bot logs: `docker compose logs -f bot`
- Database UI: Mongo Express at `http://localhost:8081` — username `admin`, password `pass`

#### 2. Production — build & run

The production target is **not** part of `docker-compose.yaml`. Build it and run it against your real MongoDB:

```bash
docker build --target production -t jsts-salebot:2.0.0 .

docker run -d --name jsts-salebot \
  --env-file .env \
  -v "$PWD/config.json:/app/config.json" \
  jsts-salebot:2.0.0
```

> **`config.json` is not baked into the image.** It is environment-specific (and gitignored), so it must be mounted at `/app/config.json` — the container exits on startup without it. Mount it **writable**: `/config` changes are persisted back to this file.

## Dev build  
### Prerequisites

- **Go** 1.26+ (CI and the Docker images use 1.27)
- **MongoDB** running locally (or a remote URI)
- A **Telegram Bot Token** from [@BotFather](https://t.me/BotFather)

### 1. Clone

```bash
git clone https://github.com/SM-26/JSTS-SaleBot.git
cd JSTS-SaleBot
```

### 2. Configure Environment

Copy the example env file and fill in your values:

```bash
cp .env.example .env
```

```env
BOT_TOKEN=your_telegram_bot_token
MONGO_URI=mongodb://localhost:27017/SalesBotDB
```

Optional variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `CONFIG_PATH` | `config.json` | Location of the runtime config file |
| `LOCALES_DIR` | *(embedded)* | Read `locales/` from this directory instead of the embedded copy |

The database name is taken from the URI path (`/SalesBotDB`); it falls back to `SalesBotDB` when the URI has none.

### 3. Configure Bot Settings

Copy `config.json.example` to `config.json` and edit it:

```json
{
  "lang": "en",
  "moderationGroupId": -100XXXXXXXXXX,
  "approvedGroupId": -100XXXXXXXXXX,
  "moderationTopicId": 11,
  "approvedTopicId": 22,
  "validatePrice": true,
  "minimumPhotos": 1,
  "dailyBumpLimit": 2,
  "donationsEnabled": true,
  "enableFaq": true,
  "mediaLayout": "slideshow",
  "broadcastTopicId": null
}
```

Values set via `/config` are validated per key (see `internal/config/schema.go`) — enums only accept their allowed values, numbers must be whole numbers, and nullable fields accept `null`.

| Field                | Type | Description                                       |
|----------------------|------|---------------------------------------------------|
| `lang`               | enum | Default locale key — must be a folder in `locales/` (`en`, `he`, `ru`) |
| `moderationGroupId`  | number | Telegram group where posts are reviewed           |
| `approvedGroupId`    | number | Telegram group where approved posts are published |
| `moderationTopicId`  | number \| null | Forum topic ID for moderation messages (set to `null` if not using topics) |
| `approvedTopicId`    | number \| null | Forum topic ID for published posts (set to `null` if not using topics) |
| `broadcastTopicId`   | number \| null | Forum topic for `/broadcast` (`null` = the General topic) |
| `validatePrice`      | boolean | Require numeric price input (accepts `2,000`)   |
| `minimumPhotos`      | number | Minimum photos/videos required per post (0 = optional) |
| `dailyBumpLimit`     | number | Maximum times a user can bump a post per day      |
| `donationsEnabled`   | boolean | Enable/Disable the /donate command                |
| `enableFaq`          | boolean | Enable/Disable the /faq command                   |
| `mediaLayout`        | enum | How multiple photos render: `"slideshow"` (swipeable) or `"collage"` (grid) |
| ~~`timeOut`~~        | number | ~~Post expiration timeout in minutes~~            |

### 4. Run

```bash
# Development
go run .

# Production
go build -o jsts-salebot .
./jsts-salebot
```

### 5. Test & lint

```bash
go test ./...     # locale integrity, /config parsing, broadcast error mapping, unit tests
go vet ./...
gofmt -l .        # prints files that need formatting (CI fails if any)
```

---

## 🔄 How It Works

```
User sends /newPost
    ↓
Bot collects: title → description → price → location → photos
    ↓
Preview shown to user → Confirm / Cancel
    ↓
Post saved to MongoDB (status: pending)
    ↓
Sent to moderation group with ✅ Approve / ❌ Reject buttons
    ↓
┌─ Approved → Published to sales group + user notified
└─ Rejected → Optional reason prompt → user notified
```

Each wizard step waits for the user's next message through the listener registry in `internal/listen`: the handler registers a listener, the controller offers every incoming update to the registered listeners first, and the listener that accepts the update is removed. Starting a second `/newPost` cancels the previous wizard.

---

## 🛠️ Tech Stack

| Layer        | Technology                  |
|--------------|-----------------------------|
| Runtime      | Go (single static binary)   |
| Telegram API | [go-telegram/bot](https://github.com/go-telegram/bot) v1.25 (Bot API 10.2, Rich Messages) |
| Database     | MongoDB + official Go driver v2 |
| Config       | JSON (`config.json`), validated by `internal/config/schema.go` |
| i18n         | Structured JSON (`locales/<lang>/common.json` + `locales/<lang>/faq.json`), embedded  |
| Validation   | `go test ./...` — Validates locale file syntax, key consistency, and FAQ structure |

---

## 📜 License

See [LICENSE.txt](../docs/LICENSE.txt) for details.

## Todo list
- [ ] make a logo for this project
- [ ] rework the faq with rich message
