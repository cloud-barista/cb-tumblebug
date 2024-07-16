#!/bin/bash

CRED_FILE_NAME="credentials.yaml"
CRED_PATH="$HOME/.cloud-barista"
FILE_PATH="$CRED_PATH/$CRED_FILE_NAME"
ENCRYPTED_FILE="$FILE_PATH.enc"
TEMP_DECRYPTED_FILE="$FILE_PATH.tmp"

# Check if OpenSSL is installed
if ! command -v openssl &> /dev/null; then
    echo "OpenSSL is not installed. Installation guide:"
    echo "Ubuntu/Debian: sudo apt-get install openssl"
    echo "CentOS/RHEL: sudo yum install openssl"
    echo "Fedora: sudo dnf install openssl"
    echo "Arch Linux: sudo pacman -S openssl"
    exit 1
fi

# Check if the file is already decrypted
if [ -f "$FILE_PATH" ]; then
    echo "The file is already decrypted."
    exit 0
fi

# Check if the encrypted file exists
if [ ! -f "$ENCRYPTED_FILE" ]; then
    echo "The encrypted file does not exist: $ENCRYPTED_FILE"
    exit 1
fi

# Prompt for password or use the environment variable
if [ -z "$TB_CRED_DECRYPT_KEY" ]; then
    read -sp "Enter the password: " PASSWORD
    echo

    if [ -z "$PASSWORD" ]; then
        echo "Password is required."
        exit 1
    fi

    # Set the environment variable for the decryption key
    TB_CRED_DECRYPT_KEY=$PASSWORD
else
    echo "Using key from environment variable TB_CRED_DECRYPT_KEY"
fi

# Decrypt the file to a temporary file, suppressing OpenSSL error messages
DECRYPT_OUTPUT=$(openssl enc -aes-256-cbc -d -pbkdf2 -in "$ENCRYPTED_FILE" -out "$TEMP_DECRYPTED_FILE" -pass pass:"$TB_CRED_DECRYPT_KEY" 2>&1)

# Check if decryption was successful
if [ $? -eq 0 ]; then
    mv "$TEMP_DECRYPTED_FILE" "$FILE_PATH"
    rm "$ENCRYPTED_FILE"
    echo "File successfully decrypted: $FILE_PATH"
    echo "Encrypted file deleted: $ENCRYPTED_FILE"
else
    echo "Failed to decrypt the file. Exiting."
    rm -f "$TEMP_DECRYPTED_FILE"  # Remove the temporary file if decryption failed
    exit 1
fi
