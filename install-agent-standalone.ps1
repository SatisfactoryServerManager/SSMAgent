param(
    [String]$AGENTNAME="",
    [Int32]$PORTOFFSET=0,
    [String]$SSMURL="",
    [String]$SSMAPIKEY="",
    [String]$ServiceUser="",
    [String]$ServicePassword=""
)

echo "#-----------------------------#"
echo "#      _____ _____ __  __     #"
echo "#     / ____/ ____|  \/  |    #"
echo "#    | (___| (___ | \  / |    #"
echo "#     \___ \\___ \| |\/| |    #"
echo "#     ____) |___) | |  | |    #"
echo "#    |_____/_____/|_|  |_|    #"
echo "#-----------------------------#"
echo "# Satisfactory Server Manager #"
echo "#-----------------------------#"

$SERVERQUERYPORT=15777 + $PORTOFFSET;
$BEACONPORT = 15000 + $PORTOFFSET;
$PORT = 7777 + $PORTOFFSET;

$currentPrincipal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
$isAdmin = $currentPrincipal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)

if($isAdmin -eq $false){
    write-error "You need to run this script as Administrator";
    exit 1;
}


if([string]::IsNullOrWhiteSpace($AGENTNAME)){
    Write-Error "Cant continue installation with empty Agent Name";
    exit 2;
}

