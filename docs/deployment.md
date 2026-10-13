# Deployment

Choose one scheduler: Vercel cron, `uptivra schedule`, or an external minute job running `uptivra tick`. PostgreSQL is the durable store. Never expose PostgreSQL directly to the Internet. Run migrations before serving a new version and keep the encryption key across deployments.

## Environment

| Variable | Where | Meaning |
| --- | --- | --- |
| `DATABASE_URL` | Go processes/function | PostgreSQL connection URL; use provider-required TLS in production, normally `sslmode=verify-full` with appropriate CA trust |
| `ENCRYPTION_KEY` | Go processes/function | Base64 encoding of exactly 32 random bytes: `openssl rand -base64 32` |
| `CRON_SECRET` | Go processes/function | At least 32 bytes; generate independently with `openssl rand -hex 32` |
| `APP_ORIGIN` | Go processes/function | Exact browser origin, e.g. `https://status.example.com`; no path, query or fragment. HTTP is accepted only for localhost development |
| `PORT` | Native Go server | Default `8080` |
| `NEXT_PUBLIC_DEMO_MODE` | Next.js build | Set exactly `true` for the temporary browser demo; unset for normal authenticated operation. Rebuild when changing |
| `API_ORIGIN` | Next.js server | Fixed Go backend origin; never a `NEXT_PUBLIC_` variable |
| `VERCEL_AUTOMATION_BYPASS_SECRET` | Next.js server, optional | Backend Vercel deployment-protection bypass; keep server-side |
| `ADMIN_EMAIL`, `ADMIN_PASSWORD` | Bootstrap command only | Initial account; password 14–72 bytes. Bootstrap refuses to replace an existing account |
| `POSTGRES_PASSWORD` | Compose only | Database password; a random hex value avoids URL escaping problems |

The application does not load `.env` itself. Compose reads it; native shell sessions must export it. Protect environment files (`chmod 600 .env`) and use your host's secret manager in production. Examples contain no production credentials.

## Docker Compose

