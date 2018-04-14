package main

import (
	"flag"
	"fmt"
	"strconv"
	"strings"

	"github.com/justinjudd/lxd-port-forward/forward"
)

var (
	daemonize  bool
	enable     bool
	container  string
	portList   string
	configFile string
)

func main() {

	flag.BoolVar(&daemonize, "daemon", false, "Run in daemon mode")
	flag.BoolVar(&enable, "enable", true, "Enable port forwarding if true")
	flag.StringVar(&container, "container", "", "Name of container to forward ports to. Expects --ports to be provided.")
	flag.StringVar(&portList, "ports", "", "Ports to forward and to forward to in the following format protocol://HostPort1:ContainerPort1,HostPort2:ContainerPort2. Expects --container to be provided.")
	flag.StringVar(&configFile, "config", "config.yaml", "Port Forwarding config file in YAML format; default option for container and port mappings")

	flag.Parse()

	config := forward.NewConfig()

	if len(container) > 0 || len(portList) > 0 {
		if len(container) == 0 {
			fmt.Println("Container must be provided if ports are provided")
			return
		}
		if len(portList) == 0 {
			fmt.Println("Ports must be provided if container is provided")
			return
		}

		forwards := forward.NewPortMappings()
		forwards.Protocol = strings.Split(portList, "://")[0]

		portList = portList[len(forwards.Protocol)+3:]
		if len(portList) == 0 {
			fmt.Println("No ports provided")
			return
		}
		for _, ports := range strings.Split(portList, ",") {
			split := strings.Split(ports, ":")
			if len(split) != 2 {
				fmt.Println("Invalid port map")
				return
			}
			_, err := strconv.Atoi(split[0])
			if err != nil {
				fmt.Printf("Port provided is not a valid number %s", split[0])
				return
			}
			containerPort, err := strconv.Atoi(split[1])
			if err != nil {
				fmt.Printf("Port provided is not a valid number %s", split[1])
				return
			}
			forwards.Ports[split[0]] = containerPort
		}
		config.Forwards[container] = []forward.PortMappings{forwards}
	} else {
		var err error
		config, err = forward.LoadYAMLConfig(configFile)
		if err != nil {
			fmt.Println(err)
			return
		}
	}
	forwarder, err := forward.NewForwarder(config)
	if err != nil {
		fmt.Println("Unable to create forwarding client", err)
		return
	}

	if daemonize {
		err := forwarder.Forward()
		if err != nil {
			fmt.Println("Error with initial forwarding of ports ", err)
		}
		forwarder.Watch()
	} else if enable {
		err := forwarder.Forward()
		if err != nil {
			fmt.Println("Unable to enable port forwarding rules", err)
			return
		}
	} else {
		err := forwarder.Reverse()
		if err != nil {
			fmt.Println("Unable to disable port forwarding rules", err)
			return
		}
	}

}
