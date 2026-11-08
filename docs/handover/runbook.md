# Runbook

GitHub builds and proves a release; it never deploys. There are no production credentials and no SSH keys to the VPS in GitHub. A maintainer deploys by hand, so every deploy is deliberate and attributable. Environments: local dev; staging (a Compose profile on the same VPS, separate ports and database, synthetic data only); production.

## Release

1. The promotion PR `development` → `main` is merged (merge commit); `main` is green, docs are updated, PR titles are clear (release notes come from merged PR titles).
2. A maintainer creates an annotated tag `vMAJOR.MINOR.PATCH` on a commit of `main` (`v0.x` until go-live). Agents never tag.
3. `release.yml` checks the tag is on `main`, reruns `make verify-full` and runs `make release VERSION=...`. The result is a draft GitHub Release holding `deploy-bundle-vX.Y.Z.tar.gz` (images, `deploy/`, migrations, `release-manifest.json`), SBOMs and `SHA256SUMS`. A maintainer reviews and publishes it.

## Deploy

UNTESTED ON THE VPS: run on staging first.

1. Download the bundle on your own laptop: `gh release download vX.Y.Z` (your own login; the VPS holds no GitHub credentials). Verify provenance with `gh attestation verify deploy-bundle-vX.Y.Z.tar.gz --repo <owner>/<repo>`.
2. Copy it to the server (`scp`), extract it, and run `deploy/scripts/deploy.sh vX.Y.Z --profile staging` (add `--dry-run` first), then `make smoke URL=...` against staging.
3. Production, after this checklist: staging smoke passed; backup verified; release notes read for migrations and config changes; `.env.production` and the KEK unchanged and in place; someone else knows a deploy is happening. Then repeat with `--profile prod`.

`deploy.sh` checks free disk space and a backup newer than 24 hours (or run `backup-now.sh`), verifies `SHA256SUMS`, loads the images, compares image ids with the manifest, runs migrations (`goose up`; expand and contract keeps the schema compatible with the old and new application), starts the stack, runs the smoke test, returns to the previous version on failure, and prints a line for `deploy-log.md` (a human commits it; the VPS has no git).

Environment files (`.env.production`, `.env.staging`, mode 0600) and the KEK file live in `/opt/bappelitbangda` (see `deploy/env.example`). They are never in a bundle. The KEK is delivered as a Docker secret or systemd credential, outside the repo and outside the backup set.

## Rollback

`deploy/scripts/rollback.sh --profile staging|prod` redeploys the previous version (its images are kept). Migrations are not rolled back automatically: run a `Down` only after review. If the last migration was not backward compatible, restore from backup (target recovery time 4 hours, NFR-10).

## Backup and restore

`deploy/scripts/backup-now.sh --profile prod` writes an `age`-encrypted `pg_dump` to `/opt/bappelitbangda/backups`. It needs `BACKUP_AGE_RECIPIENT` (public key; the private key is held off the server, OQ-17) and refuses to write an unencrypted file. Copying backups off the server is not automated yet (known gap; target depends on OQ-04). Restore steps: TODO, written during the first restore drill. Record each drill here: date, who, backup used, restore time, test KTP decrypted with the key (yes/no).

## Key rotation

TODO

## Add an admin

TODO

## TOTP break-glass

TODO

Note: this repository is public. Keep key custody details and break-glass secrets out of it; record only the procedure and where the private copy is kept.
