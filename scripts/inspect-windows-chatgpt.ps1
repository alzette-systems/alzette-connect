param([string]$OutputPath = "$env:USERPROFILE\Desktop\alzette-windows-chatgpt-inspection.json")

# Read-only inspection. Do not collect user profiles, tokens, or process environments.
$ErrorActionPreference = 'Stop'
$os = Get-CimInstance Win32_OperatingSystem
$packages = @(Get-AppxPackage | Where-Object {
    $_.Name -match 'OpenAI|ChatGPT|Codex' -or $_.Publisher -match 'OpenAI'
})
$observed = @($packages | ForEach-Object {
    $package = $_
    $manifest = Get-AppxPackageManifest -Package $package.PackageFullName
    $applications = @($manifest.Package.Applications.Application | ForEach-Object {
        [ordered]@{
            application_id = [string]$_.Id
            executable = [string]$_.Executable
            entry_point = [string]$_.EntryPoint
            activation_id = $package.PackageFamilyName + '!' + $_.Id
        }
    })
    [ordered]@{
        name = $package.Name
        version = [string]$package.Version
        architecture = [string]$package.Architecture
        package_full_name = $package.PackageFullName
        package_family_name = $package.PackageFamilyName
        publisher = $package.Publisher
        publisher_id = $package.PublisherId
        signature_kind = [string]$package.SignatureKind
        status = [string]$package.Status
        install_location = $package.InstallLocation
        applications = $applications
    }
})
$report = [ordered]@{
    observed_at = [DateTime]::UtcNow.ToString('o')
    windows = [ordered]@{ caption = $os.Caption; version = $os.Version; build = $os.BuildNumber; architecture = $os.OSArchitecture }
    microsoft_store_installed = [bool](Get-AppxPackage Microsoft.WindowsStore)
    app_installer_installed = [bool](Get-AppxPackage Microsoft.DesktopAppInstaller)
    packages = $observed
    qualification = 'Inspection only; native launch and model compatibility have not been verified.'
}
$report | ConvertTo-Json -Depth 8 | Set-Content -Encoding UTF8 -LiteralPath $OutputPath
Write-Output "Inspection saved to $OutputPath"
