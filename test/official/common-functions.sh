#!/bin/bash

function dozing()
{
	duration=$1
	printf "Dozing for %s : " $duration
	for (( i=1; i<=$duration; i++ ))
	do
		printf "%s " $i
		sleep 1
	done
	echo "(Back to work)"
}

function getCloudIndex()
{
	local CSP=$1

	if [ "${CSP}" == "all" ]; then
		echo "[Test for all CSP regions (AWS, Azure, GCP, Alibaba,  ...)]"
		CSP="aws"
		INDEX=0
	elif [ "${CSP}" == "aws" ]; then
		echo "[Test for AWS]"
		INDEX=1
	elif [ "${CSP}" == "azure" ]; then
		echo "[Test for Azure]"
		INDEX=2
	elif [ "${CSP}" == "gcp" ]; then
		echo "[Test for GCP]"
		INDEX=3
	elif [ "${CSP}" == "alibaba" ]; then
		echo "[Test for Alibaba]"
		INDEX=4
	elif [ "${CSP}" == "mock" ]; then
		echo "[Test for Mock driver]"
		INDEX=5
	elif [ "${CSP}" == "openstack" ]; then
		echo "[Test for OpenStack driver]"
		INDEX=6
	else
		echo "[No acceptable argument was provided (all, aws, azure, gcp, alibaba, mock, openstack, ...).]"
		exit
	fi

}