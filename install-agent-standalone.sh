#!/bin/bash

export DEBIAN_FRONTEND=noninteractive

echo "#-----------------------------#"
echo "#      _____ _____ __  __     #"
echo "#     / ____/ ____|  \/  |    #"
echo "#    | (___| (___ | \  / |    #"
echo "#     \___ \\\\___ \| |\/| |    #"
echo "#     ____) |___) | |  | |    #"
echo "#    |_____/_____/|_|  |_|    #"
echo "#-----------------------------#"
echo "# Satisfactory Server Manager #"
echo "#-----------------------------#"

#Colors settings
BLUE='\033[0;34m'
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

function _spinner() {
    # $1 start/stop
    #
    # on start: $2 display message
    # on stop : $2 process exit status
    #           $3 spinner function pid (supplied from stop_spinner)

    local on_success="DONE"
    local on_fail="FAIL"
    local white="\e[1;37m"
    local green="\e[1;32m"
    local red="\e[1;31m"
    local nc="\e[0m"

    case $1 in
    start)
        # calculate the column where spinner and status msg will be displayed
        let column=$(tput cols)-${#2}-8
        # display message and position the cursor in $column column
        echo -ne ${2}
        printf "%${column}s"

        # start spinner
        i=1
        sp='\|/-'
        delay=${SPINNER_DELAY:-0.15}

        while :; do
            printf "\b${sp:i++%${#sp}:1}"
            sleep $delay
        done
        ;;
    stop)
        if [[ -z ${3} ]]; then
            echo "spinner is not running.."
            exit 1
        fi

        kill $3 >/dev/null 2>&1

        # inform the user uppon success or failure
        echo -en "\b["
        if [[ $2 -eq 0 ]]; then
            echo -en "${green}${on_success}${nc}"
        else
            echo -en "${red}${on_fail}${nc}"
        fi
        echo -e "]"
        ;;
    *)
        echo "invalid argument, try {start/stop}"
        exit 1
        ;;
    esac
}

function start_spinner {
    # $1 : msg to display
    _spinner "start" "${1}" &
    # set global spinner pid
    _sp_pid=$!
    disown
}

function stop_spinner {
    # $1 : command exit status
    _spinner "stop" $1 $_sp_pid
    unset _sp_pid
}

AGENTNAME=""
PORTOFFSET="0";
SSMURL=""
SSMAPIKEY=""

PLATFORM="$(uname -s)"

if [ ! "${PLATFORM}" == "Linux" ]; then
    echo "Error: Install Script Only Works On Linux Platforms!"
    exit 1
fi

OVERRIDE_INSTALL_DIR=""
OVERRIDE_DATA_DIR=""

while [[ $# -gt 0 ]]; do
    key="$1"

    case $key in
    --name)
        AGENTNAME="$2"
        shift # past value
        shift # past value
        ;;
    --portoffset)
        PORTOFFSET="$2"
        shift # past value
        shift # past value
        ;;
    --url)
        SSMURL="$2"
        shift # past value
        shift # past value
        ;;
    --apikey)
        SSMAPIKEY="$2"
        shift # past value
        shift # past value
        ;;
    --installdir)
        OVERRIDE_INSTALL_DIR="$2"
        shift # past value
        shift # past value
        ;;
    --datadir)
        OVERRIDE_DATA_DIR="$2"
        shift # past value
        shift # past value
        ;;
    esac
done

if [ "$AGENTNAME" == "" ];then
    echo -e "${RED}Error: Agent Name is null!${NC}"
    exit 1
fi


SERVERQUERYPORT=$((15777 + $PORTOFFSET))
BEACONPORT=$((15000 + $PORTOFFSET))
PORT=$((7777 + $PORTOFFSET))
INSTALL_DIR="/opt/SSMAgent/${AGENTNAME}"
DATA_DIR="/SSM/data/${AGENTNAME}"
SSM_SERVICENAME="SSMAgent@${AGENTNAME}.service"
SSM_SERVICEFILE="/etc/systemd/system/SSMAgent@${AGENTNAME}.service"

if [ -n "$OVERRIDE_INSTALL_DIR" ]; then
    INSTALL_DIR=$OVERRIDE_INSTALL_DIR
fi

if [ -n "$OVERRIDE_DATA_DIR" ]; then
    DATA_DIR=$OVERRIDE_DATA_DIR
