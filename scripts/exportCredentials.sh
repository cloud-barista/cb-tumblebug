#!/bin/bash

if [ -z "$CBTUMBLEBUG_ROOT" ]; then
    SCRIPT_DIR=$(dirname "${BASH_SOURCE[0]-$0}")
    export CBTUMBLEBUG_ROOT=$(cd "$SCRIPT_DIR" && cd .. && pwd)
fi

credentialDir="$CBTUMBLEBUG_ROOT/conf"
credentialFile="$credentialDir/credentials.conf"
saveTo="$credentialDir/.credtmp"

# colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "\n${GREEN}Credential Exporter Script${NC}"
echo -e "This script exports credential files based on the provided config from"
echo -e "${BLUE} $credentialFile ${NC}\n"
echo -e "It generates credentials in a format that can be directly used with CSP CLI/Terraform/OpenTofu, facilitating cloud resource management.\n"


printf "${BOLD}"
while true; do
    read -p 'Export credentials. Do you want to proceed ? (y/n) : ' CHECKPROCEED
    printf "${NC}"
    case $CHECKPROCEED in
        [Yy]* ) break;;
        [Nn]* ) 
            printf "\nCancel [$0 $@]\nSee you soon. :)\n\n"
            exit 1;;
        * ) printf "Please answer yes or no.\n";;
    esac
done

mkdir -p "$saveTo"


aws_access_key_id=""
aws_secret_access_key=""

gcp_project_id=""
gcp_client_id=""
gcp_client_email=""
gcp_private_key_id=""
gcp_private_key=""

azure_client_id=""
azure_client_secret=""
azure_tenant_id=""
azure_subscription_id=""

while IFS= read -r line; do
    if [[ $line == *"AWS"* ]]; then
        if [[ $line == *"Val01"* ]]; then
            aws_access_key_id="${line#*=}"
        elif [[ $line == *"Val02"* ]]; then
            aws_secret_access_key="${line#*=}"
        fi
    elif [[ $line == *"GCP"* ]]; then
        if [[ $line == *"Val01"* ]]; then
            gcp_project_id="${line#*=}"
        elif [[ $line == *"Val02"* ]]; then
            gcp_client_id="${line#*=}"
        elif [[ $line == *"Val03"* ]]; then
            gcp_client_email="${line#*=}"
        elif [[ $line == *"Val04"* ]]; then
            gcp_private_key_id="${line#*=}"
        elif [[ $line == *"Val05"* ]]; then
            gcp_private_key="${line#*=}"            
        fi
    elif [[ $line == *"Azure"* ]]; then
        if [[ $line == *"Val01"* ]]; then
            azure_client_id="${line#*=}"
        elif [[ $line == *"Val02"* ]]; then
            azure_client_secret="${line#*=}"
        elif [[ $line == *"Val03"* ]]; then
            azure_tenant_id="${line#*=}"
        elif [[ $line == *"Val04"* ]]; then
            azure_subscription_id="${line#*=}"
        fi
    fi
done < "$credentialFile"


{
    echo "[default]"
    echo "aws_access_key_id=$aws_access_key_id"
    echo "aws_secret_access_key=$aws_secret_access_key"
} > "$saveTo/aws_credential"

cat > "$saveTo/gcp_credential.json" << EOF
{
    "type": "service_account",
    "project_id": "$gcp_project_id",
    "private_key_id": "$gcp_private_key_id",
    "private_key": $gcp_private_key,
    "client_email": "$gcp_client_email",
    "client_id": "$gcp_client_id",
    "auth_uri": "https://accounts.google.com/o/oauth2/auth",
    "token_uri": "https://oauth2.googleapis.com/token",
    "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
    "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/${gcp_client_email//@/%40}",
    "universe_domain": "googleapis.com"
}
EOF

{
    echo "client_id=$azure_client_id"
    echo "client_secret=$azure_client_secret"
    echo "tenant_id=$azure_tenant_id"
    echo "subscription_id=$azure_subscription_id"
} > "$saveTo/azure_credential"


echo -e "${GREEN}\n# AWS Credential${NC}"
cat "$saveTo/aws_credential"
echo -e "${GREEN}\n# GCP Credential${NC}"
cat "$saveTo/gcp_credential.json"
echo -e "${GREEN}\n# Azure Credential${NC}"
cat "$saveTo/azure_credential"

echo -e "\n\n"
echo -e "${GREEN}\nCredential files have been successfully generated and saved to: ${BLUE}$saveTo${NC}"
echo -e "${BLUE} $saveTo/aws_credential${NC}"
echo -e "${BLUE} $saveTo/gcp_credential${NC}"
echo -e "${BLUE} $saveTo/azure_credential${NC}"

echo -e "\n${RED}========================================================================"
echo -e "Guide to Using Generated Credential Files with Terraform/OpenTofu"
echo -e "========================================================================${NC}\n"

echo -e "${GREEN}Terraform/OpenTofu and AWS Credentials:${NC}"
echo -e "---------------------------------------"
echo -e "For Terraform/OpenTofu to use AWS credentials, set the credentials file in the default location (~/.aws/credentials) or specify the file path in your Terraform/OpenTofu configurations."
echo -e "Command example:"
echo -e "${BLUE}cp \"$saveTo/aws_credential\" ~/.aws/credentials${NC}\n"

echo -e "${GREEN}Terraform/OpenTofu and GCP Credentials:${NC}"
echo -e "---------------------------------------"
echo -e "For Terraform/OpenTofu to authenticate with GCP, set the GOOGLE_APPLICATION_CREDENTIALS environment variable to your GCP credentials JSON file."
echo -e "Command example:"
echo -e "${BLUE}export GOOGLE_APPLICATION_CREDENTIALS=\"$saveTo/gcp_credential.json\"${NC}\n"

echo -e "${GREEN}Terraform/OpenTofu and Azure Credentials:${NC}"
echo -e "-----------------------------------------"
echo -e "Terraform/OpenTofu can authenticate with Azure using a service principal or Azure CLI."
echo -e "Command examples:"
echo -e "${BLUE}export ARM_CLIENT_ID=\"$azure_client_id\"${NC}"
echo -e "${BLUE}export ARM_CLIENT_SECRET=\"$azure_client_secret\"${NC}"
echo -e "${BLUE}export ARM_TENANT_ID=\"$azure_tenant_id\"${NC}"
echo -e "${BLUE}export ARM_SUBSCRIPTION_ID=\"$azure_subscription_id\"${NC}\n"

echo -e "${RED}========================================================================${NC}\n"
echo -e "${GREEN}Note: Secure your credential files and avoid exposing sensitive information in your Terraform/OpenTofu configurations or scripts.${NC}"
