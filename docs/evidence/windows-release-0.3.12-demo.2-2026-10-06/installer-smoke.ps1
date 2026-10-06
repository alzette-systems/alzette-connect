param([string]$Installer, [string]$Phase = 'install')
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$version = '0.3.12-demo.2'
$out = 'C:\AlzetteQA\release-0.3.12-demo.2'
New-Item -ItemType Directory -Path $out -Force | Out-Null
$install = Join-Path $env:LOCALAPPDATA 'Programs\Alzette Connect'
$exe = Join-Path $install 'alzette-connect.exe'
$key = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\AlzetteConnect'
function Snapshot {
    $files = @((Join-Path $env:USERPROFILE '.codex\config.toml'), (Join-Path $env:LOCALAPPDATA 'Claude-3p\configLibrary\_meta.json'))
    $result = @{}
    foreach ($file in $files) {
        if (Test-Path -LiteralPath $file) { $result[$file] = (Get-FileHash -LiteralPath $file -Algorithm SHA256).Hash }
        else { $result[$file] = 'absent' }
    }
    return $result
}
if ($Phase -eq 'install') {
    $baseline = Snapshot
    $previous = (Get-ItemProperty -LiteralPath $key -ErrorAction SilentlyContinue).DisplayVersion
    $baseline | ConvertTo-Json | Set-Content -Encoding utf8 (Join-Path $out 'profile-baseline.json')
    $p = Start-Process -FilePath $Installer -ArgumentList '/S' -Wait -PassThru
    if ($p.ExitCode -ne 0) { throw "Installer exit: $($p.ExitCode)" }
    $registered = Get-ItemProperty -LiteralPath $key
    if ($registered.DisplayVersion -ne $version) { throw 'Unexpected installed version' }
    foreach ($file in @('alzette-connect.exe', 'Uninstall.exe', 'LICENSE.txt', 'NOTICE.txt', 'THIRD_PARTY_NOTICES.md', 'UNSIGNED-DEMO.txt')) {
        if (-not (Test-Path -LiteralPath (Join-Path $install $file))) { throw "Missing installed file: $file" }
    }
    $bytes = [System.IO.File]::ReadAllBytes($exe)
    $pe = [BitConverter]::ToInt32($bytes, 0x3c)
    $subsystem = [BitConverter]::ToUInt16($bytes, $pe + 4 + 20 + 68)
    if ($subsystem -ne 2) { throw 'Application is not a Windows GUI executable' }
    $after = Snapshot
    foreach ($file in $baseline.Keys) { if ($baseline[$file] -ne $after[$file]) { throw 'Installer changed an application profile' } }
    if (@(Get-Process -Name 'alzette-connect' -ErrorAction SilentlyContinue).Count -ne 0) { throw 'Silent installer started Connect' }
    $notice = [string](Get-Content -LiteralPath (Join-Path $install 'UNSIGNED-DEMO.txt') -Raw -Encoding UTF8)
    [ordered]@{version=$registered.DisplayVersion;upgraded_from=$previous;pe_subsystem=$subsystem;installer_exit=$p.ExitCode;per_user_install=$true;installer_sha256=(Get-FileHash -LiteralPath $Installer -Algorithm SHA256).Hash.ToLowerInvariant();application_sha256=(Get-FileHash -LiteralPath $exe -Algorithm SHA256).Hash.ToLowerInvariant();signature=(Get-AuthenticodeSignature -LiteralPath $Installer).Status.ToString();required_files_present=$true;client_profiles_unchanged=$true;automatic_launch=$false;notice=$notice} | ConvertTo-Json | Set-Content -Encoding utf8 (Join-Path $out 'install-report.json')
}
elseif ($Phase -eq 'reinstall') {
    $p = Start-Process -FilePath $Installer -ArgumentList '/S' -Wait -PassThru
    if ($p.ExitCode -ne 0) { throw "Reinstall exit: $($p.ExitCode)" }
    if (-not (Test-Path -LiteralPath $exe)) { throw 'Application missing after reinstall' }
    [ordered]@{reinstall_exit=$p.ExitCode;version=(Get-ItemProperty -LiteralPath $key).DisplayVersion} | ConvertTo-Json | Set-Content -Encoding utf8 (Join-Path $out 'reinstall-report.json')
}
elseif ($Phase -eq 'uninstall') {
    if (@(Get-Process -Name 'alzette-connect' -ErrorAction SilentlyContinue).Count -ne 0) { throw 'Quit Connect before uninstall' }
    $sentinel = Join-Path $env:APPDATA 'alzette-connect-release-qa\preserve.txt'
    New-Item -ItemType Directory -Path (Split-Path $sentinel) -Force | Out-Null
    Set-Content -Encoding ascii -LiteralPath $sentinel -Value 'PRESERVE_USER_DATA'
    $p = Start-Process -FilePath (Join-Path $install 'Uninstall.exe') -ArgumentList '/S' -Wait -PassThru
    Start-Sleep -Seconds 2
    if ($p.ExitCode -ne 0 -or (Test-Path -LiteralPath $exe) -or (Test-Path -LiteralPath $key)) { throw 'Uninstall did not remove application and registration' }
    $before = Get-Content -LiteralPath (Join-Path $out 'profile-baseline.json') -Raw | ConvertFrom-Json
    $after = Snapshot
    foreach ($property in $before.PSObject.Properties) { if ($property.Value -ne $after[$property.Name]) { throw 'Client profile changed during package acceptance' } }
    if (-not (Test-Path -LiteralPath $sentinel)) { throw 'Uninstaller removed external user data' }
    Remove-Item -LiteralPath (Split-Path $sentinel) -Recurse
    [ordered]@{uninstall_exit=$p.ExitCode;application_removed=$true;registration_removed=$true;external_user_data_preserved=$true;client_profiles_unchanged=$true} | ConvertTo-Json | Set-Content -Encoding utf8 (Join-Path $out 'uninstall-report.json')
}
