#!/bin/bash

if [ -z "$CBTUMBLEBUG_ROOT" ]; then
    SCRIPT_DIR=`dirname ${BASH_SOURCE[0]-$0}`
    export CBTUMBLEBUG_ROOT=`cd $SCRIPT_DIR && cd .. && pwd`
fi

$CBTUMBLEBUG_ROOT/src/testclient/scripts/1.configureSpider/register-cloud-interactive.sh -n tb
