param(
    [string]$BinDir = "$HOME\.local\bin",
    [switch]$InstallOnly
)

$ErrorActionPreference = "Stop"

$SkillDir = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$BinaryName = "zendesk-mgmt.exe"
$BuildOutput = Join-Path $SkillDir $BinaryName
$AgentsDest = Join-Path $HOME ".agents\skills\zendesk-management"
$ClaudeDest = Join-Path $HOME ".claude\skills\zendesk-management"
$CodexDest = Join-Path $HOME ".codex\skills\zendesk-management"
$ConfigDir = Join-Path ([Environment]::GetFolderPath("ApplicationData")) "zendesk-mgmt"
$InstallStatePath = Join-Path $ConfigDir "install.json"
$BuildVersion = "dev"
$BuildCommit = "unknown"
$BuildDate = [DateTime]::UtcNow.ToString("yyyy-MM-ddTHH:mm:ssZ")
$BuildLdflags = ""

function Write-Info([string]$Message) {
    Write-Host $Message -ForegroundColor Green
}

function Write-Warn([string]$Message) {
    Write-Host $Message -ForegroundColor Yellow
}

function Ensure-Go {
    if (Get-Command go -ErrorAction SilentlyContinue) {
        Write-Info "Go already installed: $(go version)"
        return
    }

    if (-not (Get-Command winget -ErrorAction SilentlyContinue)) {
        throw "Go is missing and winget is not available. Install Go first."
    }

    Write-Warn "Go not found. Installing via winget..."
    winget install --exact --id GoLang.Go --accept-package-agreements --accept-source-agreements
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        throw "Go install completed but go is still not on PATH. Restart the shell and rerun setup."
    }
    Write-Info "Go installed: $(go version)"
}

function Get-VersionMetadata {
    if (Get-Command git -ErrorAction SilentlyContinue) {
        try {
            Push-Location $SkillDir
            try {
                $script:BuildVersion = (git describe --tags --always 2>$null)
                if (-not $script:BuildVersion) {
                    $script:BuildVersion = "dev"
                }

                $script:BuildCommit = (git rev-parse --short HEAD 2>$null)
                if (-not $script:BuildCommit) {
                    $script:BuildCommit = "unknown"
                }
            }
            finally {
                Pop-Location
            }
        }
        catch {
            $script:BuildVersion = "dev"
            $script:BuildCommit = "unknown"
        }
    }

    $script:BuildLdflags = "-X main.Version=$script:BuildVersion -X main.Commit=$script:BuildCommit -X main.BuildDate=$script:BuildDate"
}

function Build-Cli {
    Write-Info "Building $BinaryName ..."
    Push-Location $SkillDir
    try {
        go build -trimpath -ldflags $BuildLdflags -o $BuildOutput ./cmd/zendesk-mgmt
    }
    finally {
        Pop-Location
    }
    Write-Info "Built: $BuildOutput"
}

function Install-Binary {
    New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
    Copy-Item $BuildOutput (Join-Path $BinDir $BinaryName) -Force
    Write-Info "Installed binary: $(Join-Path $BinDir $BinaryName)"
}

function Scrub-GitMetadata([string]$Path) {
    @(".git", ".gitignore", ".gitattributes", ".gitmodules") | ForEach-Object {
        $Target = Join-Path $Path $_
        if (Test-Path $Target) {
            Remove-Item $Target -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
}

function Install-SkillArtifact {
    $Exclude = @(".git", ".task-board", ".agents", ".claude", ".codex", ".local", "dist", "zendesk-mgmt", "zendesk-mgmt.exe")
    if (Test-Path $AgentsDest) {
        Remove-Item $AgentsDest -Recurse -Force
    }
    New-Item -ItemType Directory -Force -Path $AgentsDest | Out-Null

    Get-ChildItem -LiteralPath $SkillDir -Force | Where-Object { $Exclude -notcontains $_.Name } | ForEach-Object {
        Copy-Item $_.FullName -Destination $AgentsDest -Recurse -Force
    }

    Scrub-GitMetadata $AgentsDest
    Write-Info "Installed skill artifact: $AgentsDest"
}

function New-DirLink([string]$LinkPath, [string]$TargetPath) {
    $Parent = Split-Path -Parent $LinkPath
    New-Item -ItemType Directory -Force -Path $Parent | Out-Null
    if (Test-Path $LinkPath) {
        Remove-Item $LinkPath -Recurse -Force
    }
    New-Item -ItemType Junction -Path $LinkPath -Target $TargetPath | Out-Null
}

function Refresh-Links {
    New-DirLink $ClaudeDest $AgentsDest
    New-DirLink $CodexDest $AgentsDest
    Write-Info "Refreshed Claude/Codex skill links"
}

function Write-InstallState {
    New-Item -ItemType Directory -Force -Path $ConfigDir | Out-Null
    $Payload = @{
        repoPath = $SkillDir
        installedSkillPath = $AgentsDest
        binDir = $BinDir
        platform = "windows"
        arch = [System.Runtime.InteropServices.RuntimeInformation]::ProcessArchitecture.ToString().ToLowerInvariant()
        version = $BuildVersion
        commit = $BuildCommit
        buildDate = $BuildDate
        installOnly = [bool]$InstallOnly
    } | ConvertTo-Json
    Set-Content -Path $InstallStatePath -Value $Payload
    Write-Info "Install state: $InstallStatePath"
}

function Ensure-UserPath {
    $CurrentUserPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $Parts = @()
    if ($CurrentUserPath) {
        $Parts = $CurrentUserPath -split ';'
    }

    if ($Parts -notcontains $BinDir) {
        $NewPath = (($Parts + $BinDir) | Where-Object { $_ -and $_.Trim() -ne "" } | Select-Object -Unique) -join ';'
        [Environment]::SetEnvironmentVariable("Path", $NewPath, "User")
        Write-Warn "Added $BinDir to the user PATH. Restart the shell if needed."
    }

    if (($env:Path -split ';') -notcontains $BinDir) {
        $env:Path = "$BinDir;$env:Path"
    }
}

function Verify-Install {
    $InstalledBinary = Join-Path $BinDir $BinaryName
    if (-not (Test-Path $InstalledBinary)) {
        throw "Missing installed binary: $InstalledBinary"
    }
    if (-not (Test-Path (Join-Path $AgentsDest "SKILL.md"))) {
        throw "Installed skill artifact is missing SKILL.md"
    }

    & $InstalledBinary version | Out-Null
    & $InstalledBinary auth config-path | Out-Null
    Write-Info "Verified binary and skill artifact"
}

Write-Host ""
Write-Info "=== zendesk-management setup ==="
Write-Host ""
if ($InstallOnly) {
    Write-Warn "Running safe reinstall flow (--InstallOnly)"
}

Ensure-Go
Get-VersionMetadata
Build-Cli
Install-Binary
Install-SkillArtifact
Refresh-Links
Write-InstallState
Ensure-UserPath
Verify-Install

Write-Host ""
Write-Info "=== Done ==="
