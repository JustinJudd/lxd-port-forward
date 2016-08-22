package forward

import "fmt"

// IPVersion is used to modify IPTables rules as needed for iptables vs ip6tables
type IPVersion int

const (
	//IPv4 is for rules to iptables
	IPv4 IPVersion = iota
	//IPv6 is for rules to ip6tables
	IPv6
)

// Direction is used to define the direction of traffic aka if the host is the source or destination of traffic
type Direction int

const (
	// Dst is for traffic where the host is the destination
	Dst Direction = iota
	// Src is fot when the host is the source
	Src
)

func (d Direction) String() string {
	switch d {
	case Dst:
		return "dst"
	case Src:
		return "src"
	default:
		return "unknown"
	}
}

// getChain returns the custom IPTables chain that should be used for the rules for a container
func getChain(container string, dir Direction) string {
	return fmt.Sprintf("LXD-%s-%s", container, dir)
}

func getChainForwardRule(container string, ipVersion IPVersion, dir Direction) []string {
	chain := getChain(container, dir)
	switch dir {
	case Dst:
		return getDestinationChainForwardRule(chain)

	case Src:
		return getSourceChainForwardRule(chain, ipVersion)

	default:
		return []string{}
	}
}

func getSourceChainForwardRule(chain string, ipVersion IPVersion) []string {
	switch ipVersion {
	case IPv4:
		return []string{
			"-s", "127.0.0.0/8",
			"!", "-d", "127.0.0.0/8",
			"-j", chain,
		}

	case IPv6:
		return []string{
			"-s", "::1",
			"!", "-d", "::1",
			"-j", chain,
		}

	default:
		return []string{}

	}

}

func getDestinationChainForwardRule(chain string) []string {
	return []string{
		"-m", "addrtype",
		"--dst-type", "LOCAL",
		"-j", chain,
	}
}

func getPortForwardRule(protocol, containerIP, containerPort, hostPort string, ipVersion IPVersion, dir Direction) []string {
	switch dir {
	case Dst:
		return getDestinationPortForwardRule(protocol, containerIP, containerPort, hostPort, ipVersion)

	case Src:
		return getSourcePortForwardRule(protocol, containerIP, containerPort, ipVersion)

	default:
		return []string{}
	}
}

func getSourcePortForwardRule(protocol, containerIP, containerPort string, ipVersion IPVersion) []string {

	switch ipVersion {
	case IPv4:
		return []string{
			"-p", protocol,
			"-s", "127.0.0.0/8",
			"-d", containerIP,
			"--dport", containerPort,
			"-j", "MASQUERADE",
		}

	case IPv6:
		return []string{
			"-p", protocol,
			"-s", "::1",
			"-d", containerIP,
			"--dport", containerPort,
			"-j", "MASQUERADE",
		}

	default:
		return []string{}

	}

}

func getDestinationPortForwardRule(protocol, containerIP, containerPort, hostPort string, ipVersion IPVersion) []string {

	switch ipVersion {
	case IPv4:
		return []string{
			//"-i", iface,
			"-p", protocol,
			"--dport", hostPort,
			"-j", "DNAT",
			"--to", fmt.Sprintf("%s:%s", containerIP, containerPort),
		}

	case IPv6:
		return []string{
			//"-i", iface,
			"-p", protocol,
			"--dport", hostPort,
			"-j", "DNAT",
			"--to", fmt.Sprintf("[%s]:%s", containerIP, containerPort),
		}

	default:
		return []string{}

	}

}
