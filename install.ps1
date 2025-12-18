<#
.SYNOPSIS
    Installs OCI ARM Provisioner on Windows.

.DESCRIPTION
    Builds (optional) and installs the binary and configuration files.
    Can also register a Scheduled Task for background execution.

.PARAMETER Uninstall
    If set, removes the application and scheduled task.

.EXAMPLE
    .\install.ps1
    Installs the application.

.EXAMPLE
    .\install.ps1 -Uninstall
    Uninstalls the application.
#>

param (
    [switch]$Uninstall
)

$AppName = "oci-arm-provisioner"
$InstallDir = "$env:ProgramFiles\$AppName"
$ConfigDir = "$env:ProgramData\$AppName"
$BinaryName = "$AppName.exe"
$TaskName = "OCI-ARM-Provisioner-Auto"

function Test-Admin {
    $currentPrincipal = [Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()
    return $currentPrincipal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

if (-not (Test-Admin)) {
    Write-Warning "Please run this script as Administrator."
    exit 1
}

# UNINSTALL MODE
if ($Uninstall) {
    Write-Host "🗑️  Uninstalling $AppName..." -ForegroundColor Cyan

    # Stop and delete task
    if (Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue) {
        Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false
        Write-Host "   Removed Scheduled Task."
    }

    # Delete files
    if (Test-Path $InstallDir) {
        Remove-Item -Path $InstallDir -Recurse -Force
        Write-Host "   Removed Install Directory."
    }

    # Verify Config
    Write-Host "❓ Remove configuration? ($ConfigDir)"
    $response = Read-Host "   (y/N)"
    if ($response -match "^y") {
        Remove-Item -Path $ConfigDir -Recurse -Force -ErrorAction SilentlyContinue
        Write-Host "   Removed Config Directory."
    } else {
        Write-Host "   Kept Config Directory."
    }


    # Remove from PATH (User Scope)
    $CurrentPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($CurrentPath -like "*$InstallDir*") {
        $NewPath = $CurrentPath.Replace("$InstallDir;", "").Replace(";$InstallDir", "").Replace("$InstallDir", "")
        [Environment]::SetEnvironmentVariable("Path", $NewPath, "User")
        Write-Host "   Removed from User PATH."
    }

    Write-Host "✅ Uninstallation Complete!" -ForegroundColor Green
    return
}

# INSTALL MODE
Write-Host "🚀 Installing $AppName..." -ForegroundColor Cyan

# 1. Prepare/Build Binary
if (-not (Test-Path $BinaryName)) {
    Write-Host "📦 Binary not found. Attempting build..."
    if (Get-Command "go" -ErrorAction SilentlyContinue) {
        go build -ldflags="-s -w" -o $BinaryName
    } else {
        Write-Error "Go not found and binary missing. Please install Go or download the release zip."
        exit 1
    }
}

# 2. Create Directories
if (-not (Test-Path $InstallDir)) {
    New-Item -Path $InstallDir -ItemType Directory | Out-Null
}
if (-not (Test-Path $ConfigDir)) {
    New-Item -Path $ConfigDir -ItemType Directory | Out-Null
}

# 3. Copy Files
Copy-Item -Path $BinaryName -Destination "$InstallDir\$BinaryName" -Force
Write-Host "📂 Installed binary to $InstallDir"

if (-not (Test-Path "$ConfigDir\config.yaml")) {
    if (Test-Path "config.yaml.example") {
        Copy-Item -Path "config.yaml.example" -Destination "$ConfigDir\config.yaml.example"
        Write-Host "⚠️  Created config example at $ConfigDir\config.yaml.example"
        Write-Host "   Please rename it to config.yaml and edit it!" -ForegroundColor Yellow
    }
}

# 4. Add to PATH (User Scope)
$CurrentPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($CurrentPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$CurrentPath;$InstallDir", "User")
    Write-Host "🔗 Added $InstallDir to User PATH."
    Write-Host "   (You may need to restart your terminal)" -ForegroundColor Yellow
} else {
    Write-Host "🔗 Already in PATH."
}

# 5. Optional Task Scheduler
Write-Host "`n🔧 Do you want to create a background Scheduled Task?"
Write-Host "   This will run the provisioner automatically at system startup."
$response = Read-Host "   Create Task? (y/N)"

if ($response -match "^y") {
    $action = New-ScheduledTaskAction -Execute "$InstallDir\$BinaryName" -WorkingDirectory $InstallDir
    $trigger = New-ScheduledTaskTrigger -AtLogon
    $principal = New-ScheduledTaskPrincipal -UserId "SYSTEM" -LogonType ServiceAccount -RunLevel Highest
    # Note: Running as SYSTEM means it won't see user mapped drives or keys in current user registry.
    # But for OCI keys in a fixed path, it's fine.
    
    Register-ScheduledTask -Action $action -Trigger $trigger -Principal $principal -TaskName $TaskName -Description "Runs OCI ARM Provisioner" -Force | Out-Null
    Write-Host "✅ Scheduled Task '$TaskName' created."
}

Write-Host "`n✅ Installation Complete!" -ForegroundColor Green
Write-Host "👉 Run from CLI: oci-arm-provisioner (Restart terminal if not found)"
