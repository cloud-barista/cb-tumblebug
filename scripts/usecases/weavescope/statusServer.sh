#!/bin/bash

PS=$(ps -ef | head -1; ps -ef | grep scope)
echo "[Process status of scope]"
echo "$PS"

echo ""
