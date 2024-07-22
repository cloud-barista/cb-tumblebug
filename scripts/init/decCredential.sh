#!/bin/bash

# Define colors for output
RED='\033[0;31m'
LGREEN='\033[1;32m'
PURPLE='\033[0;35m'
NC='\033[0m' # No Color
CYAN='\033[0;36m'
YELLOW='\033[1;33m'

CRED_FILE_NAME="credentials.yaml"
CRED_PATH="$HOME/.cloud-barista"
FILE_PATH="$CRED_PATH/$CRED_FILE_NAME"
ENCRYPTED_FILE="$FILE_PATH.enc"
TEMP_DECRYPTED_FILE="$FILE_PATH.tmp"
KEY_FILE="$CRED_PATH/cred_key"

# Check if OpenSSL is installed
if ! command -v openssl &> /dev/null; then
    echo -e "\n${RED}OpenSSL is not installed. Installation guide:${NC}"
    echo -e "${LGREEN}Ubuntu/Debian:${NC} sudo apt-get install openssl"
    echo -e "${LGREEN}CentOS/RHEL:${NC} sudo yum install openssl"
    echo -e "${LGREEN}Fedora:${NC} sudo dnf install openssl"
    echo -e "${LGREEN}Arch Linux:${NC} sudo pacman -S openssl\n"
    exit 1
fi

# Check if the file is already decrypted
if [ -f "$FILE_PATH" ]; then
    echo -e "\n${RED}The file is already decrypted.${NC}\n"
    exit 0
fi

# Check if the encrypted file exists
if [ ! -f "$ENCRYPTED_FILE" ]; then
    echo -e "\n${RED}The encrypted file does not exist: ${CYAN}$ENCRYPTED_FILE${NC}\n"
    exit 1
fi

# Prompt for password or use the key file
if [ -f "$KEY_FILE" ]; then
    TB_CRED_DECRYPT_KEY=$(cat "$KEY_FILE")
else
    read -sp "Enter the password: " PASSWORD
    echo

    if [ -z "$PASSWORD" ]; then
        echo -e "\n${RED}Password is required.${NC}\n"
        exit 1
    fi

    # Use the entered password
    TB_CRED_DECRYPT_KEY=$PASSWORD
fi

# Decrypt the file to a temporary file, suppressing OpenSSL error messages
DECRYPT_OUTPUT=$(openssl enc -aes-256-cbc -d -pbkdf2 -in "$ENCRYPTED_FILE" -out "$TEMP_DECRYPTED_FILE" -pass pass:"$TB_CRED_DECRYPT_KEY" 2>&1)

# Check if decryption was successful
if [ $? -eq 0 ]; then
    mv "$TEMP_DECRYPTED_FILE" "$FILE_PATH"
    rm "$ENCRYPTED_FILE"
    echo -e "\n${LGREEN}File successfully decrypted: ${CYAN}$FILE_PATH${NC}"
    echo -e "(Encrypted file deleted: $ENCRYPTED_FILE)\n"
else
    echo -e "\n${RED}Failed to decrypt the file. Exiting.${NC}\n"
    echo -e "${RED}log: ${DECRYPT_OUTPUT}${NC}\n"
    rm -f "$TEMP_DECRYPTED_FILE"  # Remove the temporary file if decryption failed
    exit 1
fi

