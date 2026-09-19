# Tally — live polling

Create a poll, share the link, and watch the results move as votes land. No refresh.

**Flow:** create poll → share link → audience votes → live results

---

## Stack

| Layer | Tech | What it actually does here |
|---|---|---|
| Frontend | React 18 + Vite + React Router | Poll creation, voting UI, live result board over `EventSource` |
| Backend | Go 1.22 + Gin | Auth, validation, vote handling, SSE endpoint |
| Database | MongoDB | Durable records: users, polls, individual votes; unique indexes enforce correctness |
| Realtime | Redis | Live counters, pub/sub fan-out, duplicate-vote guard, read cache, rate limits |

---

## Run it locally

### With Docker (everything at once)

```bash
docker compose up --build
# frontend  http://localhost:5173
# backend   http://localhost:8080/healthz
```

### Without Docker

You need Go 1.22+, Node 20+, and Mongo and Redis reachable from your machine
(local installs, or free Atlas + Upstash instances).

```bash
# 1. backend
cd backend
cp .env.example .env        # fill in MONGO_URI, REDIS_URL, JWT_SECRET
go mod tidy
go run ./cmd/server

# 2. frontend, in a second terminal
cd frontend
npm install
npm run dev                 # Vite proxies /api to :8080, so no CORS locally
```

Open <http://localhost:5173>, create an account, create a poll, then open the
share link in a second browser (or a private window) and vote. The first
window updates without being touched.

---

## Project structure

```
live-polls/
├── backend/                       Go service
│   ├── cmd/server/main.go         wiring: config, db, routes, graceful shutdown
│   ├── internal/
│   │   ├── config/                every env var is read here and nowhere else
│   │   ├── db/                    Mongo + Redis connections, index creation
│   │   ├── models/                User, Poll, Option, Vote
│   │   ├── validate/              all input rules, one file, server-side only
│   │   ├── middleware/            JWT auth, CORS, rate limiting, body limits
│   │   ├── realtime/              the Redis layer: counters, pub/sub, guards
│   │   └── handlers/              auth, poll CRUD, vote, SSE stream
│   ├── Dockerfile
│   └── .env.example
├── frontend/                      React app
│   └── src/
│       ├── api/client.js          single place that talks to the backend
│       ├── context/AuthContext    token handling, current user
│       ├── hooks/useLiveResults   owns the EventSource connection
│       ├── components/            Navbar, ResultRow, ShareBar, ProtectedRoute
│       ├── pages/                 Home, Login, Signup, Dashboard, PollPage
│       └── styles.css             design tokens + component styles
├── docker-compose.yml
├── render.yaml
└── README.md
```

Frontend and backend never share a file. The only contract between them is the
JSON API below.

---

## API

| Method | Path | Auth | Purpose |
|---|---|---|---|
| POST | `/api/auth/signup` | — | Create an account, returns a JWT |
| POST | `/api/auth/login` | — | Sign in, returns a JWT |
| GET | `/api/auth/me` | required | Current user |
| POST | `/api/polls` | required | Create a poll |
| GET | `/api/polls` | required | Your polls, with live totals |
| GET | `/api/polls/:slug` | optional | Poll + current results + whether you voted |
| GET | `/api/polls/:slug/stream` | — | **SSE stream of live results** |
| GET | `/api/polls/:slug/results` | — | Snapshot fallback if SSE is unavailable |
| POST | `/api/polls/:slug/vote` | optional | Cast a vote |
| PATCH | `/api/polls/:slug/status` | owner | Close or reopen voting |
| DELETE | `/api/polls/:slug` | owner | Delete a poll and its votes |
| GET | `/healthz` | — | Health check for the platform |

---

## Key decisions

### Redis is the live path; Mongo is the record

A vote does this, in order:

1. validate the payload against the poll
2. `SADD` the voter key — the claim and the check are one atomic operation
3. `HINCRBY` the option's counter
4. `PUBLISH` the new counts to the poll's channel
5. respond to the voter
6. write the vote document to Mongo in the background

The audience is waiting on step 4, not step 6, so the durable write happens
after the response. If Redis is ever flushed, the counter hash is rebuilt from
the totals Mongo already holds, and the unique index on `(pollId, voterKey)`
still refuses a genuine duplicate. Redis is doing real work — counting, fanning
out, deduplicating, caching, rate limiting — not sitting there for show.

### SSE rather than WebSockets

The traffic is one-directional: the server pushes counts, the client posts votes
over ordinary HTTP. `EventSource` reconnects on its own, survives proxies that
drop idle WebSocket upgrades, and needs no extra protocol handling. Because each
stream subscribes to a Redis channel rather than to a local variable, a vote
received by one instance reaches viewers connected to every other instance — the
app scales horizontally without sticky sessions.

### Voting without an account, but creating with one

Requiring a login to vote would kill the share-a-link flow. Instead, an
anonymous voter gets a random `HttpOnly` cookie, and that value is hashed
together with the poll slug before it is stored — so the same person is not
traceable between polls, and the stored key is meaningless on its own. Creating
or managing a poll needs a real JWT-authenticated account.

### Everything is validated on the server

`internal/validate` is the whole trust boundary: question and option lengths,
duplicate options, control-character stripping, email shape, password strength.
The vote handler additionally checks that every submitted option id belongs to
that poll, which is what stops a crafted request from writing a stray field into
the counters. Client-side `maxLength` attributes are a convenience, never a
control.

### Other choices worth naming

- **bcrypt cost 12** for passwords; the "no such user" branch runs a dummy
  comparison so response timing doesn't reveal which emails are registered.
- **JWT algorithm pinned to HS256** at parse time, which closes the `alg: none`
  substitution attack.
- **CORS is an explicit allow-list**, never a reflected wildcard.
- **Rate limits live in Redis**, so they hold across every running instance
  rather than per-process.
- **No `WriteTimeout` on the HTTP server** — an SSE connection is long-lived by
  design and a write deadline would cut viewers off mid-poll.
- **Graceful shutdown** so in-flight votes finish when the platform redeploys.

---

## Deploying

The pieces are independent, so any combination works. One that costs nothing:

| Piece | Where | Notes |
|---|---|---|
| MongoDB | MongoDB Atlas free tier | Allow access from anywhere, or from your host's IPs |
| Redis | Upstash | Use the `rediss://` URL; `ParseURL` handles TLS |
| Backend | Render (Docker) | `render.yaml` is included; set the secret env vars |
| Frontend | Vercel or Netlify | Build `frontend`, set `VITE_API_BASE` to the API origin |

After both are up, set these on the backend or the flow will break:

- `CORS_ORIGINS` → the deployed frontend origin, exactly, no trailing slash
- `PUBLIC_APP_URL` → the same origin (this is what the share link is built from)
- `JWT_SECRET` → 32+ random characters (`openssl rand -base64 48`)

And on the frontend: `VITE_API_BASE` → the deployed backend origin.

Check `/healthz` on the API before debugging anything else.

---

## Things I'd add next

Result export to CSV, a per-poll QR code for in-room voting, and optional
IP-based throttling per poll for events where cookie clearing is likely.
