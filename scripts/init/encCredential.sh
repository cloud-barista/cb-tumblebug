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
TEMP_DECRYPTED_FILE="$FILE_PATH.tmp.dec"
KEY_FILE="$CRED_PATH/cred_key"
SCRIPT_DIR=$(dirname "$(realpath "$0")")
DECRYPT_SCRIPT_PATH="$SCRIPT_DIR/decCredential.sh"

# Check if OpenSSL is installed
if ! command -v openssl &> /dev/null; then
    echo -e "\n${RED}OpenSSL is not installed. Installation guide:${NC}"
    echo -e "${LGREEN}Ubuntu/Debian:${NC} sudo apt-get install openssl"
    echo -e "${LGREEN}CentOS/RHEL:${NC} sudo yum install openssl"
    echo -e "${LGREEN}Fedora:${NC} sudo dnf install openssl"
    echo -e "${LGREEN}Arch Linux:${NC} sudo pacman -S openssl\n"
    exit 1
fi

# Check if the file is already encrypted
if [ -f "$ENCRYPTED_FILE" ]; then
    echo -e "\n${RED}The file is already encrypted.${NC}\n"
    exit 0
fi

# Check if the file to be encrypted exists
if [ ! -f "$FILE_PATH" ]; then
    echo -e "\n${RED}The file to be encrypted does not exist: ${CYAN}$FILE_PATH${NC}\n"
    exit 1
fi

# Prompt to proceed with encryption
while true; do
    echo -e "\nDo you want to encrypt the file ${CYAN}$FILE_PATH${NC}? (y/n): \c"
    read -e CONFIRM
    case $CONFIRM in
        [Yy]* )
            break
            ;;
        [Nn]* )
            echo -e "\n${RED}Encryption process aborted.${NC}\n"
            exit 0
            ;;
        * )
            echo -e "\n${RED}Please answer yes or no.${NC}\n"
            ;;
    esac
done

# Prompt for password
read -sp "Enter a password (press enter to generate a random key): " PASSWORD
echo
if [ -n "$PASSWORD" ]; then
    read -sp "Confirm the password: " PASSWORD_CONFIRM
    echo
    if [ "$PASSWORD" != "$PASSWORD_CONFIRM" ]; then
        echo -e "\n${RED}Passwords do not match. Encryption aborted.${NC}\n"
        exit 1
    fi
    TB_CRED_DECRYPT_KEY=$PASSWORD
    # Delete the existing key file if any
    if [ -f "$KEY_FILE" ]; then
        rm "$KEY_FILE"
    fi
    echo -e "\n${YELLOW}Remember the password you have entered. You will need it to decrypt the file.${NC}\n"
else
    # Generate a random key
    TB_CRED_DECRYPT_KEY=$(openssl rand -base64 64 | tr -d '\n')
    echo "$TB_CRED_DECRYPT_KEY" > "$KEY_FILE"
    echo -e "\n${YELLOW}A random encryption key was generated and saved to ${CYAN}$KEY_FILE${NC}"
    echo -e "${YELLOW}This file should be used temporarily. Please store the key securely.${NC}\n"
fi

# Encrypt the file
openssl enc -aes-256-cbc -salt -pbkdf2 -in "$FILE_PATH" -out "$ENCRYPTED_FILE" -pass pass:"$TB_CRED_DECRYPT_KEY"

if [ $? -eq 0 ]; then
    # Verify encryption by decrypting the file to a temporary file
    openssl enc -aes-256-cbc -d -pbkdf2 -in "$ENCRYPTED_FILE" -out "$TEMP_DECRYPTED_FILE" -pass pass:"$TB_CRED_DECRYPT_KEY"
    if [ $? -eq 0 ] && cmp -s "$FILE_PATH" "$TEMP_DECRYPTED_FILE"; then
        rm "$TEMP_DECRYPTED_FILE"
        rm "$FILE_PATH"
        echo -e "\n${LGREEN}File successfully encrypted: ${CYAN}$ENCRYPTED_FILE${NC}"
        echo -e "${LGREEN}Original file deleted: ${CYAN}$FILE_PATH${NC}\n"
        echo -e "${YELLOW}To edit the credentials,${NC}"
        echo -e "Use ${CYAN}$DECRYPT_SCRIPT_PATH${NC} to decrypt the file"
        echo -e "Then edit ${CYAN}$FILE_PATH${NC}\n"
    else
        echo -e "\n${RED}Encryption verification failed.${NC}\n"
        if [ $? -ne 0 ]; then
            echo -e "${RED}Decryption failed during verification.${NC}\n"
        else
            echo -e "${RED}File comparison failed during verification.${NC}\n"
        fi
        rm "$TEMP_DECRYPTED_FILE"
    fi
else
    echo -e "\n${RED}Failed to encrypt the file.${NC}\n"
fi
