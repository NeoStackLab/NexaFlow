# NexaFlow Web

The separated Next.js frontend for NexaFlow. Project-level setup, architecture,
and deployment instructions live in the repository root `README.md` and `docs/`.

```powershell
Copy-Item .env.example .env.local
corepack pnpm install --frozen-lockfile
corepack pnpm dev
```

Quality checks:

```powershell
corepack pnpm lint
corepack pnpm typecheck
corepack pnpm build
```
