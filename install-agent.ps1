param(
    [String]$AGENTNAME="",
    [String]$SERVERQUERYPORT="15777",
    [String]$BEACONPORT="15000",
    [String]$PORT="7777",
    [String]$SSMURL="",
    [String]$SSMAPIKEY="",
    [Int]$MEMORY=1073741824
)

write-host $AGENTNAME;


$currentPrincipal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
$isAdmin = $currentPrincipal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)

$osInfo = Get-CimInstance -ClassName Win32_OperatingSystem
$isWorkstation = ($osInfo.ProductType -eq 1);

if($SSMURL -eq ""){
    $SSMURL = Read-Host -Prompt 'Enter SSM Cloud URL [https://ssmcloud.hostxtra.co.uk]'

    if ([string]::IsNullOrWhiteSpace($SSMURL))
    {
        $SSMURL = 'https://ssmcloud.hostxtra.co.uk';
    }
}

if($SSMAPIKEY -eq ""){
    $SSMAPIKEY = Read-Host -Prompt 'Enter SSM Cloud API Key [AGT-API-XXXXXXX]'

    if ([string]::IsNullOrWhiteSpace($SSMAPIKEY))
    {
        write-error "You must enter your agent API key!"
        exit 1
    }
}


write-host "* Installing Docker"

if($isWorkstation -eq $false){
    write-error "Windows Server is no longer supported!"
    exit 1;
}else{

    if($osInfo.BuildNumber -gt 19041){
        $Installed = Test-Path("HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\Docker Desktop")

        if($Installed){
            $InstalledDockerVersion = (Get-ItemProperty "HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\Docker Desktop" -ErrorAction SilentlyContinue).DisplayVersion

            $DockerVersionFile = Join-Path $Env:Temp OnlineDockerVersion.xml
            Invoke-WebRequest "https://desktop.docker.com/win/main/amd64/appcast.xml" -OutFile $DockerVersionFile

            $versionInfo = (Select-Xml -Path $DockerVersionFile -XPath "rss/channel/item/title")
            $OnlineVersion = (($versionInfo[$versionInfo.Count -1]).Node.InnerText).Split(" ")[0];

            if($InstalledDockerVersion -ne $OnlineVersion){
                dism.exe /online /enable-feature /featurename:Microsoft-Windows-Subsystem-Linux /all /norestart
                dism.exe /online /enable-feature /featurename:VirtualMachinePlatform /all /norestart

                $WSLInstaller = Join-Path $Env:Temp "WSL2.0.msi"
                Invoke-WebRequest "https://wslstorestorage.blob.core.windows.net/wslblob/wsl_update_x64.msi" -OutFile $WSLInstaller
                msiexec.exe /I $WSLInstaller /quiet
                wsl --set-default-version 2

                sleep -m 3000

                del $WSLInstaller

                    $DockerInstaller = Join-Path $Env:Temp InstallDocker.msi
                Invoke-WebRequest "https://desktop.docker.com/win/main/amd64/Docker%20Desktop%20Installer.exe" -OutFile $DockerInstaller

                cmd /c start /wait $DockerInstaller install --quiet

                sleep -m 3000
                del $DockerInstaller
        
                sleep -m 2000
            }

            $dockerSettingPath = "$($env:APPDATA)\Docker\settings.json"
            $settingsContent = Get-Content $dockerSettingPath -Raw | ConvertFrom-Json
            $settingsContent.exposeDockerAPIOnTCP2375 = $true
            $settingsContent | ConvertTo-Json | Set-Content $dockerSettingPath

            restart-service "com.docker.service"
        }else{
          
            dism.exe /online /enable-feature /featurename:Microsoft-Windows-Subsystem-Linux /all /norestart
            dism.exe /online /enable-feature /featurename:VirtualMachinePlatform /all /norestart

            $WSLInstaller = Join-Path $Env:Temp "WSL2.0.msi"
            Invoke-WebRequest "https://wslstorestorage.blob.core.windows.net/wslblob/wsl_update_x64.msi" -OutFile $WSLInstaller
            msiexec.exe /I $WSLInstaller /quiet
            wsl --set-default-version 2

            sleep -m 3000

            del $WSLInstaller

            $DockerInstaller = Join-Path $Env:Temp InstallDocker.msi
            Invoke-WebRequest "https://desktop.docker.com/win/main/amd64/Docker%20Desktop%20Installer.exe" -OutFile $DockerInstaller

            cmd /c start /wait $DockerInstaller install --quiet

            sleep -m 3000
            del $DockerInstaller
        
            sleep -m 2000

            $dockerSettingPath = "$($env:APPDATA)\Docker\settings.json"
            $settingsContent = Get-Content $dockerSettingPath -Raw | ConvertFrom-Json
            $settingsContent.exposeDockerAPIOnTCP2375 = $true
            $settingsContent | ConvertTo-Json | Set-Content $dockerSettingPath

            restart-service "com.docker.service"
            dism.exe /online /enable-feature /featurename:Microsoft-Windows-Subsystem-Linux /all /norestart
            dism.exe /online /enable-feature /featurename:VirtualMachinePlatform /all /norestart

            $WSLInstaller = Join-Path $Env:Temp "WSL2.0.msi"
            Invoke-WebRequest "https://wslstorestorage.blob.core.windows.net/wslblob/wsl_update_x64.msi" -OutFile $WSLInstaller
            msiexec.exe /I $WSLInstaller /quiet
            wsl --set-default-version 2

            sleep -m 3000

            del $WSLInstaller

                $DockerInstaller = Join-Path $Env:Temp InstallDocker.msi
            Invoke-WebRequest "https://desktop.docker.com/win/main/amd64/Docker%20Desktop%20Installer.exe" -OutFile $DockerInstaller

            cmd /c start /wait $DockerInstaller install --quiet

            sleep -m 3000
            del $DockerInstaller
        
            sleep -m 2000

            $dockerSettingPath = "$($env:APPDATA)\Docker\settings.json"
            $settingsContent = Get-Content $dockerSettingPath -Raw | ConvertFrom-Json
            $settingsContent.exposeDockerAPIOnTCP2375 = $true
            $settingsContent | ConvertTo-Json | Set-Content $dockerSettingPath

            restart-service "com.docker.service"
        }
    }else{
        write-Error "Cant Install docker on this machine! Must be Windows 10 20H2 and Build Number 19041 or higher!"
        exit;
    }
}

write-host "* Docker Installed"


docker run -d `
-e SSM_URL="$($SSMURL)" `
-e SSM_APIKEY="$($SSMAPIKEY)" `
-p "$($SERVERQUERYPORT):15777" `
-p "$($BEACONPORT):15000" `
-p "$($PORT):7777" `
-v "C:\SSMAgent\$($AGENTNAME)\SSM:/home/ssm/SSMAgent" `
-v "C:\SSMAgent\$($AGENTNAME)\.config:/home/ssm/.config/Epic/FactoryGame" `
-m $MEMORY `
--name "$($AGENTNAME)" `
mrhid6/ssmagent:latest