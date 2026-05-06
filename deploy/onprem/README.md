# touch-connect on-prem deployment

This compose bundle is the first-time bootstrap target for the on-prem host.

Target host:

```text
172.16.0.34
```

Jenkins updates the running services through Watchtower after the first manual deploy.

## First deploy

```sh
sudo mkdir -p /srv/touch-connect/server /srv/touch-connect/nats
sudo chown -R "$USER":"$USER" /srv/touch-connect

cd /absolute/path/to/touch-connect/deploy/onprem
cp .env.example .env
```

Set `WATCHTOWER_HTTP_API_TOKEN` in `.env` to the same secret stored in Jenkins credential
`nangman-infra-touch-connect-watchtower-token`.

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
