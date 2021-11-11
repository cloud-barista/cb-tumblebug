#!/bin/bash

SECONDS=0

echo "[Check jq package (if not, install)]"
if ! dpkg-query -W -f='${Status}' jq | grep "ok installed"; then sudo apt install -y jq; fi

source ../conf.env

echo "####################################################################"
echo "## Command (SSH) to MCIS "
echo "####################################################################"

source ../common-functions.sh

# CSP=${1}
# REGION=${2:-1}
# POSTFIX=${3:-developer}

# NUM_MCIS=${1}

# if [ "${NUM_MCIS}" == "" ]; then
#     echo "Usage: ./script.sh <NUM_MCIS> <MCIS_1> <MCIS_2> <MCIS_3> ..."
#     exit 1
# fi

# for (( i=0; i<${NUM_MCIS}; i++ ));
# do
#     j=$((i+2))
#     echo ${$j};
# done

NUM_MCIS=$#

WHOLE_IPLIST=""
WHOLE_PRIVIPLIST=""
LORDIP=""
LORDVM=""
LORDMCIS=""

for MCISID in "$@"; do
    MCISINFO=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/mcis/${MCISID}?option=status)
    VMARRAY=$(jq -r '.status.vm' <<<"$MCISINFO")
    MASTERIP=$(jq -r '.status.masterIp' <<<"$MCISINFO")
    MASTERVM=$(jq -r '.status.masterVmId' <<<"$MCISINFO")

    echo "MASTERIP: $MASTERIP"
    echo "MASTERVM: $MASTERVM"
    echo "VMARRAY: $VMARRAY"

    IPLIST=""

    for row in $(echo "${VMARRAY}" | jq -r '.[] | @base64'); do
        _jq() {
            echo ${row} | base64 --decode | jq -r ${1}
        }

        IPLIST+=$(_jq '.publicIp')
        IPLIST+=" "
    done

    IPLIST=$(echo ${IPLIST})
    echo "PublicIPList: $IPLIST"

    PRIVIPLIST=""

    for row in $(echo "${VMARRAY}" | jq -r '.[] | @base64'); do
        _jq() {
            echo ${row} | base64 --decode | jq -r ${1}
        }

        PRIVIPLIST+=$(_jq '.privateIp')
        PRIVIPLIST+=" "
    done

    PRIVIPLIST=$(echo ${PRIVIPLIST})
    echo "PrivateIPList: $PRIVIPLIST"

    WHOLE_IPLIST+=$IPLIST
    WHOLE_IPLIST+=" "
    WHOLE_PRIVIPLIST+=$PRIVIPLIST
    WHOLE_PRIVIPLIST+=" "
    # WHOLE_IPLIST=(${WHOLE_IPLIST[@]} ${IPLIST[@]})
    # WHOLE_PRIVIPLIST=(${WHOLE_PRIVIPLIST[@]} ${PRIVIPLIST[@]})
    LORDIP=$MASTERIP
    LORDVM=$MASTERVM
    LORDMCIS=$MCISID
done

echo ""
echo "WHOLE_PublicIPList: $WHOLE_IPLIST"
echo "WHOLE_PrivateIPList: $WHOLE_PRIVIPLIST"

# for MCISID in "$@" ; do
#     echo ""
#     echo "Installing Weavescope to MCIS..."
#     echo ""
#     curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mcis/$MCISID -H 'Content-Type: application/json' -d \
#     '{
#     "command": "sudo apt-get update > /dev/null;  sudo apt install docker.io -y; sudo curl -L git.io/scope -o /usr/local/bin/scope; sudo chmod a+x /usr/local/bin/scope"
#     }' | jq ''
#     echo ""
# done

LAUNCHCMD="sudo scope stop"
echo "Stopping Weavescope for master node if exist..."
curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mcis/$LORDMCIS/vm/$LORDVM -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "${LAUNCHCMD}"
	}
EOF

#LAUNCHCMD="sudo scope launch $WHOLE_IPLIST $WHOLE_PRIVIPLIST"
LAUNCHCMD="sudo scope launch $WHOLE_IPLIST"
#echo $LAUNCHCMD

echo "Launching Weavescope for master node..."
curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mcis/$LORDMCIS/vm/$LORDVM -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "${LAUNCHCMD}"
	}
EOF

# 	echo ""
# 	echo "[MCIS Weavescope: master node only] Access to"
#     echo " $LORDIP:4040/#!/state/{\"contrastMode\":true,\"topologyId\":\"containers-by-hostname\"}"
# 	echo ""

#     LAUNCHCMD="sudo scope launch $LORDIP $WHOLE_PRIVIPLIST"
# 	echo "Working on clustring..."

#     for MCISID in "$@" ; do

#         echo "Launching Weavescope for the other nodes..."
#         curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mcis/$MCISID -H 'Content-Type: application/json' -d @- <<EOF
#         {
#         "command"        : "${LAUNCHCMD}"
#         }
# EOF

#     done

echo "Done!"
duration=$SECONDS

printElapsed $@
echo ""

echo "[MCIS Weavescope: complete cluster] Access to"
echo " $LORDIP:4040/#!/state/{\"contrastMode\":true,\"topologyId\":\"containers-by-hostname\"}"
echo ""
