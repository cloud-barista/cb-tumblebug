#!/bin/bash

SECONDS=0

./check-test-config.sh "$@"

read -p 'Confirm the above configuration. Do you want to proceed ? (y/n) : ' CHECKPROCEED
if [ "${CHECKPROCEED}" != "y" ]; then
	echo "[Command: $0 $@] has been cancelled. See you soon. :)"
    exit 0
fi

./create-mcir-ns-cloud.sh "$@"

./create-mcis-only.sh "$@"

duration=$SECONDS

source ../common-functions.sh
printElapsed $@
echo "" >>./executionStatus.history

