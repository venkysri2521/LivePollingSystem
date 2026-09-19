# Before you submit

Work through this — the brief says a missed point can mean rejection.

## Build
- [ ] `docker compose up --build` runs clean from a fresh clone
- [ ] Create a poll, open the share link in a second browser, vote — the first
      window updates with no refresh
- [ ] Try to vote twice: second attempt is refused
- [ ] Try a poll with 1 option, an empty question, a 500-character question —
      all refused by the server, not just the form
- [ ] Close a poll from the dashboard; open tabs switch to closed state live

## Deploy
- [ ] Mongo Atlas cluster created, network access allows your backend
- [ ] Redis (Upstash) created, `rediss://` URL copied
- [ ] Backend deployed, `/healthz` returns ok
- [ ] `CORS_ORIGINS` and `PUBLIC_APP_URL` set to the real frontend origin
- [ ] `JWT_SECRET` is 32+ random characters, not the example value
- [ ] Frontend deployed with `VITE_API_BASE` pointing at the live API
- [ ] Open the live link on your phone on mobile data — full flow works there

## Repo
- [ ] Public GitHub repo
- [ ] No `.env` committed (`git log -p | grep -i secret` to be sure)
- [ ] README explains how to run it and your key decisions

## Video (3–5 min, mandatory)
- [ ] Walk through the live app, end to end
- [ ] The one challenge that gave you the most trouble, and how you solved it
- [ ] Whether you used AI tools, which ones, and how they helped or got in the way
- [ ] Unlisted YouTube or public Drive link, and check the link works signed out

## Email
- [ ] GitHub repo link
- [ ] Live deployed link
- [ ] Video link
- [ ] Sent to devhiring@hclguvi.com
