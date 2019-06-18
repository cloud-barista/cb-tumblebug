# Proof of Concepts for the Cloud-Barista Multi-Cloud Project.
#      * Cloud-Barista: https://github.com/cloud-barista
#
# by powerkim@powerkim.co.kr, 2019.03.

protoc -I grpc_def/ grpc_def/farmoni_agent.proto --go_out=plugins=grpc:grpc_def
