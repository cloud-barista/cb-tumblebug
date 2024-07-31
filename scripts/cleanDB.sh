#!/bin/bash

RED='\033[0;31m'
LGREEN='\033[1;32m'
NC='\033[0m' # No Color

if [ -z "$TB_ROOT_PATH" ]; then
    SCRIPT_DIR=`dirname ${BASH_SOURCE[0]-$0}`
    export TB_ROOT_PATH=`cd $SCRIPT_DIR && cd .. && pwd`
fi


TBMETAPATH="$TB_ROOT_PATH/meta_db/dat"
VOL_TB_META_PATH="$TB_ROOT_PATH/container-volume/cb-tumblebug-container"
VOL_SP_META_PATH="$TB_ROOT_PATH/container-volume/cb-spider-container"
VOL_ETC_DATA_PATH="$TB_ROOT_PATH/container-volume/etcd"

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
echo -e "Path:${RED}${VOL_TB_META_PATH} ${NC}"
ls $VOL_TB_META_PATH
echo 
echo -e "Path:${RED}${VOL_SP_META_PATH} ${NC}"
ls $VOL_SP_META_PATH
echo
echo -e "Path:${RED}${VOL_ETC_DATA_PATH} ${NC}"
ls $VOL_ETC_DATA_PATH
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
ls $VOL_TB_META_PATH
ls $VOL_SP_META_PATH
ls $VOL_ETC_DATA_PATH

echo

sudo rm -rf $TBMETAPATH
sudo rm -rf $VOL_TB_META_PATH
sudo rm -rf $VOL_SP_META_PATH
sudo rm -rf $VOL_ETC_DATA_PATH

echo -e "${LGREEN} Done! ${NC}"
