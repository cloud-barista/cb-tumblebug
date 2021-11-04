#!/bin/bash

RED='\033[0;31m'
LGREEN='\033[1;32m'
PURPLE='\033[0;35m'
NC='\033[0m' # No Color

if [ -z "$CBTUMBLEBUG_ROOT" ]; then
    SCRIPT_DIR=`dirname ${BASH_SOURCE[0]-$0}`
    export CBTUMBLEBUG_ROOT=`cd $SCRIPT_DIR && cd .. && pwd`
fi

CONF_PATH="$CBTUMBLEBUG_ROOT/conf"
TEMPLATE_FILE_NAME="template.credentials.conf"
CRED_FILE_NAME="credentials.conf"

echo
echo ==========================================================
echo "[Info]"
echo ==========================================================
echo -e "This script genrete ${RED}${CRED_FILE_NAME}${NC} file for Cloud credentials"
echo
echo -e "- Copy ${CONF_PATH}/${LGREEN}${TEMPLATE_FILE_NAME} ${NC} file"
echo -e "-   to ${CONF_PATH}/${RED}${CRED_FILE_NAME} ${NC}"
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
echo -e "Copying.. (if you don't want overwrite, type ${RED}[n] or [no]${NC}) ${RED}"
cp -i ${CONF_PATH}/${TEMPLATE_FILE_NAME} ${CONF_PATH}/${CRED_FILE_NAME}
echo -e "${NC}"
echo -e "Current contents of ${CONF_PATH}/${RED}${CRED_FILE_NAME} ${PURPLE}"
cat ${CONF_PATH}/${CRED_FILE_NAME}
echo -e "${NC}"

echo
echo ==========================================================
echo "[What's next]"
echo ==========================================================
echo -e "Edit ${RED}${CRED_FILE_NAME} file to add your Cloud credentials"
echo
echo -e "Ex) vi ${CONF_PATH}/${RED}${CRED_FILE_NAME} ${NC}"
echo