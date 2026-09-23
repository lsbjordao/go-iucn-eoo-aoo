param(
    [string]$Version = "latest",
    [string]$InstallDir = "$env:LOCALAPPDATA\Programs\eoo-aoo"
)

$ErrorActionPreference = "Stop"
$Repo = if ($env:EOO_AOO_REPO) { $env:EOO_AOO_REPO } else { "lsbjordao/go-iucn-eoo-aoo" }
$Asset = "eoo-aoo-windows-amd64.zip"

if (-not [Environment]::Is64BitOperatingSystem) {
    throw "The prebuilt Windows release requires 64-bit Windows."
}

if ($Version -eq "latest") {
    $Base = "https://github.com/$Repo/releases/latest/download"
} else {
    $Tag = if ($Version.StartsWith("v")) { $Version } else { "v$Version" }
    $Base = "https://github.com/$Repo/releases/download/$Tag"
}

$Headers = @{}
if ($env:GITHUB_TOKEN) {
    $Headers["Authorization"] = "Bearer $env:GITHUB_TOKEN"
    $Headers["X-GitHub-Api-Version"] = "2022-11-28"
}

$Temp = Join-Path ([IO.Path]::GetTempPath()) ("eoo-aoo-" + [guid]::NewGuid().ToString("N"))
$Zip = Join-Path $Temp $Asset
$Checksums = Join-Path $Temp "SHA256SUMS"
$Stage = Join-Path $Temp "stage"

try {
    New-Item -ItemType Directory -Force -Path $Temp, $Stage | Out-Null

    Write-Host "==> Downloading $Asset"
    Invoke-WebRequest -Headers $Headers -Uri "$Base/$Asset" -OutFile $Zip
    Invoke-WebRequest -Headers $Headers -Uri "$Base/SHA256SUMS" -OutFile $Checksums

    $Line = Get-Content $Checksums | Where-Object { $_.Trim().EndsWith($Asset, [StringComparison]::Ordinal) } | Select-Object -First 1
    if (-not $Line) { throw "Checksum for $Asset was not found." }
    $Expected = ($Line -split '\s+')[0].ToLowerInvariant()
    $Actual = (Get-FileHash -Algorithm SHA256 $Zip).Hash.ToLowerInvariant()
    if ($Actual -ne $Expected) { throw "SHA-256 mismatch for $Asset." }

    Expand-Archive -Path $Zip -DestinationPath $Stage -Force
    $Root = Get-ChildItem -Path $Stage -Directory | Select-Object -First 1
    if (-not $Root) { throw "Release archive has an unexpected layout." }
    $Exe = Join-Path $Root.FullName "eoo-aoo.exe"
    if (-not (Test-Path $Exe)) { throw "Release archive does not contain eoo-aoo.exe." }

    if (Test-Path $InstallDir) {
        Remove-Item -Recurse -Force $InstallDir
    }
    New-Item -ItemType Directory -Force -Path (Split-Path $InstallDir -Parent) | Out-Null
    Move-Item -Path $Root.FullName -Destination $InstallDir

    $UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $PathEntries = @($UserPath -split ';' | Where-Object { $_ })
    if ($PathEntries -notcontains $InstallDir) {
        $NewPath = (($PathEntries + $InstallDir) -join ';')
        [Environment]::SetEnvironmentVariable("Path", $NewPath, "User")
        Write-Host "==> Added $InstallDir to your user PATH"
    }
    if (($env:Path -split ';') -notcontains $InstallDir) {
        $env:Path = "$InstallDir;$env:Path"
    }

    Write-Host "==> Installed eoo-aoo to $InstallDir"
    & (Join-Path $InstallDir "eoo-aoo.exe") version
    Write-Host "==> Installation complete; no Go compiler, MSYS2, or source build was required"
}
finally {
    if (Test-Path $Temp) { Remove-Item -Recurse -Force $Temp }
}
