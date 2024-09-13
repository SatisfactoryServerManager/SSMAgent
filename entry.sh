#!/bin/bash

EXE="/opt/SSMAgent/SSMAgent"

#Define cleanup procedure
cleanup() {
    echo "Container stopped, performing cleanup..."
    pid=$(ps -ef | awk '$8=="${EXE}" {print $2}')
    kill -INT $pid

    while true; do
        echo "Waiting for process to finish"
        pid=$(ps -ef | awk '$8=="${EXE}" {print $2}')
        if [ "$pid" == "" ]; then
            break
        fi
        sleep 5
    done
    exit 0
}

#Trap SIGTERM
trap 'cleanup' SIGTERM

hostname

su ssm -c "${EXE} -name=$SSM_NAME -apikey=$SSM_APIKEY -url=$SSM_URL" &

wait $!
sleep 40
