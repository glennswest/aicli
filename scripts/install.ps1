# aicli installer for Windows
# Run as Administrator: powershell -ExecutionPolicy Bypass -File install.ps1

$ErrorActionPreference = "Stop"

$BinaryName = "aicli.exe"
$InstallDir = "$env:LOCALAPPDATA\aicli"

Write-Host "Installing aicli to $InstallDir"

# Create install directory
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

# Find binary
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$BinaryPath = $null

if (Test-Path "$ScriptDir\windows-amd64\$BinaryName") {
    $BinaryPath = "$ScriptDir\windows-amd64\$BinaryName"
} elseif (Test-Path "$ScriptDir\..\dist\windows-amd64\$BinaryName") {
    $BinaryPath = "$ScriptDir\..\dist\windows-amd64\$BinaryName"
}

if (-not $BinaryPath) {
    Write-Error "Binary not found. Please build first with: make windows-amd64"
    exit 1
}

# Copy binary
Copy-Item $BinaryPath "$InstallDir\$BinaryName" -Force

# Add to PATH if not already there
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    Write-Host "Adding $InstallDir to PATH..."
    [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
    Write-Host "Please restart your terminal for PATH changes to take effect."
}

Write-Host ""
Write-Host "Installation complete!"
Write-Host "Run 'aicli --init' to create default config"
Write-Host "Run 'aicli --help' for usage"
