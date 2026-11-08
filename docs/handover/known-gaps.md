# Known gaps

- PRD v1.1 is not signed.
- Open questions that block design: OQ-01, 02, 03, 04, 11, 12, 13, 14, 15, 16, 17.
- Technical design: PRD Appendix D is the decision log; no separate TDD, tech-stack or database-tooling document exists (see reconciliation item 1).
- Open with Diskominfo: Git host (organization or a Diskominfo server), whether CI can run there, whether source code may be sent to an AI service, who owns the system after January 2027.
- The runbook has no restore steps, key rotation, add-admin or TOTP break-glass yet, and no restore drill has been done.
- Deploy tooling (`deploy/`, `scripts/release.sh`) is tested only on a local Docker host, never on the real VPS; the systemd fallback (no Docker) does not exist yet and must be tested before 23 Nov.
- nginx has no TLS, HSTS or CSP yet (certificate and standard depend on OQ-16); upload size limits wait for OQ-12.
- nginx's admin allowlist matches /admin and /api/admin, but admin API paths are /api/v1/admin/...; fix before any admin endpoint ships (chore issue drafted with the auth spec).
- Backups are encrypted with `age`, but copying them off the server is manual; the encryption standard waits for OQ-16.
- SBOMs cover the Go module and the npm lockfile, not the container images.
- Antivirus scanning of public-form uploads (NFR-06) is not in the stack yet.
