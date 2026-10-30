# Builds mpx-hello.exe. Run from a "Developer PowerShell for VS" (so cl.exe is on PATH).
#   .\build.ps1
$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot
New-Item -ItemType Directory -Force -Path build | Out-Null
cl.exe /nologo /std:c++20 /EHsc /MT /O2 /W4 /permissive- /DUNICODE /D_UNICODE /DWIN32_LEAN_AND_MEAN /DNOMINMAX `
    mpx-hello.cpp /Fo:build\ /Fe:build\mpx-hello.exe `
    /link windowsapp.lib user32.lib
if ($LASTEXITCODE -ne 0) { throw "cl.exe failed with exit code $LASTEXITCODE" }
Write-Host "Built $PSScriptRoot\build\mpx-hello.exe"
