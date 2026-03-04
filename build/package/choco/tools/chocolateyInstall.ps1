$ErrorActionPreference = 'Stop'

$toolsDir = "$(Split-Path -parent $MyInvocation.MyCommand.Definition)"
$installDir = "$env:ProgramFiles\TranscodeFactory"

# Create installation directory
if (-not (Test-Path $installDir)) {
  New-Item -ItemType Directory -Path $installDir -Force | Out-Null
}

# Stop running service if it exists
$service = Get-Service "TranscodeFactory" -ErrorAction SilentlyContinue
if ($service) {
  Write-Host 'Stopping existing TranscodeFactory service...'
  Stop-Service -Name $serviceName -Force
  Start-Sleep -Seconds 2
}

# Copy executable
Write-Host 'Copying executable...'
Copy-Item -Path "$toolsDir\transcode-factory.exe" -Destination "$installDir\transcode-factory.exe" -Force

# Install as Windows service
if (-not $service) {
  Write-Host 'Installing TranscodeFactory as a Windows service...'
  & "$installDir\transcode-factory.exe" --service install
  if ($LASTEXITCODE) {
    Write-Error 'Failed to install service'
    exit 1
  }
}

# Start service
Write-Host 'Starting TranscodeFactory service...'
Start-Service TranscodeFactory

Write-Host "Transcode Factory installed successfully to $installDir"