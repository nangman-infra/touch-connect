# touch-connect on-prem deployment

This compose bundle is the first-time bootstrap target for the on-prem host.

Target host:

```text
172.16.0.34
```

Jenkins updates the running services through Watchtower after the first manual deploy.

## First deploy

```sh
cd /absolute/path/to/touch-connect/deploy/onprem
mkdir -p data/server data/nats
sudo chown -R 10001:10001 data/server
cp .env.example .env
```

`tc-server` runs as the non-root `touchconnect` user with UID `10001`; `data/server`
must be writable by that UID so SQLite can create `/data/touch-connect.db`.

Set `WATCHTOWER_HTTP_API_TOKEN` in `.env` to the same secret stored in Jenkins credential
`nangman-infra-touch-connect-watchtower-token`.
Set `TOUCH_CONNECT_HOST` to the LAN address workers should use for auto-discovery.
The default is `172.16.0.34`, and `tc-server` publishes it through mDNS/Bonjour
as `_touch-connect._tcp.local.`.

```sh
docker compose --env-file .env -f compose.yml pull
docker compose --env-file .env -f compose.yml up -d
docker compose --env-file .env -f compose.yml ps
```

## Health checks

```sh
curl -fsS http://172.16.0.34:8080/healthz
curl -fsS http://172.16.0.34:8081/readyz
```

Expected components:

```text
tc-server:  {"status":"ok","component":"tc-server",...}
tc-control: {"status":"ready","component":"tc-control",...}
```
