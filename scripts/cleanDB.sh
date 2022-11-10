#!/bin/bash

RED='\033[0;31m'
LGREEN='\033[1;32m'
NC='\033[0m' # No Color

if [ -z "$CBTUMBLEBUG_ROOT" ]; then
    SCRIPT_DIR=`dirname ${BASH_SOURCE[0]-$0}`
    export CBTUMBLEBUG_ROOT=`cd $SCRIPT_DIR && cd .. && pwd`
fi


TBMETAPATH="$CBTUMBLEBUG_ROOT/meta_db/dat"
SPMETAPATH="$CBTUMBLEBUG_ROOT/container-volume/cb-spider-container"

echo
echo ==========================================================
echo "[Info]"
echo ==========================================================
echo -e "This script will ${LGREEN}remove all MetaDate in Local DataBases.${NC}"
echo "(MetaData from both CB-Tumblebug and CB-Spider container)"
echo
echo -e "Will remove following directories and files"
echo
echo -e "Path:${RED}${TBMETAPATH} ${NC}"
ls $TBMETAPATH
echo 
echo -e "Path:${RED}${SPMETAPATH} ${NC}"
ls $SPMETAPATH
echo

while true; do
    read -p 'Do you want to proceed ? (y/n) : ' CHECKPROCEED
    case $CHECKPROCEED in
    [Yy]*)
        break
        ;;
    [Nn]*)
        echo
        echo "Stop $0 See you soon :)"
        exit 1
        ;;
    *)
        echo "Please answer yes or no."
        ;;
    esac
done

echo

ls $TBMETAPATH
ls $SPMETAPATH

echo

sudo rm -rf $TBMETAPATH
sudo rm -rf $SPMETAPATH

echo -e "${LGREEN} Done! ${NC}"
