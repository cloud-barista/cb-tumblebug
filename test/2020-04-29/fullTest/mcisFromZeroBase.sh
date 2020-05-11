#!/bin/bash
source ../conf.env
source ../credentials.conf

echo "####################################################################"
echo "## Create MCIS from Zero Base"
echo "####################################################################"

CSP=${1}
POSTFIX=${2:-developer}
if [ "${CSP}" == "aws" ]; then
	echo "[Test for AWS]"
	INDEX=1
elif [ "${CSP}" == "azure" ]; then
	echo "[Test for Azure]"
	INDEX=2
elif [ "${CSP}" == "gcp" ]; then
	echo "[Test for GCP]"
	INDEX=3
else
	echo "[No acceptable argument was provided (aws, azure, gcp, ..). Default: Test for AWS]"
	CSP="aws"
	INDEX=1
fi

../0.settingSpider/register-cloud.sh $CSP $POSTFIX
../0.settingTB/create-ns.sh $CSP $POSTFIX
../1.vNet/create-vNet.sh $CSP $POSTFIX
../2.securityGroup/create-securityGroup.sh $CSP $POSTFIX
../3.sshKey/create-sshKey.sh $CSP $POSTFIX
../4.image/register-image.sh $CSP $POSTFIX
../5.spec/register-spec.sh $CSP $POSTFIX
../6.mcis/create-mcis.sh $CSP $POSTFIX
echo "============== sleep 10 to check MCIS : Start"
sleep 10
echo "============== sleep 10 to check MCIS : End"
../6.mcis/status-mcis.sh $CSP $POSTFIX



