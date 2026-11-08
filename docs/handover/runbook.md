# Runbook

GitHub builds and proves a release; it never deploys. There are no production credentials and no SSH keys to the VPS in GitHub. A maintainer deploys by hand, so every deploy is deliberate and attributable. Environments: local dev; staging (a Compose profile on the same VPS, separate ports and database, synthetic data only); production.

## Release

1. The promotion PR `development` → `main` is merged (merge commit); `main` is green, docs are updated, PR titles are clear (release notes come from merged PR titles).
2. A maintainer creates an annotated tag `vMAJOR.MINOR.PATCH` on a commit of `main` (`v0.x` until go-live). Agents never tag.
3. `release.yml` checks the tag is on `main`, reruns `make verify-full` and runs `make release VERSION=...`. The result is a draft GitHub Release holding `deploy-bundle-vX.Y.Z.tar.gz` (images, `deploy/`, migrations, `release-manifest.json`), SBOMs and `SHA256SUMS`. A maintainer reviews and publishes it.

## Database runtime role

Once per environment, before deploying the first version with migration `00002_auth`:

1. Create the `app` role:
   ```bash
   docker compose -f docker-compose.yml --profile prod exec db psql -U postgres -d bappelitbangda -c "CREATE ROLE app LOGIN"
   # or on staging:
   docker compose -f docker-compose.yml --profile staging exec db-staging psql -U postgres -d bappelitbangda -c "CREATE ROLE app LOGIN"
   ```
2. Set its password interactively (typed interactively, never on the command line):
   ```bash
   docker compose -f docker-compose.yml --profile prod exec db psql -U postgres -d bappelitbangda -c "\password app"
   # or on staging:
   docker compose -f docker-compose.yml --profile staging exec db-staging psql -U postgres -d bappelitbangda -c "\password app"
   ```
3. Put the password in `DATABASE_URL` of the env file (`.env.production` or `.env.staging`).

## Deploy

UNTESTED ON THE VPS: run on staging first.

1. Download the bundle on your own laptop: `gh release download vX.Y.Z` (your own login; the VPS holds no GitHub credentials). Verify provenance with `gh attestation verify deploy-bundle-vX.Y.Z.tar.gz --repo <owner>/<repo>`.
2. Copy it to the server (`scp`), extract it, and run `deploy/scripts/deploy.sh vX.Y.Z --profile staging` (add `--dry-run` first), then `make smoke URL=...` against staging.
3. Production, after this checklist: staging smoke passed; backup verified; release notes read for migrations and config changes; `.env.production` and the KEK unchanged and in place; someone else knows a deploy is happening. Then repeat with `--profile prod`.

`deploy.sh` checks free disk space and a backup newer than 24 hours (or run `backup-now.sh`), verifies `SHA256SUMS`, loads the images, compares image ids with the manifest, runs migrations (`goose up`; expand and contract keeps the schema compatible with the old and new application), starts the stack, runs the smoke test, returns to the previous version on failure, and prints a line for `deploy-log.md` (a human commits it; the VPS has no git).

Environment files (`.env.production`, `.env.staging`, mode 0600) and the KEK file live in `/opt/bappelitbangda` (see `deploy/env.example`; host path configurable via `KEK_SECRET_FILE` for production or `KEK_SECRET_FILE_STAGING` for staging). They are never in a bundle. The KEK is delivered as a Docker secret or systemd credential, outside the repo and outside the backup set.

## Rollback

`deploy/scripts/rollback.sh --profile staging|prod` redeploys the previous version (its images are kept). Migrations are not rolled back automatically: run a `Down` only after review. If the last migration was not backward compatible, restore from backup (target recovery time 4 hours, NFR-10).

## Backup and restore

`deploy/scripts/backup-now.sh --profile prod` writes an `age`-encrypted `pg_dump` to `/opt/bappelitbangda/backups`. It needs `BACKUP_AGE_RECIPIENT` (public key; the private key is held off the server, OQ-17) and refuses to write an unencrypted file. Copying backups off the server is not automated yet (known gap; target depends on OQ-04). Restore steps: TODO, written during the first restore drill. Record each drill here: date, who, backup used, restore time, test KTP decrypted with the key (yes/no).

## Key rotation

### Data encryption key (DEK)

1. Rotate the active data key:
   ```bash
   docker compose -f docker-compose.yml --profile prod exec api /api keys rotate-dek
   # or on staging:
   docker compose -f docker-compose.yml --profile staging exec api-staging /api keys rotate-dek
   ```
2. Restart the API service so new writes use the new DEK:
   ```bash
   docker compose -f docker-compose.yml --profile prod restart api
   # or on staging:
   docker compose -f docker-compose.yml --profile staging restart api-staging
   ```
New writes use the new DEK after the restart.

### Key encryption key (KEK)

1. Append `kek-2 <base64>` to the KEK file (`/opt/bappelitbangda/secrets/kek` or staging `kek-staging`) and keep `kek-1`.
2. Set `KEK_ACTIVE_ID=kek-2` in the env file (`.env.production` or `.env.staging`).
3. Apply the updated configuration:
   ```bash
   docker compose -f docker-compose.yml --profile prod up -d api
   # or on staging:
   docker compose -f docker-compose.yml --profile staging up -d api-staging
   ```
4. Rewrap all data keys with the new KEK:
   ```bash
   docker compose -f docker-compose.yml --profile prod exec api /api keys rewrap
   # or on staging:
   docker compose -f docker-compose.yml --profile staging exec api-staging /api keys rewrap
   ```
5. Remove `kek-1` from the file and force recreate the container:
   ```bash
   docker compose -f docker-compose.yml --profile prod up -d --force-recreate api
   # or on staging:
   docker compose -f docker-compose.yml --profile staging up -d --force-recreate api-staging
   ```
6. Check health (`/api/v1/health`) and verify a login.
7. Keep the old KEK offline as long as backups made before the rewrap are retained.

## Add an admin

Create a super_admin user:

```bash
docker compose -f docker-compose.yml --profile prod exec api /api admin create --username U --full-name "N"
# or on staging:
docker compose -f docker-compose.yml --profile staging exec api-staging /api admin create --username U --full-name "N"
```

The temporary password is handed over in person; first sign-in forces a password change, then authenticator enrollment.

## TOTP break-glass

Reset authenticator enrollment:

```bash
docker compose -f docker-compose.yml --profile prod exec api /api admin reset-totp --username U
# or on staging:
docker compose -f docker-compose.yml --profile staging exec api-staging /api admin reset-totp --username U
```

And for a lost password:

```bash
docker compose -f docker-compose.yml --profile prod exec api /api admin reset-password --username U
# or on staging:
docker compose -f docker-compose.yml --profile staging exec api-staging /api admin reset-password --username U
```

Both end the user's sessions.

Note: this repository is public. Keep key custody details and break-glass secrets out of it; record only the procedure and where the private copy is kept.
