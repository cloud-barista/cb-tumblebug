#!/bin/bash

function dozing() {
	duration=$1
	printf "Dozing for %s : " $duration
	for ((i = 1; i <= $duration; i++)); do
		printf "%s " $i
		sleep 1
	done
	echo "(Finish dozing. Back to work)"
}

function readParametersByName() {
	CSP="all"
	REGION="1"
	POSTFIX="user"
	TestSetFile="../testSet.env"
	OPTION01=""
	OPTION02=""
	OPTION03=""

	# Update values for network parameters by named input parameters (i, c)
	while getopts ":f:n:x:y:z:h:" opt; do
		case $opt in
		f)
			TestSetFile="$OPTARG"
			;;
		n)
			POSTFIX="$OPTARG"
			;;
		x)
			OPTION01="$OPTARG"
			;;
		y)
			OPTION02="$OPTARG"
			;;
		z)
			OPTION03="$OPTARG"
			;;
		h)
			echo "How to use '-h' (ex: ./${0##*/} -c ../testSet.env -n myname)"
			exit 0
			;;
		\?)
			echo "Invalid option -$OPTARG (Use: -i for NETWORK_INTERFACE, -c for POD_NETWORK_CIDR)" >&2
			exit 0
			;;
		esac
	done

	echo "[Input parameters]"
	echo "CSP: $CSP"
	echo "REGION: $REGION"
	echo "POSTFIX: $POSTFIX"
	echo "OPTION01: $OPTION01"
	echo "OPTION02: $OPTION02"
	echo "OPTION03: $OPTION03"


}

function readParameters() {
	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}

	echo "[Input parameters]"
	echo "CSP: $CSP, REGION: $REGION, POSTFIX: $POSTFIX,"
}

function printElapsed() {
	echo ""
	NowHist="[DATE: $(date +'%d/%m/%Y %H:%M:%S')]"
	CommandHist="[Command: $0 $@]"
	ElapsedHist="[ElapsedTime: ${SECONDS}s ($(($SECONDS / 60))m:$(($SECONDS % 60))s)]"
	echo "${NowHist} ${ElapsedHist} ${CommandHist}"
	echo "${NowHist} ${ElapsedHist} ${CommandHist}" >>./executionStatus.history
}

# function getCloudIndex()
# {
# 	local CSP=$1

# 	if [ "${CSP}" == "all" ]; then
# 		echo "[For all CSP regions (AWS, Azure, GCP, Alibaba,  ...)]"
# 		CSP="aws"
# 		INDEX=0
# 	elif [ "${CSP}" == "aws" ]; then
# 		echo "[For AWS]"
# 		INDEX=1
# 	elif [ "${CSP}" == "alibaba" ]; then
# 		echo "[For Alibaba]"
# 		INDEX=2
# 	elif [ "${CSP}" == "gcp" ]; then
# 		echo "[For GCP]"
# 		INDEX=3
# 	elif [ "${CSP}" == "azure" ]; then
# 		echo "[For Azure]"
# 		INDEX=4
# 	elif [ "${CSP}" == "mock" ]; then
# 		echo "[For Mock driver]"
# 		INDEX=5
# 	elif [ "${CSP}" == "openstack" ]; then
# 		echo "[For OpenStack driver]"
# 		INDEX=6
# 	elif [ "${CSP}" == "ncp" ]; then
# 		echo "[For NCP driver]"
# 		INDEX=7
# 	else
# 		echo "[No acceptable argument was provided (all, aws, azure, gcp, alibaba, mock, openstack, ...).]"
# 		exit
# 	fi

# }