Follow the [README quick start](../README.md#run-locally). Compose builds non-root Go and Next.js images, waits for PostgreSQL readiness, runs an explicit migration service, then starts the API, scheduler and frontend. Only `127.0.0.1:3000` is published. The `postgres-data` volume is persistent; `docker compose down -v` destroys it.

For a public installation, terminate HTTPS with a reverse proxy on the host. A minimal Caddy site is:

```caddy
status.example.com {
    reverse_proxy 127.0.0.1:3000
}
```

Set `APP_ORIGIN=https://status.example.com` before starting the backend. Do not put a caching layer in front of `/api/*`. No public Go port is necessary for the browser; Next.js proxies requests over the private network.

Inspect readiness and scheduler logs:

```sh
docker compose ps
docker compose logs --tail=100 api scheduler
docker compose exec api wget -qO- http://localhost:8080/healthz
curl --fail https://status.example.com/api/v1/public/status
```

Back up before upgrading. Build the new images, stop application writers, run migrations explicitly, then start services:

```sh
docker compose build
docker compose stop web api scheduler
docker compose run --rm migrate
docker compose up -d
```

Changing `POSTGRES_PASSWORD` does not change the password inside an existing database volume. Rotate the database role and application configuration together. Database major-version upgrades require PostgreSQL's documented upgrade procedure.

Compose configuration was validated with the official Compose binary. This development host had no Docker daemon, so image builds, container networking and first-run orchestration still require acceptance testing on a Docker host. The native equivalents were exercised against real PostgreSQL.

## Native development

Install Go 1.27.2, Node 24.11 and PostgreSQL 16. Create a database owned by the local PostgreSQL role, then configure the application:

```sh
createdb uptivra
cp .env.example .env
chmod 600 .env
openssl rand -base64 32
openssl rand -hex 32
```

Set `ENCRYPTION_KEY` and `CRON_SECRET` from those independent outputs. On a Linux installation with local peer authentication, set `DATABASE_URL='postgres:///uptivra?host=/var/run/postgresql'`; otherwise use your role/password/TLS connection URL. Keep the localhost origins from the example. Quote values containing shell metacharacters because the next commands source this trusted local file.

```sh
set -a
. ./.env
set +a
go mod download
go build -trimpath -o bin/uptivra ./cmd/uptivra
./bin/uptivra migrate
read -r -p 'Administrator email: ' ADMIN_EMAIL
read -r -s -p 'Administrator password: ' ADMIN_PASSWORD
printf '\n'
export ADMIN_EMAIL ADMIN_PASSWORD
./bin/uptivra bootstrap
unset ADMIN_EMAIL ADMIN_PASSWORD
npm --prefix web ci
```

Use three terminals with the same exported environment: `./bin/uptivra serve`, `./bin/uptivra schedule` and `npm --prefix web run dev`. Open `http://localhost:3000`. Bootstrap is a one-time command; subsequent password changes are in the authenticated settings screen. There is no unauthenticated password-reset endpoint.

For production Next.js, build and serve the standalone artifact:

```sh
npm --prefix web run build
cp -r web/.next/static web/.next/standalone/.next/static
cp -r web/public web/.next/standalone/public
HOSTNAME=127.0.0.1 PORT=3000 node web/.next/standalone/server.js
```

Copy into a fresh artifact directory on subsequent releases so stale assets do not accumulate.

## Temporary browser demo on Vercel

Deploy **one frontend project**, without PostgreSQL, the Go API, cron, or an administrator account:

1. Import this repository into a new Vercel project. Set **Root Directory** to `web` and **Framework Preset** to **Next.js**. Use the default install/build commands (`npm ci`, `npm run build`); do not use the repository-root Go `vercel.json`.
2. Set `NEXT_PUBLIC_DEMO_MODE=true` for the environments you want to demo (Production and/or Preview) **before building**. Leave `API_ORIGIN`, database variables, encryption keys, cron secrets and bypass secrets unset. No backend project is needed.
3. Deploy. Open `/`, create an example monitor, edit/pause/resume/delete it, change the status-page title in Settings, and open `/status` from the workspace. The preview stays in the same tab. Another browser or fresh tab starts with an independent workspace.
4. Confirm the “Temporary browser demo” banner is visible and no `/api/v1` calls appear in the browser network panel. Direct `/api/v1/*` requests return 403, even if an API origin was accidentally configured.

The flag is baked into the browser **and proxy** at build time. Changing it requires a new build/deployment; there is no URL switch. Keep demo and production in separate Vercel projects. Vercel documents [root-directory configuration](https://vercel.com/docs/builds/configure-a-build); Next.js documents [build-time public variables](https://nextjs.org/docs/app/guides/environment-variables).

This is a configuration and interface demo, not an uptime measurement service. It starts empty, holds at most 50 monitors and uses `sessionStorage` in the visitor's tab. Reloading preserves edits. The workspace expires two hours after its first access and is discarded on the next read; **Reset demo** clears it immediately. Closing a tab normally clears session storage, but browser session restoration or tab duplication can restore/copy it. Browser storage must be enabled. Theme preference continues to use browser-local storage independently.

No monitor configuration, credentials or history are sent to the application server. Vercel still serves the site and may retain ordinary hosting/access logs; this mode does not disable platform logging. Avoid adding analytics or third-party scripts if this boundary matters. Do not enter real secrets or sensitive URLs: browser-local data is readable by scripts on the same origin and anyone with access to that browser profile. The two-hour expiry is convenience cleanup, not secure erasure or an access-control boundary.

Live HTTP checks, scheduling, incidents/recovery, alerts, login/passwords and API keys need the full backend and are unavailable. No synthetic uptime, response times or incident history are generated: enabled monitors stay unknown, paused/maintenance controls are configuration previews, and `/status` shows only locally marked-public monitors. It cannot be shared as a real public status page. Demo form validation is a UI aid; it does not verify the Go engine's network/security rules. Use a full isolated installation to evaluate those behaviors.

Local preview and automated verification:

```sh
npm --prefix web ci
NEXT_PUBLIC_DEMO_MODE=true npm --prefix web run build
NEXT_PUBLIC_DEMO_MODE=true npm --prefix web run start
# In another terminal, after stopping the preview if necessary:
npm --prefix web run test:demo
```

The demo browser suite builds and starts a production frontend on port 3100 with the flag enabled. It covers isolated workspaces, CRUD, unknown metrics, settings/status preview, expiry/reset, and blocked API forwarding. Install Playwright Chromium and its system dependencies first. A remote Vercel demo deployment has not been verified from this workspace.

## Vercel Pro

Use **two projects from the same repository**. The root Go project runs the API and scheduler; the `web` project runs Next.js. This keeps Go business logic independent and uses the supported Go Function adapter. Vercel's Go runtime is currently beta. A mixed Services setup is not required by this implementation; it has not been verified here.

1. Provision PostgreSQL reachable from Vercel, preferably in the function region. Configure encrypted transport and a managed connection pooler where appropriate. Each function instance opens at most four connections; concurrent instances multiply this total.
2. Reserve the frontend production origin. Generate the encryption key and cron secret once. Run the native binary's `migrate` and `bootstrap` commands from a trusted environment against the production database using the production `APP_ORIGIN`. Remove bootstrap credentials afterward. Do not upload them to either project.
3. Create the **backend project**, repository root, framework **Other**. Keep the root `vercel.json`; it defines the Go function, a 60-second duration, routing and `* * * * *` cron at `/api/cron`. There is no frontend install/build command in this project. Set `DATABASE_URL`, `ENCRYPTION_KEY`, `CRON_SECRET` and `APP_ORIGIN` for Production. Deploy after schema initialization.
4. Create the **frontend project**, root directory **web**, framework **Next.js**. Set server-side `API_ORIGIN` to the backend production HTTPS origin. Deploy to the exact origin configured by `APP_ORIGIN`; an origin mismatch blocks cookie mutations. For a protected backend, also set its automation bypass secret in the frontend's `VERCEL_AUTOMATION_BYPASS_SECRET`.
5. Confirm direct backend `/healthz`, frontend login/CRUD, public status, and an actual cron run in Vercel's dashboard. Check that configured deployment protection permits platform cron and your required health checks. Direct API clients also need any platform protection credentials. Frontend public status must be reachable without a Vercel login if you intend it to be public.
6. Follow the [acceptance checklist](operations.md#acceptance) before depending on notifications. Test a failed target and recovery with a real receiver, rather than merely observing a successful cron HTTP response.

The configured `CRON_SECRET` is automatically sent by Vercel cron as `Authorization: Bearer …`; the backend compares it before claiming work. The cron URL must be on the **backend**, not through the frontend's 15-second proxy. Cron schedules run on production deployments. Hobby's daily cron is insufficient; the supplied minute schedule needs Pro or higher. Scheduling is not exact real-time execution, and missed invocations are not replayed as synthetic observations.

Preview environments need their own database, encryption key, cron secret and matching origin. Never point untrusted previews at production data. A generated preview frontend domain will not match a fixed production `APP_ORIGIN`.

The official `@vercel/go` builder successfully compiled this adapter, and `vercel.json` was checked against the official schema. No remote Vercel deployment was made. Provider routing, protection settings, managed-database access and cron execution must be verified in your account. Preserve the root module name `uptivra` and adapter package declaration: the Go builder rewrites the module path based on that package.

Official references: [Go runtime](https://vercel.com/docs/functions/runtimes/go), [cron limits](https://vercel.com/docs/cron-jobs/usage-and-pricing), [cron management and authentication](https://vercel.com/docs/cron-jobs/manage-cron-jobs), [function duration](https://vercel.com/docs/functions/configuring-functions/duration), [project configuration](https://vercel.com/docs/project-configuration).

## VPS services

The [systemd examples](../deploy) run the Go server, scheduler and a prepared Next.js standalone bundle. Create an unprivileged `uptivra` account, place the binary at `/opt/uptivra/uptivra`, and copy the complete standalone bundle (including static/public files) to `/opt/uptivra/web`. Adjust the Node executable path in the web unit for your installation. Install the three units into `/etc/systemd/system`.

Store the backend environment in `/etc/uptivra.env`, owned by `root:uptivra` with mode `0640`. It needs the four required Go variables and `PORT=8080`. Run migrations and bootstrap with that environment before starting services. Install the frontend environment at `/etc/uptivra-web.env` with `API_ORIGIN=http://127.0.0.1:8080`; apply the same permissions. Do not retain administrator bootstrap credentials in service files.

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now uptivra-api uptivra-scheduler uptivra-web
sudo journalctl -u uptivra-scheduler -n 100
```

The native API listens on all interfaces at its configured port. Restrict port 8080 with the host firewall; publish only the TLS reverse proxy and keep PostgreSQL private. The frontend unit binds loopback. These units are installation examples, not remotely tested provisioning automation. Backups and external coverage monitoring remain your responsibility.
