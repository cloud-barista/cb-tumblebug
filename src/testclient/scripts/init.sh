#!/bin/bash

source $CBTUMBLEBUG_ROOT/src/testclient/scripts/common-functions.sh
readParametersByName "$@"
set -- "$CSP" "$REGION" "$POSTFIX" "$TestSetFile" "$OPTION01" "$OPTION02" "$OPTION03"

if [ ! -f "$TestSetFile" ]; then
	echo "$TestSetFile does not exist."
	exit
fi
source $TestSetFile
source $CBTUMBLEBUG_ROOT/src/testclient/scripts/conf.env

getCloudIndex $CSP
MCISID=${POSTFIX}
K8SCLUSTERID_PREFIX=${POSTFIX}

#install jq and puttygen
if ! dpkg-query -W -f='${Status}' jq | grep "ok installed" > /dev/null; then echo "install jq package"; sudo apt install -y jq; fi
if ! dpkg-query -W -f='${Status}' putty-tools | grep "ok installed" > /dev/null; then echo "install putty-tools package"; sudo apt install -y putty-tools; fi
