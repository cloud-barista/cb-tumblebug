#!/bin/bash

CRED_FILE_NAME="credentials.yaml"
CRED_PATH="$HOME/.cloud-barista"
FILE_PATH="$CRED_PATH/$CRED_FILE_NAME"
ENCRYPTED_FILE="$FILE_PATH.enc"
TEMP_DECRYPTED_FILE="$FILE_PATH.tmp.dec"

# Check if OpenSSL is installed
if ! command -v openssl &> /dev/null; then
    echo "OpenSSL is not installed. Installation guide:"
    echo "Ubuntu/Debian: sudo apt-get install openssl"
    echo "CentOS/RHEL: sudo yum install openssl"
    echo "Fedora: sudo dnf install openssl"
    echo "Arch Linux: sudo pacman -S openssl"
    exit 1
fi

# Check if the file is already encrypted
if [ -f "$ENCRYPTED_FILE" ]; then
    echo "The file is already encrypted."
    exit 0
fi

# Check if the file to be encrypted exists
if [ ! -f "$FILE_PATH" ]; then
    echo "The file to be encrypted does not exist: $FILE_PATH"
    exit 1
fi

# Prompt to proceed with encryption
read -p "Do you want to encrypt the file $FILE_PATH? (y/n): " CONFIRM
if [ "$CONFIRM" != "y" ]; then
    echo "Encryption process aborted."
    exit 0
fi

# Prompt for password
read -sp "Enter a password (press enter to generate a random key): " PASSWORD
echo
if [ -n "$PASSWORD" ]; then
    read -sp "Confirm the password: " PASSWORD_CONFIRM
    echo
    if [ "$PASSWORD" != "$PASSWORD_CONFIRM" ]; then
        echo "Passwords do not match. Encryption aborted."
        exit 1
    fi
fi

if [ -z "$PASSWORD" ]; then
    # Generate a random key
    TB_CRED_DECRYPT_KEY=$(openssl rand -base64 64 | tr -d '\n')
else
    TB_CRED_DECRYPT_KEY=$PASSWORD
fi

# Encrypt the file
openssl enc -aes-256-cbc -salt -pbkdf2 -in "$FILE_PATH" -out "$ENCRYPTED_FILE" -pass pass:"$TB_CRED_DECRYPT_KEY"

if [ $? -eq 0 ]; then
    # Verify encryption by decrypting the file to a temporary file
    openssl enc -aes-256-cbc -d -pbkdf2 -in "$ENCRYPTED_FILE" -out "$TEMP_DECRYPTED_FILE" -pass pass:"$TB_CRED_DECRYPT_KEY"
    if [ $? -eq 0 ] && cmp -s "$FILE_PATH" "$TEMP_DECRYPTED_FILE"; then
        rm "$TEMP_DECRYPTED_FILE"
        rm "$FILE_PATH"
        echo "File successfully encrypted: $ENCRYPTED_FILE"
        echo "Original file deleted: $FILE_PATH"
        if [ -z "$PASSWORD" ]; then
            echo "Your encryption key is: $TB_CRED_DECRYPT_KEY"
            echo "To decrypt the file, export the key as an environment variable:"
            echo "export TB_CRED_DECRYPT_KEY=\"$TB_CRED_DECRYPT_KEY\""
        fi
    else
        echo "Encryption verification failed."
        if [ $? -ne 0 ]; then
            echo "Decryption failed during verification."
        else
            echo "File comparison failed during verification."
        fi
        rm "$TEMP_DECRYPTED_FILE"
    fi
else
    echo "Failed to encrypt the file."
fi
