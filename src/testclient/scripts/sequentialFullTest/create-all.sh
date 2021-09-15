#!/bin/bash

SECONDS=0

source ../conf.env
source ../common-functions.sh

checkPrerequisite

./check-test-config.sh "$@"

while true; do
    read -p 'Confirm the above configuration. Do you want to proceed ? (y/n) : ' CHECKPROCEED
    case $CHECKPROCEED in
        [Yy]* ) 
            break
            ;;
        [Nn]* ) 
            echo "[Command: $0 $@] has been cancelled. See you soon. :)"
            exit 1
            ;;
        * ) 
            echo "Please answer yes or no.";;
    esac
done

./create-mcir-ns-cloud.sh "$@"

./create-mcis-only.sh "$@"

duration=$SECONDS


printElapsed $@
echo "" >>./executionStatus.history

