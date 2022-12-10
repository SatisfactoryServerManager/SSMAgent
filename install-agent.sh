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

if [ ! "${PLATFORM}" == "Linux" ]; then
    echo -e "${RED}Error: Install Script Only Works On Linux Platforms!${NC}"
    exit 1
fi

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
SERVERQUERYPORT="15777"
BEACONPORT="15000"
PORT="7777"

while [[ $# -gt 0 ]]; do
    key="$1"

    case $key in
    --name)
        AGENTNAME="$2"
        shift # past value
        shift # past value
        ;;
    --serverqueryport)
        SERVERQUERYPORT="$2"
        shift # past value
        shift # past value
        ;;
    --serverqueryport)
        SERVERQUERYPORT="$2"
        shift # past value
        shift # past value
        ;;
    --beaconport)
        BEACONPORT="$2"
        shift # past value
        shift # past value
        ;;
    --port)
        PORT="$2"
        shift # past value
        shift # past value
        ;;
    esac
done

start_spinner "${YELLOW}Installing Docker${NC}"
wget -q https://get.docker.com/ -O - | sh >/dev/null 2>&1

stop_spinner $?

read -r -p "Enter SSM Cloud URL [https://ssmcloud.hostxtra.co.uk]: " SSMURL </dev/tty

if [ "${SSMURL}" == "" ]; then
    SSMURL = "https://ssmcloud.hostxtra.co.uk"
fi

read -r -p "Enter SSM Cloud API Key [AGT-API-XXXXXXX]: " SSMAPIKEY </dev/tty

if [ "${SSMAPIKEY}" == "" ]; then
    echo -e "${RED}You must enter your agent API key${NC}"
    exit 1
fi

docker run -d \
    -e SSM_URL="${SSMURL}" \
    -e SSM_APIKEY="${SSMAPIKEY}" \
    -p "${SERVERQUERYPORT}:15777" \
    -p "${BEACONPORT}:15000" \
    -p "${PORT}:7777" \
    --name "${AGENTNAME}" \
    mrhid6/ssmagent:latest
