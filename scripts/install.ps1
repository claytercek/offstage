#Requires -Version 5.1
$ErrorActionPreference = "Stop"

$Repo = "claytercek/offstage"
$Binary = "offstage"
$InstallDir = Join-Path $env:LOCALAPPDATA "Programs\offstage"

$Arch = if (
    [System.Runtime.InteropServices.RuntimeInformation]::ProcessArchitecture -eq
    [System.Runtime.InteropServices.Architecture]::Arm64
) { "arm64" } else { "amd64" }

$Release = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest"
$Version = $Release.tag_name
$VersionNoV = $Version.TrimStart("v")

$Filename = "${Binary}_${VersionNoV}_Windows_${Arch}.zip"
$BaseUrl = "https://github.com/$Repo/releases/download/$Version"

$Tmp = Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $Tmp | Out-Null

try {
    Write-Host "Downloading $Binary $Version..."
    Invoke-WebRequest "$BaseUrl/$Filename" -OutFile "$Tmp\$Filename"
    Invoke-WebRequest "$BaseUrl/checksums.txt" -OutFile "$Tmp\checksums.txt"

    Write-Host "Verifying checksum..."
    $Line = Get-Content "$Tmp\checksums.txt" | Where-Object { $_ -match [regex]::Escape($Filename) }
    if (-not $Line) {
        Write-Error "Checksum not found for $Filename"
    }
    $Expected = ($Line -split '\s+')[0]
    $Actual = (Get-FileHash -Algorithm SHA256 "$Tmp\$Filename").Hash.ToLower()
    if ($Expected -ne $Actual) {
        Write-Error "Checksum mismatch`nExpected: $Expected`nActual:   $Actual"
    }

    Expand-Archive "$Tmp\$Filename" -DestinationPath $Tmp

    if (-not (Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir | Out-Null
    }
    Copy-Item -Force "$Tmp\$Binary.exe" "$InstallDir\$Binary.exe"

    $UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($UserPath -notlike "*$InstallDir*") {
        [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
        Write-Host "Added $InstallDir to PATH. Restart your terminal for it to take effect."
    }

    Write-Host "$Binary $Version installed to $InstallDir\$Binary.exe"
} finally {
    Remove-Item -Recurse -Force $Tmp -ErrorAction SilentlyContinue
}
