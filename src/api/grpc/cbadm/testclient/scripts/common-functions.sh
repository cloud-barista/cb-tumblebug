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