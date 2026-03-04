$ErrorActionPreference = 'Stop'

$installDir = "$env:ProgramFiles\TranscodeFactory"
$exePath = "$installDir\transcode-factory.exe"
$dataPath = "$env:ProgramData\TranscodeFactory"

Write-Output 'Stopping Service...'
Get-Service TranscodeFactory | Stop-Service -Force -ErrorAction SilentlyContinue
Write-Output 'Removing Service...'
& $exePath --service uninstall
Write-Output 'Removing Files...'
Get-Item $installDir | Remove-Item -Recurse -Force
Get-Item $dataPath  | Remove-Item -Recurse -Force