fi


if [ "$SERVERQUERYPORT" -lt "15777" ]; then
    echo -e "${RED}Error: Port Offset cannot be < 0!${NC}"
    exit 1;
fi


if [ "${SSMURL}" == "" ]; then
    read -r -p "Enter SSM Cloud URL [https://api-ssmcloud.hostxtra.co.uk]: " SSMURL </dev/tty

    if [ "${SSMURL}" == "" ]; then
        SSMURL="https://api-ssmcloud.hostxtra.co.uk"
    fi
fi
if [ "${SSMAPIKEY}" == "" ]; then
    read -r -p "Enter SSM Cloud API Key [AGT-API-XXXXXXX]: " SSMAPIKEY </dev/tty

    if [ "${SSMAPIKEY}" == "" ]; then
        echo -e "${RED}You must enter your agent API key${NC}"
        exit 1
    fi
fi

start_spinner "${YELLOW}Gathering Version Number${NC}"
curl --silent "https://api.github.com/repos/satisfactoryservermanager/ssmagent/releases/latest" >${TEMP_DIR}/SSM_releases.json
SSM_VER=$(cat ${TEMP_DIR}/SSM_releases.json | jq -r ".tag_name")
SSM_URL=$(cat ${TEMP_DIR}/SSM_releases.json | jq -r ".assets[].browser_download_url" | grep -i "Linux" | sort | head -1)
stop_spinner $?

echo -e "${BLUE}Installation Summary: ${NC}"
echo "Agent Name: ${AGENTNAME}"
echo "Installation Directory: ${INSTALL_DIR}"
echo "Data Directory: ${DATA_DIR}"
echo "SF Server Query Port: ${SERVERQUERYPORT}"
echo "SF Beacon Port: ${BEACONPORT}"
echo "SF Port: ${PORT}"
echo "SSM Cloud URL: ${SSMURL}"
echo "SSM Cloud API Key: ${SSMAPIKEY}"
echo ""

read -r -p "Is the information correct? [y/N]: " response </dev/tty
case $response in
[yY]*)
;;
*)
    echo -e "${RED}Installation was incorrect re-run install script${NC}"
    exit 1
;;
esac

UPDATE_SSM=0

if [ -f "${INSTALL_DIR}/version.txt" ]; then
    EXISTING_VER=$(cat "${INSTALL_DIR}/version.txt")
    echo -e "${YELLOW}Found Existing Installation ${EXISTING_VER}${NC}"
    UPDATE_SSM=1

    if [ "$EXISTING_VER" == "$SSM_VER" ]; then
        echo -e "${YELLOW}Installed Version ${EXISTING_VER} Is The Latest Version${NC}"
        read -r -p "Would you like to force install ${SSM_VER}? [y/N]: " response </dev/tty
        case $response in
        [yY]*)
        ;;
        *)
            echo -e "${RED}Canceled Installation${NC}"
            exit 1
        ;;
        esac
    else
        read -r -p "Would you like to update to ${SSM_VER}? [Y/n]: " response </dev/tty
        case $response in
        [nN]*)
            echo -e "${RED}Canceled Installation${NC}"
            exit 1
            ;;
        *)
            ;;
        esac
    fi
fi


SSM_SERVICE=$(
        systemctl list-units --full -all | grep -Fq "${SSM_SERVICENAME}"
        echo $?
    )

if [ -f "${SSM_SERVICEFILE}" ]; then
    if [ ${SSM_SERVICE} -eq 0 ]; then
        start_spinner "${YELLOW}Stopping SSM Service${NC}"
        systemctl stop ${SSM_SERVICENAME}
        stop_spinner $?
    fi
fi



start_spinner "${YELLOW}Updating System${NC}"
apt-get -qq update -y >/dev/null 2>&1
apt-get -qq upgrade -y >/dev/null 2>&1
stop_spinner $?

start_spinner "${YELLOW}Updating Timezone${NC}"
ln -fs /usr/share/zoneinfo/Europe/London /etc/localtime
apt-get -qq install -y tzdata >/dev/null 2>&1
dpkg-reconfigure --frontend noninteractive tzdata >/dev/null 2>&1
stop_spinner $?

