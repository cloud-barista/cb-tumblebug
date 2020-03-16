#!/bin/bash
source ../setup.env

for NAME in "${CONNECT_NAMES[@]}"
do
	curl -sX POST http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/spec -H 'Content-Type: application/json' -d '{"connectionName":"'$NAME'", 
	"name": "t2.micro",
    "os_type": "ubuntu",
    "num_vCPU": "1",
    "num_core": "",
    "mem_GiB": "1",
    "storage_GiB": "1",
    "description": "",
    "cost_per_hour": "1",
    "num_storage": "",
    "max_num_storage": "",
    "max_total_storage_TiB": "",
    "net_bw_Gbps": "",
    "ebs_bw_Mbps": "",
    "gpu_model": "",
    "num_gpu": "",
    "gpumem_GiB": "",
    "gpu_p2p": ""
}' | json_pp
done
