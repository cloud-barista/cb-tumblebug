#!/bin/bash

echo ""
echo "Get IP of the current host and set SELF_ENDPOINT environment variable"
echo "(Default setting is in conf/setup.env)"
echo ""
echo "Setting SELF_ENDPOINT allows outside users access to CB-Tumblebug Swagger API Dashboard."
echo ""

if [[ "$(basename -- "$0")" == "setPublicIP.sh" ]]; then
    echo
    echo
    echo "- Executing [$0] will not work properly"
    echo "- Source the script with the following command [source $0]"
    echo
    exit 1
fi

# Get IP address which is accessable from outsite.
# `https://api.ipify.org` is one of IP lookup services. If it is not available we need to change.
str=$(curl https://api.ipify.org)

if [ -z "$str" ]
then
    echo "The result for IP lookup is empty."
    echo "Set SELF_ENDPOINT=localhost:1323"
    str=localhost
fi

export SELF_ENDPOINT=$str:1323

echo ""
echo "SELF_ENDPOINT (CB-TB Swagger API Dashboard): $SELF_ENDPOINT"
echo "[Note] To apply \$SELF_ENDPOINT, CB-TB restart is required."
echo ""