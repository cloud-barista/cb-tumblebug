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
	POSTFIX=${3:-dev01}
	TestSetFile=${4:-../testSet.env}
}

function readParametersByName() {

	CSP="all"
	REGION="1"
	POSTFIX="dev01"
	TestSetFile="../testSet.env"
	OPTION01=""
	OPTION02=""
	OPTION03=""

	FlagNamedPram=""

	echo ""
	# Update values for network parameters by named input parameters (i, c)
	while getopts ":n:f:c:r:x:y:z:h:" opt; do
		FlagNamedPram="yes"
		case $opt in
		n)
			POSTFIX="$OPTARG"
			;;
		f)
			TestSetFile="$OPTARG"
			;;
		c)
			CSP="$OPTARG"
			;;
		r)
			REGION="$OPTARG"
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
			echo "[Usage] ./${0##*/} -param value -param value"
			echo "[Example] ./${0##*/} -n myname01 -f ../testSet.env"
			echo ""
			echo " -n [postfix of resources to generate and retrieve] (ex: -n myname01, default: $POSTFIX)"
			echo " -f [file path to describe a cloud test set] (ex: -f ../testSet01.env, default: $TestSetFile)"
			echo ""
			echo " -c [specific cloud type] (ex: -c aws, optional)"
			echo " -r [index of a specific zone of the cloud type] (ex: -r 3, optional)"
			echo ""
			echo " -x [any value passed to a parameter of the command] (ex: -x vmid01, optional)"
			echo " -y [any value passed to a parameter of the command] (ex: -y 3, optional)"
			echo " -z [any value passed to a parameter of the command] (ex: -z file, optional)"
			exit 0
			;;
		\?)
			echo "[Warrning] Invalid option [$@]. -$OPTARG is not an option" >&2
			echo ""
			echo "[Usage] ./${0##*/} -param value -param value"
			echo "[Example] ./${0##*/} -n myname01 -f ../testSet.env"
			echo ""
			echo " -n [postfix of resources to generate and retrieve] (ex: -n myname01, default: $POSTFIX)"
			echo " -f [file path to describe a cloud test set] (ex: -f ../testSet01.env, default: $TestSetFile)"
			echo ""
			echo " -c [specific cloud type] (ex: -c aws, optional)"
			echo " -r [index of a specific zone of the cloud type] (ex: -r 3, optional)"
			echo ""
			echo " -x [any value passed to a parameter of the command] (ex: -x vmid01, optional)"
			echo " -y [any value passed to a parameter of the command] (ex: -y 3, optional)"
			echo " -z [any value passed to a parameter of the command] (ex: -z file, optional)"
			exit 0
			;;
		esac
	done

	if [ -z "$FlagNamedPram" ]; then
			echo "[Warrning] Invalid option [$@]"
			echo ""
			echo "[Usage] ./${0##*/} -param value -param value"
			echo "[Example] ./${0##*/} -n myname01 -f ../testSet.env"
			echo ""
			echo " -n [postfix of resources to generate and retrieve] (ex: -n myname01, default: $POSTFIX)"
			echo " -f [file path to describe a cloud test set] (ex: -f ../testSet01.env, default: $TestSetFile)"
			echo ""
			echo " -c [specific cloud type] (ex: -c aws, optional)"
			echo " -r [index of a specific zone of the cloud type] (ex: -r 3, optional)"
			echo ""
			echo " -x [any value passed to a parameter of the command] (ex: -x vmid01, optional)"
			echo " -y [any value passed to a parameter of the command] (ex: -y 3, optional)"
			echo " -z [any value passed to a parameter of the command] (ex: -z file, optional)"
		exit 0
	fi

	echo "[Input parameters]"
	echo "[POSTFIX: $POSTFIX] | [TestSetFile: $TestSetFile] | [CSP: $CSP] | [REGION: $REGION]"
	echo "[OPTION01: $OPTION01] | [OPTION02: $OPTION02] | [OPTION03: $OPTION03]"

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