start_spinner "${YELLOW}Installing Prereqs${NC}"
apt-get -qq install apt-utils curl wget jq binutils software-properties-common libcap2-bin unzip zip htop dnsmasq -y >/dev/null 2>&1


if [ $UPDATE_SSM -eq 0 ]; then
    apt-get -qq update -y >/dev/null 2>&1
    add-apt-repository multiverse -y >/dev/null 2>&1
    dpkg --add-architecture i386 >/dev/null 2>&1
    apt-get -qq install lib32gcc-s1 -y 
    apt-get -qq update -y
fi
stop_spinner $?

start_spinner "${YELLOW}Creating Directories${NC}"
mkdir -p ${INSTALL_DIR} >/dev/null 2>&1
mkdir -p ${DATA_DIR} >/dev/null 2>&1

rm -r ${INSTALL_DIR}/* >/dev/null 2>&1
stop_spinner 0


start_spinner "${YELLOW}Downloading SSM Agent${NC}"
wget -q "${SSM_URL}" -O "${INSTALL_DIR}/SSMAgent.zip"
stop_spinner $?

start_spinner "${YELLOW}Installing SSM Agent${NC}"
unzip -q "${INSTALL_DIR}/SSMAgent.zip" -d "${INSTALL_DIR}"

mv "${INSTALL_DIR}/release/linux/SSMAgent" "${INSTALL_DIR}" >/dev/null 2>&1
rm -r "${INSTALL_DIR}/release/linux" >/dev/null 2>&1

rm "${INSTALL_DIR}/SSMAgent.zip" >/dev/null 2>&1
rm "${INSTALL_DIR}/build.log" >/dev/null 2>&1
echo ${SSM_VER} >"${INSTALL_DIR}/version.txt"

stop_spinner $?


setcap cap_net_bind_service=+ep $(readlink -f ${INSTALL_DIR}/SSMAgent)

start_spinner "${YELLOW}Creating SSM User Account${NC}"
if id "ssm" &>/dev/null; then
    usermod -u 9999 ssm >/dev/null 2>&1
    groupmod -g 9999 ssm >/dev/null 2>&1
else
    useradd -m ssm -u 9999 -s /bin/bash >/dev/null 2>&1
fi
sleep 1
stop_spinner $?

start_spinner "${YELLOW}Updating Folder Permissions${NC}"
chown -R ssm:ssm /home/ssm >/dev/null 2>&1
chown -R ssm:ssm /opt/SSMAgent >/dev/null 2>&1
chown -R ssm:ssm /SSM >/dev/null 2>&1
chmod -R 755 /opt/SSMAgent >/dev/null 2>&1
chmod -R 755 /SSM >/dev/null 2>&1
stop_spinner 0



ENV_SYSTEMD=$(pidof systemd | wc -l)
ENV_SYSTEMCTL=$(which systemctl | wc -l)

if [[ ${ENV_SYSTEMD} -eq 0 ]] && [[ ${ENV_SYSTEMCTL} -eq 0 ]]; then
    echo -e "${RED}Error: Cant install service on this system!${NC}"
    exit 3
fi

if [ ${SSM_SERVICE} -eq 0 ]; then
    start_spinner "${YELLOW}Removing Old SSM Agent Service${NC}"
    systemctl disable ${SSM_SERVICENAME} >/dev/null 2>&1
    rm -r "${SSM_SERVICEFILE}" >/dev/null 2>&1
    systemctl daemon-reload >/dev/null 2>&1
    stop_spinner $?
fi

start_spinner "${YELLOW}Creating SSM Agent Service${NC}"

cat >>${SSM_SERVICEFILE} <<EOL
[Unit]
Description=SSM Agent Daemon - ${AGENTNAME}
After=network.target

[Service]
User=ssm
Group=ssm

Type=simple
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/SSMAgent -name=${AGENTNAME} -p=${PORTOFFSET} -url=${SSMURL} -apikey=${SSMAPIKEY} -datadir="${DATA_DIR}"
TimeoutStopSec=20
KillMode=process
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOL

sleep 1
stop_spinner $?

start_spinner "${YELLOW}Starting SSM Agent Service${NC}"
systemctl daemon-reload >/dev/null 2>&1
systemctl enable ${SSM_SERVICENAME} >/dev/null 2>&1
systemctl start ${SSM_SERVICENAME} >/dev/null 2>&1
stop_spinner $?