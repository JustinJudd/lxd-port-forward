# Systemd init file for LXD Port Forward

The following steps will enable LXD Port Forward to run as a systemd init job that watches LXD containers and dynamically sets up port forwarding rules.

* Create folder for config file: `mkdir /etc/lxd-port-forward`
* Save YAML config file to `/etc/lxd-port-forward/config.yaml`
* Copy binary to `/usr/local/bin`
* Install the systemd init file: `cp lxd-port-forward.service /etc/systemd/system/`
* Reload the systemd daemon: `systemctl daemon-reload`
* Enable lxd-port-forward service to autostart on boot: `systemctl enable lxd-port-forward`
* Start lxd-port-forward service: `systemctl start lxd-port-forward`
