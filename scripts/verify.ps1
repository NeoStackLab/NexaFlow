$ErrorActionPreference = "Stop"

$repositoryRoot = Split-Path -Parent $PSScriptRoot

function Invoke-NativeChecked {
    param(
        [Parameter(Mandatory = $true)]
        [scriptblock]$Command,
        [Parameter(Mandatory = $true)]
        [string]$Description
    )

    & $Command
    if ($LASTEXITCODE -ne 0) {
        throw "$Description failed with exit code $LASTEXITCODE."
    }
}

Push-Location (Join-Path $repositoryRoot "backend")
try {
    Invoke-NativeChecked { go test ./... } "Backend tests"
    Invoke-NativeChecked { go vet ./... } "Backend vet"
} finally {
    Pop-Location
}

Push-Location (Join-Path $repositoryRoot "frontend")
try {
    Invoke-NativeChecked { corepack pnpm lint } "Frontend lint"
    Invoke-NativeChecked { corepack pnpm typecheck } "Frontend typecheck"
    Invoke-NativeChecked { corepack pnpm build } "Frontend build"
} finally {
    Pop-Location
}

if (Get-Command docker -ErrorAction SilentlyContinue) {
    Invoke-NativeChecked { docker compose -f (Join-Path $repositoryRoot "docker\compose.yaml") config --quiet } "Docker Compose validation"
} else {
    Write-Warning "Docker CLI not found; skipped Compose validation."
}

Write-Output "NexaFlow verification passed."
