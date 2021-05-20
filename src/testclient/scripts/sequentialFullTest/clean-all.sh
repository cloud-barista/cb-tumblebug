#!/bin/bash

SECONDS=0

./clean-mcis-only.sh "$@"

./clean-mcir-ns-cloud.sh "$@"

duration=$SECONDS

source ../common-functions.sh
printElapsed $@
echo "" >>./executionStatus.history
