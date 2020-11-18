// Copyright 2020 The gVisor Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package testbench has utilities to send and receive packets, and also command
// the DUT to run POSIX functions. It is the packetimpact test API.
package testbench

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"

	"gvisor.dev/gvisor/test/packetimpact/netdevs"
)

var (
	// Native indicates that the test is being run natively.
	Native = false
	// RPCKeepalive is the gRPC keepalive.
	RPCKeepalive = 10 * time.Second
	// RPCTimeout is the gRPC timeout.
	RPCTimeout = 100 * time.Millisecond

	// dutTestNetsPath is the json file that describes all the test networks to duts available to use.
	dutTestNetsPath string
	// dutTestNets the pool among which for the testbench can choose DUT to work with.
	dutTestNets chan *DUTTestNet

	// TODO: Remove the following variables once the test runner side is ready.
	localDevice       = ""
	remoteDevice      = ""
	localIPv4         = ""
	remoteIPv4        = ""
	ipv4PrefixLength  = 0
	localIPv6         = ""
	remoteIPv6        = ""
	localInterfaceID  uint32
	remoteInterfaceID uint64
	localMAC          = ""
	remoteMAC         = ""
	posixServerIP     = ""
	posixServerPort   = 40000
)

// DUTTestNet describes the test network setup on dut and how the testbench should connect with an existing DUT.
type DUTTestNet struct {
	// LocalMAC is the local MAC address on the test network.
	LocalMAC net.HardwareAddr
	// RemoteMAC is the DUT's MAC address on the test network.
	RemoteMAC net.HardwareAddr
	// LocalIPv4 is the local IPv4 address on the test network.
	LocalIPv4 net.IP
	// RemoteIPv4 is the DUT's IPv4 address on the test network.
	RemoteIPv4 net.IP
	// IPv4PrefixLength is the network prefix length of the IPv4 test network.
	IPv4PrefixLength int
	// LocalIPv6 is the local IPv6 address on the test network.
	LocalIPv6 net.IP
	// RemoteIPv6 is the DUT's IPv6 address on the test network.
	RemoteIPv6 net.IP
	// LocalInterfaceID is the ID of the local interface on the test network.
	LocalDevID uint32
	// RemoteInterfaceID is the ID of the remote interface on the test network.
	RemoteDevID uint32
	// LocalDevName is the device that testbench uses to inject traffic.
	LocalDevName string
	// RemoteDevName is the device name on the DUT, individual tests can
	// use the name to construct tests.
	RemoteDevName string

	// The following two fields on actually on the control network instead
	// of the test network, including them for convenience.

	// POSIXServerIP is the POSIX server's IP address on the control network.
	POSIXServerIP net.IP
	// POSIXServerPort is the UDP port the POSIX server is bound to on the
	// control network.
	POSIXServerPort uint16
}

// RegisterFlags defines flags and associates them with the package-level
// exported variables above. It should be called by tests in their init
// functions.
func RegisterFlags(fs *flag.FlagSet) {
	fs.StringVar(&posixServerIP, "posix_server_ip", posixServerIP, "ip address to listen to for UDP commands")
	fs.IntVar(&posixServerPort, "posix_server_port", posixServerPort, "port to listen to for UDP commands")
	fs.StringVar(&localIPv4, "local_ipv4", localIPv4, "local IPv4 address for test packets")
	fs.StringVar(&remoteIPv4, "remote_ipv4", remoteIPv4, "remote IPv4 address for test packets")
	fs.StringVar(&remoteIPv6, "remote_ipv6", remoteIPv6, "remote IPv6 address for test packets")
	fs.StringVar(&remoteMAC, "remote_mac", remoteMAC, "remote mac address for test packets")
	fs.StringVar(&localDevice, "local_device", localDevice, "local device to inject traffic")
	fs.StringVar(&remoteDevice, "remote_device", remoteDevice, "remote device on the DUT")
	fs.Uint64Var(&remoteInterfaceID, "remote_interface_id", remoteInterfaceID, "remote interface ID for test packets")

	fs.BoolVar(&Native, "native", Native, "whether the test is running natively")
	fs.DurationVar(&RPCTimeout, "rpc_timeout", RPCTimeout, "gRPC timeout")
	fs.DurationVar(&RPCKeepalive, "rpc_keepalive", RPCKeepalive, "gRPC keepalive")
	fs.StringVar(&dutTestNetsPath, "dut_test_nets_path", dutTestNetsPath, "path to the dut test nets json file")
}

// Initialize the testbench.
func Initialize(fs *flag.FlagSet) {
	RegisterFlags(fs)
	flag.Parse()
	if err := loadDUTTestNets(); err != nil {
		panic(err)
	}
}

// loadDUTTestNets loads available DUT test networks from the json file, it must be
// called after flag.Parse().
func loadDUTTestNets() error {
	dutTestNetsFile, err := os.Open(dutTestNetsPath)
	if err != nil {
		return fmt.Errorf("failed to open pool file: %w", err)
	}
	dutTestNetsBytes, err := ioutil.ReadAll(dutTestNetsFile)
	if err != nil {
		return fmt.Errorf("failed to read from pool file: %w", err)
	}
	var parsedTestNets []DUTTestNet
	if err := json.Unmarshal(dutTestNetsBytes, &parsedTestNets); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}
	if len(parsedTestNets) < 1 {
		return fmt.Errorf("doesn't have enough test networks to run the test")
	}
	// Using a buffered channel as semaphore
	dutTestNets = make(chan *DUTTestNet, len(parsedTestNets))
	for i := range parsedTestNets {
		parsedTestNets[i].LocalIPv4 = parsedTestNets[i].LocalIPv4.To4()
		parsedTestNets[i].RemoteIPv4 = parsedTestNets[i].RemoteIPv4.To4()
		dutTestNets <- &parsedTestNets[i]
	}
	return nil
}

// genPseudoFlags populates flag-like global config based on real flags.
//
// genPseudoFlags must only be called after flag.Parse.
func genPseudoFlags() error {
	out, err := exec.Command("ip", "addr", "show").CombinedOutput()
	if err != nil {
		return fmt.Errorf("listing devices: %q: %w", string(out), err)
	}
	devs, err := netdevs.ParseDevices(string(out))
	if err != nil {
		return fmt.Errorf("parsing devices: %w", err)
	}

	_, deviceInfo, err := netdevs.FindDeviceByIP(net.ParseIP(localIPv4), devs)
	if err != nil {
		return fmt.Errorf("can't find deviceInfo: %w", err)
	}

	localMAC = deviceInfo.MAC.String()
	localIPv6 = deviceInfo.IPv6Addr.String()
	localInterfaceID = deviceInfo.ID

	if deviceInfo.IPv4Net != nil {
		ipv4PrefixLength, _ = deviceInfo.IPv4Net.Mask.Size()
	} else {
		ipv4PrefixLength, _ = net.ParseIP(localIPv4).DefaultMask().Size()
	}
	return nil
}

// GenerateRandomPayload generates a random byte slice of the specified length,
// causing a fatal test failure if it is unable to do so.
func GenerateRandomPayload(t *testing.T, n int) []byte {
	t.Helper()
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("rand.Read(buf) failed: %s", err)
	}
	return buf
}

// GetDUTTestNet gets a usable TestNet, the function will block until any
// becomes available.
func GetDUTTestNet() *DUTTestNet {
	return <-dutTestNets
}

// Release releases the TestNet back to the pool so that some other test
// can use.
func (n *DUTTestNet) Release() {
	dutTestNets <- n
}
