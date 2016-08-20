package forward

import (
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"

	"github.com/coreos/go-iptables/iptables"
	"github.com/lxc/lxd"
	"gopkg.in/yaml.v2"
)

// PortMappings contains information for mapping ports from a host to a container
type PortMappings struct {
	// Name of the container - May be left empty in YAML config file
	Name string `yaml:"name,omitempty"`
	// Protocol should be "tcp" or "udp"
	Protocol string `yaml:"protocol"`
	// Ports is a mapping of host ports as keys to container ports as values
	Ports map[string]int `yaml:",inline"`
}

// NewPortMappings initializes and returns an empty PortMappings struct
func NewPortMappings() PortMappings {

	p := PortMappings{}
	p.Ports = map[string]int{}
	return p
}

// Config represents the Config File format that can be stored in YAML format
type Config struct {
	Forwards map[string][]PortMappings `yaml:",inline"`
}

// NewConfig creates and returns initialized config
func NewConfig() Config {
	c := Config{}
	c.Forwards = map[string][]PortMappings{}
	return c
}

// LoadYAMLConfig loads a YAML Port Forwarding config file and builds the appropriate config
func LoadYAMLConfig(path string) (config Config, err error) {

	yml, err := ioutil.ReadFile(path)
	if err != nil {
		return config, err
	}
	err = yaml.Unmarshal(yml, &config)
	return config, err
}

// Validate checks a config for correctness. Currently provides the following checks:
//	* For each container, makes sure an equal number of Host and Container Ports are provided
//	* Makes sure no Host port is used more than once.
func (c Config) Validate() (bool, error) {
	// First do some sanity checks
	hostPorts := map[string]interface{}{}

	for container, portForwards := range c.Forwards {
		for _, portForward := range portForwards {

			// Make sure that port lists were actually provided
			if len(portForward.Ports) == 0 {
				return false, fmt.Errorf("No ports provided for container %s", container)
			}
			for hPort := range portForward.Ports {
				_, err := strconv.Atoi(hPort)
				if err != nil {
					return false, fmt.Errorf("Invalid port %s provided for container %s", hPort, container)
				}

				// Can only forward a port from the host to one container, check to ensure no duplicate host ports
				fullPort := portForward.Protocol + ":" + hPort
				_, ok := hostPorts[fullPort]
				if ok {
					return false, fmt.Errorf("Port %s has already been mapped", fullPort)
				}
				hostPorts[fullPort] = nil
				portForward.Name = container
			}
		}
	}
	return true, nil
}

// Forwarder represents a port forwarding client that can setup and teardown port forwarding for LXD containers
type Forwarder struct {
	Config
	*lxd.Client
}

const (
	// ContainerStarted matches the text used in monitoring for a Container Starting up
	ContainerStarted = "ContainerStart"

	// ContainerStopped matches the text used in monitoring for a Container shutting down or being stopped
	ContainerStopped = "ContainerStop"

	// IPTable is the table that all IPTable rules should be added to
	IPTable = "nat"
)

// NewForwarder validates the provided config then creates and returns port forward client
func NewForwarder(config Config) (*Forwarder, error) {
	_, err := config.Validate()
	if err != nil {
		return nil, err
	}

	c := Forwarder{}
	c.Client, err = lxd.NewClient(&lxd.DefaultConfig, "local")
	if err != nil {
		return nil, err
	}
	c.Config = config

	return &c, nil
}

// Forward enables forwarding for all containers and port mappings provided in the client config
func (f Forwarder) Forward() error {
	errs := []string{}
	for container := range f.Config.Forwards {
		err := f.ForwardContainer(container)
		if err != nil {
			errs = append(errs, container)
		}
	}

	var err error
	if len(errs) > 0 {
		err = fmt.Errorf("Unable to forward ports for containers %s", strings.Join(errs, ", "))
	}
	return err
}

// Reverse disables forwarding for all containers and port mappings provided in the client config
func (f Forwarder) Reverse() error {
	errs := []string{}
	for container := range f.Config.Forwards {
		err := f.ReverseContainer(container)
		if err != nil {
			errs = append(errs, container)
		}
	}

	var err error
	if len(errs) > 0 {
		err = fmt.Errorf("Unable to remove forwarding of ports for containers %s", strings.Join(errs, ", "))
	}
	return err
}

