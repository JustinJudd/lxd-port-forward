# lxd-port-forward

[![GoDoc](https://godoc.org/dev.justinjudd.org/justin/lxd-port-forward?status.svg)](https://godoc.org/dev.justinjudd.org/justin/lxd-port-forward)

Forward ports from an LXD host to containers. Supports a command line interface as well as a daemon option that can watch as containers spin up and down and adjust port forwarding rules accordingly.

## Download

The latest binaries can be downloaded from the [releases page](https://dev.justinjudd.org/justin/lxd-port-forward/releases).

You can grab the go library with  
 `go get dev.justinjudd.org/justin/lxd-port-forward/forward`

## Usage Guidance

The config file format is yaml in the following format and should be saved as `config.yaml`.

``` yaml
---
container1:
- protocol: tcp
  "80": 80
  "443": 443
---
```
The above config file would map standard http and https ports from the LXD host to the container with the name `container1`.

The command line option could then be used as follows.
`./lxd-port-forward`
 to enable the port forwarding rules.
While `./lxd-port-forward enable=false` to tear down the port forwarding rules.

LXD Port Forward can also run in daemon mode by calling `./lxd-port-forward --daemon`. It will read the `config.yaml` file, enable any port forwarding rules for already active containers, and then watch if listed containers are brought up or down, and add or remove port forwarding rules accordingly.

Systemd support can be found in the [init subdirectory](https://dev.justinjudd.org/justin/lxd-port-forward/src/master/init) so that LXD Port Forward can be run as a service.