if($SSMURL -eq ""){
    $SSMURL = Read-Host -Prompt 'Enter SSM Cloud URL [https://api-ssmcloud-dev.hostxtra.co.uk]'

    if ([string]::IsNullOrWhiteSpace($SSMURL))
    {
        $SSMURL = 'https://api-ssmcloud-dev.hostxtra.co.uk';
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


$INSTALL_DIR="C:\Program Files\SSMAgent\$AGENTNAME"

write-host -ForegroundColor Cyan "Installation Summary:"
echo "Agent Name: $AGENTNAME"
echo "Installation Directory: $INSTALL_DIR"
echo "SF Server Query Port: $SERVERQUERYPORT"
echo "SF Beacon Port: $BEACONPORT"
echo "SF Port: $PORT"
echo "SSM Cloud URL: $SSMURL"
echo "SSM Cloud API Key: $SSMAPIKEY"
echo ""

$response = Read-Host -Prompt 'Is the information correct? [y/N]'

if($response.ToLower() -ne "y"){
    Write-Warning "If you need to change any information you will need to re-run the installation script"
    exit 1;
}


$SSM_releases = "https://api.github.com/repos/satisfactoryservermanager/ssmagent/releases/latest"

$SSM_release = (Invoke-WebRequest $SSM_releases -UseBasicParsing| ConvertFrom-Json)[0]
$SSM_VER = $SSM_release[0].tag_name
$SSM_URL = $SSM_release.assets[1].browser_download_url

if((Test-Path $INSTALL_DIR) -eq $true){

    $response = Read-Host -Prompt 'Do you want to update SSM? [y/N]'
    if($response.ToLower() -ne "y"){
        Write-Warning "You have requested the Update be skipped"
        exit 1;
    }

    $SSM_CUR=Get-Content -path "$($INSTALL_DIR)\version.txt" -ErrorAction SilentlyContinue
    write-host "Updating SSM $($SSM_CUR) to $($SSM_VER)"
    
}else{
    write-host "Installing SSM $($SSM_VER)"
    New-Item -ItemType Directory -Path "$($INSTALL_DIR)" -Force | out-null
}


$SSM_SeriveName="SSMAgent@$($AGENTNAME)"
$SSM_Service = Get-Service -Name "$($SSM_SeriveName)" -ErrorAction SilentlyContinue
sleep -m 1000

if($SSM_Service -ne $null -and $isAdmin -eq $true){
	write-host "Stopping SSM Service"
    & "$($INSTALL_DIR)\nssm.exe" "stop" "$($SSM_SeriveName)" "confirm"

    sleep -m 2000
}


write-host "* Downloading SSM"
Remove-Item -Path "$($INSTALL_DIR)\*" -Recurse | out-null

Invoke-WebRequest $SSM_URL -Out "$($INSTALL_DIR)\SSMAgent.zip" -UseBasicParsing
Expand-Archive "$($INSTALL_DIR)\SSMAgent.zip" -DestinationPath "$($INSTALL_DIR)" -Force
Move-Item -Path "$($INSTALL_DIR)\release\windows\*" -Destination "$($INSTALL_DIR)"

write-host "* Cleanup"
Remove-Item -Recurse -Path "$($INSTALL_DIR)\release"
Remove-Item -Path "$($INSTALL_DIR)\SSMAgent.zip"
Set-Content -Path "$($INSTALL_DIR)\version.txt" -Value "$($SSM_VER)"


write-host "Creating SSM Service"
write-host "* Downloading NSSM"
	
Invoke-WebRequest "https://nssm.cc/ci/nssm-2.24-101-g897c7ad.zip" -Out "$($INSTALL_DIR)\nssm.zip" -UseBasicParsing
Expand-Archive "$($INSTALL_DIR)\nssm.zip" -DestinationPath "$($INSTALL_DIR)" -Force
	
Move-item -Path "$($INSTALL_DIR)\nssm-2.24-101-g897c7ad\win64\nssm.exe" -Destination "$($INSTALL_DIR)\nssm.exe" -force
Remove-Item -Path "$($INSTALL_DIR)\nssm-2.24-101-g897c7ad" -Recurse
Remove-Item -Path "$($INSTALL_DIR)\nssm.zip"


if($SSM_Service -ne $null){
	write-host "* Removing Old SSM Service"
	& "$($INSTALL_DIR)\nssm.exe" "remove" "$($SSM_SeriveName)" "confirm" | out-null
    sleep -m 2000
}


if($ServiceUser -eq "" -or $ServicePassword -eq ""){
    write-host "Please provide a Service User Account to run the SSM Agent"
    $CurrentUser = [System.Security.Principal.WindowsIdentity]::GetCurrent().Name
    $ServiceUser = Read-Host -Prompt "Service Username [$($CurrentUser)]"

    if($ServiceUser -eq ""){
        $ServiceUser = $($CurrentUser)
    }

    $ServicePasswordRes = Read-Host -AsSecureString -Prompt "Service Password"
    $ServicePassword = [Runtime.InteropServices.Marshal]::PtrToStringAuto([Runtime.InteropServices.Marshal]::SecureStringToBSTR($ServicePasswordRes))
    if($ServicePassword -eq ""){
        Write-host -ForegroundColor RED "Error please provide a service account password!"
        exit 1;
    }
}

write-host "* Create SSM Service"
& "$($INSTALL_DIR)\nssm.exe" "install" "$($SSM_SeriveName)" "$($INSTALL_DIR)\SSMAgent.exe" "-name=$($AGENTNAME) -p=$($PORTOFFSET) -url=$($SSMURL) -apikey=$($SSMAPIKEY) -datadir=C:\SSM\data\$($AGENTNAME)" | out-null

if($LASTEXITCODE -ne 0){
    Write-host -ForegroundColor RED "Error setting up SSM Agent Service!"
    exit $LASTEXITCODE
}

& "$($INSTALL_DIR)\nssm.exe" "set" "$($SSM_SeriveName)" "AppDirectory" "$($INSTALL_DIR)" | out-null
& "$($INSTALL_DIR)\nssm.exe" "set" "$($SSM_SeriveName)" "DisplayName" "SSMAgent_$($AGENTNAME)" | out-null
& "$($INSTALL_DIR)\nssm.exe" "set" "$($SSM_SeriveName)" "Description" "Service for SSM Agent" | out-null
& "$($INSTALL_DIR)\nssm.exe" "set" "$($SSM_SeriveName)" "ObjectName" "$ServiceUser" "$ServicePassword" | out-null

if($LASTEXITCODE -ne 0){
    Write-host -ForegroundColor RED "Error setting up SSM Agent Service!"
    exit $LASTEXITCODE
}



$SSM_Service = Get-Service -Name "$($SSM_SeriveName)" -ErrorAction SilentlyContinue

sleep -m 1500
write-host "* Start SSM Service"
& "$($INSTALL_DIR)\nssm.exe" "start" "$($SSM_SeriveName)" | out-null