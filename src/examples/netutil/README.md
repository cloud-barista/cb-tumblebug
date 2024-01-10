## `netutil` example

Thia is an example of `netutil` package.
(see [netutil package](https://github.com/cloud-barista/cb-tumblebug/blob/main/src/core/common/netutil/netutil.go))

### How to use this example

**Move to the example directory**

```bash
cd PROJECT_ROOT_DIR/src/examples/netutil
```

**Build this example**

```bash
go build .
```

**See help**
```bash
./netutil --help
```

```
This program demonstrates the usage of the netutil package.

Usage:
  ./netutil [flags]

Examples:
./netutil --cidr "10.0.0.0/16" --minsubnets 4 --hosts 500
or
./netutil -c "10.0.0.0/16" -s 4 -n 500

Flags:
  -c, --cidr string      Base network CIDR block (default "192.168.0.0/16")
  -h, --help             help for ./netutil
  -n, --hosts int        Number of hosts per subnet (default 500)
  -s, --minsubnets int   Minimum number of subnets required (default 4)
```

**Run example**
```bash
./netutil
```
```bash
./netutil -c "10.0.0.0/16" -s 4 -n 500
```
```bash
./netutil --cidr "10.0.0.0/16" --minsubnets 4 --hosts 500
```
