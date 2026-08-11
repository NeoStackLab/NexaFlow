$ErrorActionPreference = "Stop"

$repositoryRoot = Split-Path -Parent $PSScriptRoot

Push-Location (Join-Path $repositoryRoot "backend")
try {
    go test ./...
    go vet ./...
} finally {
    Pop-Location
}

Push-Location (Join-Path $repositoryRoot "frontend")
try {
    corepack pnpm lint
    corepack pnpm typecheck
    corepack pnpm build
} finally {
    Pop-Location
}

if (Get-Command docker -ErrorAction SilentlyContinue) {
    docker compose -f (Join-Path $repositoryRoot "docker\compose.yaml") config --quiet
} else {
    Write-Warning "Docker CLI not found; skipped Compose validation."
}

Write-Output "NexaFlow verification passed."
