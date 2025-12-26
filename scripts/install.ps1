# aicli installer for Windows
# Run: irm https://raw.githubusercontent.com/glennswest/aicli/main/scripts/install.ps1 | iex
# Or: powershell -ExecutionPolicy Bypass -File install.ps1

$ErrorActionPreference = "Stop"

$Repo = "glennswest/aicli"
$BinaryName = "aicli.exe"
$InstallDir = "$env:LOCALAPPDATA\aicli"

function Write-Info { param($Message) Write-Host "==> " -ForegroundColor Green -NoNewline; Write-Host $Message }
function Write-Warn { param($Message) Write-Host "==> " -ForegroundColor Yellow -NoNewline; Write-Host $Message }
function Write-Err { param($Message) Write-Host "==> " -ForegroundColor Red -NoNewline; Write-Host $Message; exit 1 }

Write-Host ""
Write-Host "  aicli installer"
Write-Host "  ==============="
Write-Host ""

# Get latest version
Write-Info "Checking for latest release..."
try {
    $Release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
    $Version = $Release.tag_name
    Write-Info "Latest version: $Version"
} catch {
    Write-Err "Failed to get latest version: $_"
}

# Download
$Platform = "windows-amd64"
$DownloadUrl = "https://github.com/$Repo/releases/download/$Version/aicli-$Platform.zip"
$TempDir = Join-Path $env:TEMP "aicli-install"
$ZipFile = Join-Path $TempDir "aicli.zip"

Write-Info "Downloading aicli-$Platform.zip..."

# Create temp directory
if (Test-Path $TempDir) { Remove-Item $TempDir -Recurse -Force }
New-Item -ItemType Directory -Path $TempDir -Force | Out-Null

try {
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $ZipFile -UseBasicParsing
} catch {
    Write-Err "Failed to download: $_"
}

# Extract
Write-Info "Extracting..."
Expand-Archive -Path $ZipFile -DestinationPath $TempDir -Force

# Install
Write-Info "Installing to $InstallDir..."
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

Move-Item -Path (Join-Path $TempDir $BinaryName) -Destination (Join-Path $InstallDir $BinaryName) -Force

# Cleanup
Remove-Item $TempDir -Recurse -Force

# Add to PATH if not already there
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    Write-Info "Adding $InstallDir to PATH..."
    [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
    Write-Warn "Please restart your terminal for PATH changes to take effect."
}

Write-Host ""
Write-Info "Installation complete!"
Write-Host ""
Write-Host "  Run 'aicli --init' to create default config"
Write-Host "  Run 'aicli --help' for usage"
Write-Host ""
