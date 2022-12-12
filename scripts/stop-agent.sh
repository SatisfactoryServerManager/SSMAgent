#!/bin/bash

PID=$(ps -ef | grep "/opt/SSMAgent/SSMAgent" | grep -v "su ssm" | grep -v "grep" | awk '{print $2}')

kill -15 $PID;