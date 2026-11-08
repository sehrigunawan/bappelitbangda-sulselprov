# SvelteKit frontend rules

- Svelte 5 runes. Use `$derived` for derived values, not `$effect`. No `<slot />`; use snippets. No `createEventDispatcher`; pass callback props. No Svelte 4 stores.
- SvelteKit 3: declare env vars in `src/env.ts` (`defineEnvVars`) and import them from `$app/env/private` or `$app/env/public`. Import hook types from `@sveltejs/kit/hooks`.
- Public pages use universal `load` against the Go API; the admin is an SPA. Keep pages prerender-capable.
- API types come from the generated `src/lib/api/schema.d.ts`; use the typed client in `src/lib/http/client.ts`. Never write response types by hand.
- Tests: component tests are `*.svelte.spec.ts` (vitest browser mode, Chromium); SSR/no-JS tests use `render` from `svelte/server` in plain `*.spec.ts`. Run one with `make test-web FILE=<path>`.
- Interface text is Indonesian (NFR-16). Pages must work without JavaScript for content (NFR-04).
- Run `npm run check` (svelte-check) and `npm run lint` before saying done (or just `make verify`).
