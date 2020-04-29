#!/bin/bash

export CONN_CONFIG=aws-us-east-1-config
export IMAGE_NAME=ami-085925f297f89fce1
export SPEC_NAME=t3.micro

./full_test.sh
