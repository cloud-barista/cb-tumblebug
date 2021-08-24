#!/bin/bash

source ../common-functions.sh
readParametersByName "$@"
set -- "$CSP" "$REGION" "$POSTFIX" "$TestSetFile" "$OPTION01" "$OPTION02" "$OPTION03"

FILE=../credentials.conf
if [ ! -f "$FILE" ]; then
	echo "$FILE does not exist."
	exit
fi

if [ ! -f "$TestSetFile" ]; then
	echo "$TestSetFile does not exist."
	exit
fi
source $TestSetFile
source ../conf.env
source ../credentials.conf

getCloudIndex $CSP
MCISID=${MCISPREFIX}-${POSTFIX}

#install jq and puttygen
echo "[Check jq package (if not, install)]"
if ! dpkg-query -W -f='${Status}' jq | grep "ok installed"; then sudo apt install -y jq; fi
if ! dpkg-query -W -f='${Status}' putty-tools | grep "ok installed"; then sudo apt install -y putty-tools; fi
