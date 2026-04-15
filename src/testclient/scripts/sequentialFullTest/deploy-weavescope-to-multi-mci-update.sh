#!/bin/bash

SECONDS=0

echo "[Check jq package (if not, install)]"
if ! dpkg-query -W -f='${Status}' jq | grep "ok installed"; then sudo apt install -y jq; fi

source ../conf.env

echo "####################################################################"
echo "## Command (SSH) to Infra "
echo "####################################################################"

source ../common-functions.sh

# CSP=${1}
# REGION=${2:-1}
# POSTFIX=${3:-developer}

# NUM_Infra=${1}

# if [ "${NUM_Infra}" == "" ]; then
#     echo "Usage: ./script.sh <NUM_Infra> <Infra_1> <Infra_2> <Infra_3> ..."
#     exit 1
# fi

# for (( i=0; i<${NUM_Infra}; i++ ));
# do
#     j=$((i+2))
#     echo ${$j};
# done

NUM_Infra=$#

WHOLE_IPLIST=""
WHOLE_PRIVIPLIST=""
LORDIP=""
LORDVM=""
LORDInfra=""

for InfraID in "$@"; do
    InfraINFO=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/infra/${InfraID}?option=status)
    VMARRAY=$(jq -r '.status.vm' <<<"$InfraINFO")
    MASTERIP=$(jq -r '.status.masterIp' <<<"$InfraINFO")
    MASTERVM=$(jq -r '.status.masterVmId' <<<"$InfraINFO")

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
    LORDInfra=$InfraID
done

echo ""
echo "WHOLE_PublicIPList: $WHOLE_IPLIST"
echo "WHOLE_PrivateIPList: $WHOLE_PRIVIPLIST"

for InfraID in "$@"; do
    echo ""
    echo "Installing Weavescope to Infra..."
    ScopeInstallFile="git.io/scope"
    ScopeInstallFile="https://gist.githubusercontent.com/seokho-son/bb2703ca49555f9afe0d0097894c74fa/raw/9eb65b296b85bc53f53af3e8733603d807fb9287/scope"
    INSTALLCMD="sudo apt-get update > /dev/null;  sudo apt install docker.io -y; sudo curl -L $ScopeInstallFile -o /usr/local/bin/scope; sudo chmod a+x /usr/local/bin/scope"
    echo ""

    VAR1=$(
        curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/infra/$InfraID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "[${INSTALLCMD}]"
	}
EOF
    )
    echo "${VAR1}" | jq '.'
    echo ""
done

LAUNCHCMD="sudo scope stop"
echo "Stopping Weavescope for master node if exist..."
curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/infra/$LORDInfra/vm/$LORDVM -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "[${LAUNCHCMD}]"
	}
EOF

#LAUNCHCMD="sudo scope launch $WHOLE_IPLIST $WHOLE_PRIVIPLIST"
LAUNCHCMD="sudo scope launch $WHOLE_IPLIST"
#echo $LAUNCHCMD

echo "Launching Weavescope for master node..."
curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/infra/$LORDInfra/vm/$LORDVM -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "[${LAUNCHCMD}]"
	}
EOF

echo ""
echo "[Infra Weavescope: master node only] Access to"
echo " $LORDIP:4040/#!/state/{\"contrastMode\":true,\"topologyId\":\"containers-by-hostname\"}"
echo ""

LAUNCHCMD="sudo scope launch $LORDIP $WHOLE_PRIVIPLIST"
echo "Working on clustring..."

for InfraID in "$@"; do

    echo "Launching Weavescope for the other nodes..."
    curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/infra/$InfraID -H 'Content-Type: application/json' -d @- <<EOF
        {
        "command"        : "[${LAUNCHCMD}]"
        }
EOF

done

echo "Done!"
duration=$SECONDS

printElapsed $@
echo ""

echo "[Infra Weavescope: complete cluster] Access to"
echo " $LORDIP:4040/#!/state/{\"contrastMode\":true,\"topologyId\":\"containers-by-hostname\"}"
echo ""
