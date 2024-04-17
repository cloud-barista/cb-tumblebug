

### Overview

This test CLI includes to:
- create an MCIS,
- create VPN tunnels,
- delete the VPN tunnels, and
- delete the MCIS.

#### Getting started

##### Setup environment variables
Note - `$CBTUMBLEBUG_ROOT` means your root directory of CB-Tumblebug
```bash
cd $CBTUMBLEBUG_ROOT
source conf/setup.env
```

##### Build the app
```bash
cd $CBTUMBLEBUG_ROOT/testclient/test-clis/vpn-tunnel/gcp-aws
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
## [Demo] This program demonstrates VPN tunnel configuration on MCIS. ##
########################################################################

Usage:
  ./demo [command]

Examples:

  [Long] ./demo --namespaceId "ns01" --mcisId "mcis01" --resourceGroupId "rg01"
  [Short] ./demo -n "ns01" -m "mcis01" -r "rg01"

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
  mcis        Create MCIS dynamically
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
  mcis        Suspend and terminate MCIS
  vpn         Destroy GCP to AWS VPN tunnel

Flags:
  -h, --help   help for delete

Use "./demo delete [command] --help" for more information about a command.
```
