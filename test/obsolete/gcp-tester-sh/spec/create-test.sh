#!/bin/bash
source ../setup.env

for NAME in "${CONNECT_NAMES[@]}"
do
        curl -sX POST http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/spec -H 'Content-Type: application/json' -d '{"connectionName":"'$NAME'", 
        "name": "f1-micro",
    "os_type": "",
    "num_vCPU": "",
    "num_core": "",
    "mem_GiB": "",
    "storage_GiB": "",
    "description": "",
    "cost_per_hour": "",
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
