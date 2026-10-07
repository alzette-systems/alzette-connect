$ErrorActionPreference='Stop'
$r=[ordered]@{identity=[System.Security.Principal.WindowsIdentity]::GetCurrent().Name;administrator=([System.Security.Principal.WindowsPrincipal][System.Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([System.Security.Principal.WindowsBuiltInRole]::Administrator)}
try {
 if($r.administrator){throw 'Expected a standard-user token'}
 $install=Join-Path $env:LOCALAPPDATA 'Programs\Alzette Connect';$exe=Join-Path $install 'alzette-connect.exe';$reg='HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\AlzetteConnect'
 $files=@((Join-Path $env:USERPROFILE '.codex\config.toml'),(Join-Path $env:LOCALAPPDATA 'Claude-3p\configLibrary\_meta.json'),(Join-Path $env:APPDATA 'alzette-qa\preserve.txt'))
 foreach($f in $files){New-Item (Split-Path $f) -ItemType Directory -Force | Out-Null;[IO.File]::WriteAllText($f,'STANDARD_USER_PRESERVE')}
 $before=@{};foreach($f in $files){$before[$f]=(Get-FileHash $f).Hash}
 foreach($v in @('0.3.12-demo.1','0.3.12-demo.2','0.3.12-demo.2')) {
  $p=Start-Process "C:\AlzetteQA\Alzette-Connect-$v-windows-x64-unsigned-demo.exe" -ArgumentList '/S' -Wait -PassThru
  if($p.ExitCode -ne 0 -or -not (Test-Path $exe) -or (Get-ItemProperty $reg).DisplayVersion -ne $v){throw "Install failed: $v / $($p.ExitCode)"}
  $r["installed_$v"]=$true
 }
 $r.per_user_path=$install
 $r.installer_signature=(Get-AuthenticodeSignature 'C:\AlzetteQA\Alzette-Connect-0.3.12-demo.2-windows-x64-unsigned-demo.exe').Status.ToString()
 $r.executable_signature=(Get-AuthenticodeSignature $exe).Status.ToString()
 $bytes=[IO.File]::ReadAllBytes($exe);$pe=[BitConverter]::ToInt32($bytes,0x3c);$r.pe_subsystem=[BitConverter]::ToUInt16($bytes,$pe+4+20+68)
 $p=Start-Process (Join-Path $install 'Uninstall.exe') -ArgumentList '/S' -Wait -PassThru
 Start-Sleep 3
 $r.uninstall_exit=$p.ExitCode;$r.application_removed=-not(Test-Path $exe);$r.registration_removed=-not(Test-Path $reg)
 $r.external_profiles_and_data_preserved=$true;foreach($f in $files){if(-not(Test-Path $f) -or (Get-FileHash $f).Hash -ne $before[$f]){$r.external_profiles_and_data_preserved=$false}}
 if(-not $r.application_removed -or -not $r.registration_removed -or -not $r.external_profiles_and_data_preserved){throw 'Uninstall or profile preservation failed'}
 $r.passed=$true
}catch{$r.passed=$false;$r.error=$_.Exception.Message}
$r | ConvertTo-Json | Set-Content -Encoding utf8 (Join-Path $env:LOCALAPPDATA 'alzette-standard-acceptance.json')
