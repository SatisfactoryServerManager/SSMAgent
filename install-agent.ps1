param(
    [String]$AGENTNAME="",
    [Int32]$PORTOFFSET=0,
    [String]$SSMURL="",
    [String]$SSMAPIKEY="",
    [String]$MEMORY=1073741824,
    [switch]$NoDockerInstall = $false
)


$currentPrincipal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
$isAdmin = $currentPrincipal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)

$osInfo = Get-CimInstance -ClassName Win32_OperatingSystem
$isWorkstation = ($osInfo.ProductType -eq 1);

$DOCKERTEST= docker ps -a -q -f name=$AGENTNAME
$DOCKEREXISTS=![string]::IsNullOrWhiteSpace($DOCKERTEST)


$SERVERQUERYPORT=15777 + $PORTOFFSET;
$BEACONPORT = 15000 + $PORTOFFSET;
$PORT = 7777 + $PORTOFFSET;

if($SSMURL -eq ""){
    $SSMURL = Read-Host -Prompt 'Enter SSM Cloud URL [https://api-ssmcloud.hostxtra.co.uk]'

    if ([string]::IsNullOrWhiteSpace($SSMURL))
    {
        $SSMURL = 'https://api-ssmcloud.hostxtra.co.uk';
    }
}

if($SSMAPIKEY -eq ""){

    if($DOCKEREXISTS -eq $True){
        write-host -ForegroundColor Cyan "Found Existing Docker with Name [$($AGENTNAME)]"
        $response = Read-Host -Prompt 'Do you want to use the existing containers api key? [Y/n]'

        if($response -eq "Y"){
            $SSMAPIKEYData=(docker inspect --format='{{range .Config.Env}}{{println .}}{{end}}' ${AGENTNAME}).split("=")

            $SSMAPIKEYData | %{
                if($_.startsWith("AGT-API")){
                    $SSMAPIKEY = $_;
                }
            }
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
}

write-host -ForegroundColor Cyan "Installation Summary:"
echo "Agent Name: $AGENTNAME"
echo "SF Port: $PORT"
echo "SSM Cloud URL: $SSMURL"
echo "SSM Cloud API Key: $SSMAPIKEY"
echo "Skip Docker Install: $NoDockerInstall"
echo ""

$response = Read-Host -Prompt 'Is the information correct? [y/N]'

if($response.ToLower() -ne "y"){
    Write-Warning "If you need to change any information you will need to re-run the installation script"
    exit 1;
}

if($NoDockerInstall){
    write-host -ForegroundColor Yellow "Docker install skipped"
}else{
    write-host "* Installing Docker"

    if($isWorkstation -eq $false){
        write-host -ForegroundColor Red "Windows Server is no longer supported!"
        write-host -ForegroundColor Red "Please use the standalone agent instead!"
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
            
            }
        }else{
            write-Error "Cant Install docker on this machine! Must be Windows 10 20H2 and Build Number 19041 or higher!"
            exit;
        }
    }
    sleep -m 3000

    write-host "* Docker Installed"

}



$DOCKER_IMG="mrhid6/ssmagent:latest"

docker pull -q $DOCKER_IMG

if($DOCKEREXISTS -eq $True){
    write-host "Removing existing docker container"
    docker rm -f $AGENTNAME
}

docker run -d `
-e SSM_NAME="$($AGENTNAME)" `
-e SSM_URL="$($SSMURL)" `
-e SSM_APIKEY="$($SSMAPIKEY)" `
-p "$($PORT):7777/udp" `
-p "$($PORT):7777/tcp" `
-v "C:\SSMAgent\$($AGENTNAME)\SSM:/home/ssm/SSM/Agents/$($AGENTNAME)" `
-v "C:\SSMAgent\$($AGENTNAME)\.config:/home/ssm/.config/Epic/FactoryGame" `
-v "C:\SSMAgent\$($AGENTNAME)\Data:/SSM/data" `
-m $MEMORY `
--name "$($AGENTNAME)" `
--restart always `
$DOCKER_IMG