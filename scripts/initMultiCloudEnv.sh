#!/bin/bash

if [ -z "$CBTUMBLEBUG_ROOT" ]; then
    SCRIPT_DIR=`dirname ${BASH_SOURCE[0]-$0}`
    export CBTUMBLEBUG_ROOT=`cd $SCRIPT_DIR && cd .. && pwd`
fi

$CBTUMBLEBUG_ROOT/src/testclient/scripts/1.configureSpider/register-cloud-interactive.sh -n tb

echo -e "${BOLD}"
while true; do
    read -p 'Load common Specs and Images. Do you want to proceed ? (y/n) : ' CHECKPROCEED
    echo -e "${NC}"
    case $CHECKPROCEED in
    [Yy]*)
        break
        ;;
    [Nn]*)
        echo
        echo "Cancel [$0 $@]"
        echo "See you soon. :)"
        echo
        exit 1
        ;;
    *)
        echo "Please answer yes or no."
        ;;
    esac
done

$CBTUMBLEBUG_ROOT/src/testclient/scripts/2.configureTumblebug/load-common-resource.sh -n tb

echo -e "${BOLD}"
while true; do
    read -p 'Create default namespace (ns01). Do you want to proceed ? (y/n) : ' CHECKPROCEED
    echo -e "${NC}"
    case $CHECKPROCEED in
    [Yy]*)
        break
        ;;
    [Nn]*)
        echo
        echo "Cancel [$0 $@]"
        echo "See you soon. :)"
        echo
        exit 1
        ;;
    *)
        echo "Please answer yes or no."
        ;;
    esac
done

$CBTUMBLEBUG_ROOT/src/testclient/scripts/2.configureTumblebug/create-ns.sh -n tb
