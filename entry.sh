#!/bin/bash

su ssm -c "/opt/SSMAgent/SSMAgent -name=$SSM_NAME -apikey=$SSM_APIKEY -url=$SSM_URL"
