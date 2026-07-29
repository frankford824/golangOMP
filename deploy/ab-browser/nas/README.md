# Synology Clone B verification deployment

This deployment moves the reviewed Dev-Plus G7 Clone B environment to the
Synology host `192.168.0.125`. It is a writable test environment, not a
production release.

## Isolation

- LAN entry point: `http://192.168.0.125:18180`
- MySQL, Redis, backend, upload fixture and object fixture have no host ports.
- ERP sync, external-asset sync, AI, cleanup, preview, batch and scheduled
  workers are disabled.
- The database comes from the frozen local Clone B dump.
- Frontend, backend and fixture images retain the reviewed hashes.
- Configuration, fixture data and evidence live under
  `/volume1/docker/yongbo-v8-cloneb`; MySQL and Redis use project-scoped Docker
  volumes to avoid Synology ACL drift.
- Existing NAS containers and `/volume1/docker/mysql` are not reused or
  modified.

## Operations

On the NAS:

```bash
cd /volume1/docker/yongbo-v8-cloneb
./bootstrap.sh status
./bootstrap.sh stop
./bootstrap.sh start
```

The first `start` verifies every transferred artifact, imports the database
once, validates core table counts and starts the web edge. Restarting does not
reset test data.

## Tester handoff

Open:

```text
http://192.168.0.125:18180
```

Use existing Clone B test credentials shared out-of-band. Do not commit or
message passwords. UI actions write only the NAS Clone B database and fixture
upload root.

## Reset boundary

A reset requires a new attested dump and fixture snapshot. Never delete or
replace `data/mysql` while the Compose project is running. A future reset must
stop the project, preserve an evidence backup, replace only this deployment's
data, reimport, and rerun the count/hash checks.
