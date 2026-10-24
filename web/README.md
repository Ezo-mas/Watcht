# watcht web

The watcht frontend: a SvelteKit single-page app, built to static files and
embedded into the Go binary. Conventions and commands are in
[docs/agent-guides/frontend.md](../docs/agent-guides/frontend.md).

```bash
pnpm install --frozen-lockfile
pnpm dev     # Vite on :5173, proxying /api and /healthz to Go on :8080
pnpm check   # svelte-check
pnpm lint    # prettier and eslint
pnpm test    # vitest
pnpm build   # static build in build/, embedded by `make build`
```
