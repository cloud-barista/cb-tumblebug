

### Overview

This test CLI includes to:
- create an MCI,
- create VPN tunnels,
- delete the VPN tunnels, and
- delete the MCI.

#### Getting started

##### Setup environment variables
Note - `$TB_ROOT_PATH` means your root directory of CB-Tumblebug
```bash
cd $TB_ROOT_PATH
source conf/setup.env
```

##### Build the app
```bash
cd $TB_ROOT_PATH/testclient/test-clis/vpn-tunnel/gcp-aws
go build -o app
```

##### How to use

Please refer to the `--help` 

(Sample 1)
```bash
./app --help
```
```
########################################################################
## [Demo] This program demonstrates VPN tunnel configuration on MCI. ##
########################################################################

Usage:
  ./demo [command]

Examples:

  [Long] ./demo --namespaceId "ns01" --mciId "mci01" --resourceGroupId "rg01"
  [Short] ./demo -n "ns01" -m "mci01" -r "rg01"

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  create      Create resources
  delete      Dreate resources
  help        Help about any command

Flags:
  -h, --help   help for ./demo

Use "./demo [command] --help" for more information about a command.
```

(Sample 2)
```bash
./app create --help
```
```
Create resources

Usage:
  ./demo create [command]

Available Commands:
  mci        Create MCI dynamically
  vpn         Create GCP to AWS VPN tunnel

Flags:
  -h, --help   help for create

Use "./demo create [command] --help" for more information about a command.
```

(Sample 3)
```bash
./app delete --help
```
```
Delete resources

Usage:
  ./demo delete [command]

Available Commands:
  mci        Suspend and terminate MCI
  vpn         Destroy GCP to AWS VPN tunnel

Flags:
  -h, --help   help for delete

Use "./demo delete [command] --help" for more information about a command.
```
