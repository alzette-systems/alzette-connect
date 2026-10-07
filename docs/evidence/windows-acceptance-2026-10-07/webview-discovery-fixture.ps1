param([string]$Action='hide')
$ErrorActionPreference='Stop'
$backup='C:\AlzetteQA\webview-runtime-registration.json'
if($Action -eq 'hide'){
 if(Test-Path $backup){throw 'Registration backup already exists'}
 $rows=@()
 $ids=@('{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}','{2CD8A007-E189-409D-A2C8-9AF4EF3C72AA}','{0D50BFEC-CD6A-4F9A-964C-C7416E3ACB10}','{65C35B14-6C1D-4122-AC46-7148CC9D6497}')
 foreach($root in @('HKLM:\SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\ClientState','HKCU:\SOFTWARE\Microsoft\EdgeUpdate\ClientState')){
  foreach($id in $ids){$key=Join-Path $root $id;$v=(Get-ItemProperty $key -Name EBWebView -ErrorAction SilentlyContinue).EBWebView;if($v){$rows+=@{key=$key;value=$v}}}
 }
 if($rows.Count -eq 0){throw 'No WebView runtime registration to hide'}
 $rows | ConvertTo-Json | Set-Content -Encoding utf8 $backup
 foreach($row in $rows){Set-ItemProperty -Path $row.key -Name EBWebView -Value ''}
 "Temporarily hid $($rows.Count) runtime registration(s); binaries remain installed."
}else{
 foreach($row in @(Get-Content $backup -Raw | ConvertFrom-Json)){Set-ItemProperty -Path $row.key -Name EBWebView -Value $row.value}
 Remove-Item $backup
 'Runtime registration restored.'
}