// ForwardContainer turns on port forwarding for the provided container
// Uses iptables to place ipv4 and ipv6 port forwarding rules
func (f Forwarder) ForwardContainer(container string) error {

	_, ok := f.Config.Forwards[container]
	if !ok {
		return fmt.Errorf("No port rules provided for %s", container)
	}

	state, err := f.ContainerState(container)
	if err != nil {
		return fmt.Errorf("unable to get container state for container %s: %s", container, err)
	}

	// Get list of IP addresses on the container to forward to
	ip4Addresses := []string{}
	ip6Addresses := []string{}
	for name, network := range state.Network {
		if strings.Contains(name, "eth") || strings.Contains(name, "enp") {

			// TODO: Can map interface in container to bridge being used, find standard way to find which interfaces on host bridge is tied to

			for _, address := range network.Addresses {
				switch address.Family {
				case "inet":
					ip4Addresses = append(ip4Addresses, address.Address)

				case "inet6":
					ip6Addresses = append(ip6Addresses, address.Address)

				}
			}

		}

	}

	iptable, err := iptables.New()
	if err != nil {
		return err
	}
	ip6table, err := iptables.NewWithProtocol(iptables.ProtocolIPv6)
	if err != nil {
		return err
	}

	// Create a new custom chain for the IPTable rules for just this container
	customChain := getChain(container)
	err = iptable.NewChain(IPTable, customChain)
	if err != nil {
		return err
	}
	err = ip6table.NewChain(IPTable, customChain)
	if err != nil {
		return err
	}

	// Tell IPTables when to use our custom chain
	err = iptable.Insert(IPTable, "PREROUTING", 1, []string{
		"-m", "addrtype",
		"--dst-type", "LOCAL",
		"-j", customChain,
	}...)
	if err != nil {
		return err
	}
	err = ip6table.Insert(IPTable, "PREROUTING", 1, []string{
		"-m", "addrtype",
		"--dst-type", "LOCAL",
		"-j", customChain,
	}...)
	if err != nil {
		return err
	}

	// Set up rules within the custom chain of the actual port forwardings
	for _, portForwards := range f.Config.Forwards[container] {
		protocol := portForwards.Protocol
		for hostPort, containerPort := range portForwards.Ports {

			for _, address := range ip4Addresses {
				iptable.Append(IPTable, customChain, []string{
					//"-i", iface,
					"-p", protocol,
					"--dport", hostPort,
					"-j", "DNAT",
					"--to", fmt.Sprintf("%s:%d", address, containerPort),
				}...)
			}

			for _, address := range ip6Addresses {
				ip6table.Append(IPTable, customChain, []string{
					//"-i", iface,
					"-p", protocol,
					"--dport", hostPort,
					"-j", "DNAT",
					"--to", fmt.Sprintf("[%s]:%d", address, containerPort),
				}...)
			}

		}

	}
	return nil
}

// ReverseContainer removes port forwarding for the provided container
func (f Forwarder) ReverseContainer(container string) error {
	customChain := getChain(container)
	iptable, err := iptables.New()
	if err != nil {
		return err
	}
	ip6table, err := iptables.NewWithProtocol(iptables.ProtocolIPv6)
	if err != nil {
		return err
	}

	err = iptable.Delete(IPTable, "PREROUTING", []string{
		"-m", "addrtype",
		"--dst-type", "LOCAL",
		"-j", customChain,
	}...)
	if err != nil {
		return err
	}
	err = ip6table.Delete(IPTable, "PREROUTING", []string{
		"-m", "addrtype",
		"--dst-type", "LOCAL",
		"-j", customChain,
	}...)
	if err != nil {
		return err
	}

	iptable.ClearChain(IPTable, customChain)
	iptable.DeleteChain(IPTable, customChain)
	ip6table.ClearChain(IPTable, customChain)
	ip6table.DeleteChain(IPTable, customChain)

	return nil
}

// Watch monitors LXD events and identifies when containers named in the config are stopped or started,
// and disables or enables port forwarding respecitvely
func (f Forwarder) Watch() {
	handler := func(i interface{}) {
		var container string
		var message string
		var context map[string]interface{}
		data := i.(map[string]interface{})
		metadata := data["metadata"].(map[string]interface{})

		tmp, ok := metadata["context"]
		if ok {
			context = tmp.(map[string]interface{})
		}

		tmp, ok = context["container"]
		if ok {
			container = tmp.(string)
		}

		_, ok = f.Forwards[container]
		if ok {
			tmp, ok := metadata["message"]
			if ok {
				message = tmp.(string)
			}
			switch message {
			case ContainerStarted:
				f.ForwardContainer(container)
			case ContainerStopped:
				f.ReverseContainer(container)
			}
		}

	}

	f.Monitor([]string{}, handler)
}

// getChain returns the custom IPTables chain that should be used for the rules for a container
func getChain(container string) string {
	return fmt.Sprintf("LXD-%s", container)
}
