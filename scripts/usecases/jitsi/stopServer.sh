#!/bin/bash

echo "[Purge Jitsi]"

sudo apt purge jigasi jitsi-meet jitsi-meet-web-config jitsi-meet-prosody jitsi-meet-turnserver jitsi-meet-web jicofo jitsi-videobridge2 -y

PS=$(ps -ef | head -1; ps -ef | grep jitsi)
echo "[Process status of Jitsi]"
echo "$PS"
