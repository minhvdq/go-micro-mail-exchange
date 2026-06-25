# Quarantio

**AI-powered email compliance and quarantine for Google Workspace.**

[quarantio.app](https://quarantio.app) · [Privacy Policy](https://quarantio.app/privacy) · [Terms of Service](https://quarantio.app/terms)

Quarantio connects to your Gmail account via OAuth and intercepts incoming emails in real time using Gmail Push Notifications (Pub/Sub). Each message is run through an AI compliance agent — powered by Mistral — that checks it against your organization's policies using RAG over uploaded policy documents. Risky emails are quarantined before they reach the inbox; clean emails are never touched.

---

## How it works

1. **Connect Gmail** — OAuth2 connects your Gmail account. Quarantio registers a Gmail watch so Google pushes a notification within ~1 second of any new inbox message.
2. **Compliance scan** — The AI agent embeds the email, retrieves relevant policy chunks and past history via pgvector similarity search, then returns a verdict: `CLEAN`, `LOW`, `MEDIUM`, or `HIGH`.
3. **Action** — Clean and low-risk emails are delivered normally. Medium-risk emails are quarantined for review. High-risk emails are blocked. All verdicts are logged in the audit trail.
4. **Review & release** — Admins review quarantined emails from the dashboard. Releasing an email restores it to the Gmail inbox as the original message — no re-sending, no impersonation.

---

## Architecture

```
Gmail ──Pub/Sub──▶ tenant-service ──sync HTTP──▶ ai-compliance-service
                        │                               │
                        │                          pgvector RAG
                        │                        (policy + history)
                        │                               │
                        ▼                               ▼
                   PostgreSQL ◀──────── audit log / quarantine
                        │
                   RabbitMQ ──▶ quarantine worker / blocked worker
```

| Service | Port | Role |
|---|---|---|
| `tenant-service` | 8082 | Auth, Gmail OAuth, Pub/Sub webhook, quarantine API, billing |
| `ai-compliance-service` | 8083 | Mistral agent, RAG pipeline, audit log writes |
| `mail-service` | internal | Brevo email delivery (quarantine notifications) |
| `front-end` | 3000 | React dashboard (Vite) |
| PostgreSQL + pgvector | 5432 | Primary store, vector similarity search |
| RabbitMQ | 5672 | Async quarantine/blocked routing |
| Redis | 6379 | Session cache |

---

## Tech stack

- **Go 1.25** — all backend services
- **Mistral AI** — `mistral-small-latest` for compliance reasoning, `mistral-embed` (1024-dim) for vector embeddings
- **pgvector** — policy and email history similarity search
- **RabbitMQ** — topic exchange routing (`email.quarantine`, `email.blocked`)
- **Google Gmail API** — OAuth2, Push Notifications (Pub/Sub), History API, label management
- **Brevo** — transactional email for quarantine notifications
- **Stripe** — subscription billing (Free / Starter / Pro / Business)
- **React + Vite + TypeScript** — frontend dashboard
- **Caddy** — reverse proxy with automatic TLS on production
- **Docker Compose** — local and production orchestration

---

## Local development

### Prerequisites

- Docker + Docker Compose
- Go 1.25+
- Node 20+
- A [Mistral API key](https://console.mistral.ai)
- A [Brevo API key](https://app.brevo.com) with a verified sender

### Setup

```bash
git clone https://github.com/minhvdq/Quarantio.git
cd Quarantio/project
cp .env.example .env   # fill in required keys
```

**.env required variables:**

```env
MISTRAL_API_KEY=
BREVO_API_KEY=
FROM_NAME=Quarantio
FROM_ADDRESS=your-verified@sender.com
ENCRYPTION_KEY=          # 32-byte hex string
JWT_SECRET=
GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=
GOOGLE_REDIRECT_URI=http://localhost:8082/v1/google/callback
STRIPE_SECRET_KEY=
STRIPE_WEBHOOK_SECRET=
STRIPE_PRICE_STARTER=
STRIPE_PRICE_PRO=
STRIPE_PRICE_BUSINESS=
PUBSUB_TOPIC=            # projects/<project>/topics/<topic>
PUBSUB_SECRET=           # shared secret for webhook auth
FRONTEND_URL=http://localhost:3000
APP_BASE_URL=http://localhost:8082
```

### Start all services

```bash
cd project
docker compose up -d
make migrate
```

Frontend dev server:

```bash
cd front-end
npm install
npm run dev
```

App runs at `http://localhost:3000`. API at `http://localhost:8082`.

### Useful make targets

| Command | Description |
|---|---|
| `make up` | Start all containers |
| `make down` | Stop all containers |
| `make migrate` | Apply all SQL migrations |
| `make build_tenant` | Build tenant-service binary |
| `make build_compliance` | Build ai-compliance-service binary |
| `make build_mail` | Build mail-service binary |
| `make start` | Start Vite dev server |

---

## Database migrations

Migrations live in `db-migrations/` and are applied in filename order. Run them against a live postgres container:

```bash
make migrate
```

---

## Gmail Pub/Sub setup

Quarantio uses Gmail Push Notifications for real-time scanning. To set this up:

1. Create a GCP Pub/Sub topic and grant `gmail-api-push@system.gserviceaccount.com` the `Pub/Sub Publisher` role on it.
2. Create a push subscription pointing to `https://api.yourdomain.com/v1/gmail/pubsub?token=<PUBSUB_SECRET>`.
3. Set `PUBSUB_TOPIC` and `PUBSUB_SECRET` in your `.env`.
4. Gmail watches expire after 7 days — the service auto-renews them on a 6-day ticker.

---

## Plans

| Plan | Monthly | Scans | Mailboxes | Retention |
|---|---|---|---|---|
| Free | $0 | 500 | 1 | 30 days |
| Starter | $29 | 5,000 | 5 | 90 days |
| Pro | $99 | 25,000 | 20 | 1 year |
| Business | $299 | Unlimited | Unlimited | 3 years |

---

## Production deployment

The production stack runs on Hetzner (Ubuntu) behind Caddy.

```bash
# On server
git clone https://github.com/minhvdq/Quarantio.git /opt/gomail
cd /opt/gomail/project
cp .env.example .env    # fill in production values
docker compose up -d
make migrate
```

Caddy config proxies `quarantio.app → localhost:3000` and `api.quarantio.app → localhost:8082` with automatic HTTPS.

---

## License

Proprietary. See [Terms of Service](https://quarantio.app/terms).
