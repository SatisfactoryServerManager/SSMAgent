#!/bin/bash

EXE="/opt/SSMAgent/SSMAgent"
echo "Entry started"
#Define cleanup procedure
cleanup() {
    echo "Container stopped, performing cleanup..."
    pid=$(ps -ef | awk '{print $2" "$8}' | grep "$EXE" | awk '{print $1}')
    kill -INT $pid

    while true; do
        echo "Waiting for process to finish"
        pid=$(ps -ef | awk '{print $2" "$8}' | grep "$EXE" | awk '{print $1}')
        if [ "$pid" == "" ]; then
            break
        fi
        sleep 5
    done
    exit 0
}

#Trap SIGTERM
trap 'cleanup' SIGTERM

chown -R ssm:ssm /opt/SSMAgent
chown -R ssm:ssm /home/ssm
chown -R ssm:ssm /SSM/data

su ssm -c "${EXE} --name=$SSM_NAME --apikey=$SSM_APIKEY --url=$SSM_URL --grpcaddr=$SSM_GRPCADDR --grpcinsecure=${SSM_INSECURE:-false}" &

wait $!
sleep 40
