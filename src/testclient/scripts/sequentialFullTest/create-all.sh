#!/bin/bash

SECONDS=0

source ../conf.env
source ../common-functions.sh

checkPrerequisite

./check-test-config.sh "$@"
echo -e "${BOLD}"
while true; do
    read -p 'Confirm the above configuration. Do you want to proceed ? (y/n) : ' CHECKPROCEED
    echo -e "${NC}"
    case $CHECKPROCEED in
        [Yy]* ) 
            break
            ;;
        [Nn]* ) 
            echo
            echo "Cancel [$0 $@]"
            echo "See you soon. :)"
            echo
            exit 1
            ;;
        * ) 
            echo "Please answer yes or no.";;
    esac
done

./create-resource-ns-cloud.sh "$@"

./create-mci-only.sh "$@"

duration=$SECONDS


printElapsed $@
echo "" >>./executionStatus.history

