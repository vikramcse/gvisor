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

package header_test

import (
	"testing"

	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

// TestIGMPHeader tests the functions within header.igmp
func TestIGMPHeader(t *testing.T) {
	b := []byte{
		0x11,       // IGMP Type, Membership Query
		0xF0,       // Maximum Response Time
		0xC0, 0xC0, // Checksum
		0x01, 0x02, 0x03, 0x04, // Group Address
	}

	igmpHeader := header.IGMP(b)

	if got, want := igmpHeader.Type(), header.IGMPMembershipQuery; got != want {
		t.Errorf("got igmpHeader.Type() = %x, want = %x", got, want)
	}

	if got, want := igmpHeader.MaxRespTime(), byte(0xF0); got != want {
		t.Errorf("got igmpHeader.MaxRespTime() = %x, want = %x", got, want)
	}

	if got, want := igmpHeader.Checksum(), uint16(0xC0C0); got != want {
		t.Errorf("got igmpHeader.Checksum() = %x, want = %x", got, want)
	}

	if got, want := igmpHeader.GroupAddress(), tcpip.Address("\x01\x02\x03\x04"); got != want {
		t.Errorf("got igmpHeader.GroupAddress() = %s, want = %s", got, want)
	}

	igmpType := header.IGMPv2MembershipReport
	igmpHeader.SetType(igmpType)
	if got := igmpHeader.Type(); got != igmpType {
		t.Errorf("got igmpHeader.Type() = %x, want = %x", got, igmpType)
	}
	if got := header.IGMPType(b[0]); got != igmpType {
		t.Errorf("got IGMPtype in backing buffer = %x, want %x", got, igmpType)
	}

	respTime := byte(0x02)
	igmpHeader.SetMaxRespTime(respTime)
	if got := igmpHeader.MaxRespTime(); got != respTime {
		t.Errorf("got igmpHeader.MaxRespTime() = %x, want = %x", got, respTime)
	}

	checksum := uint16(0x0102)
	igmpHeader.SetChecksum(checksum)
	if got := igmpHeader.Checksum(); got != checksum {
		t.Errorf("got igmpHeader.Checksum() = %x, want = %x", got, checksum)
	}

	groupAddress := tcpip.Address("\x04\x03\x02\x01")
	igmpHeader.SetGroupAddress(groupAddress)
	if got := igmpHeader.GroupAddress(); got != groupAddress {
		t.Errorf("got igmpHeader.GroupAddress() = %s, want = %s", got, groupAddress)
	}
}

// TestIGMPChecksum ensures that the checksum calculator produces the expected
// checksum.
func TestIGMPChecksum(t *testing.T) {
	b := []byte{
		0x11,       // IGMP Type, Membership Query
		0xF0,       // Maximum Response Time
		0xC0, 0xC0, // Checksum
		0x01, 0x02, 0x03, 0x04, // Group Address
	}

	igmpHeader := header.IGMP(b)

	// Calculate the initial checksum after setting the checksum temporarily to 0
	// to avoid checksumming the checksum.
	initialChecksum := igmpHeader.Checksum()
	igmpHeader.SetChecksum(0)
	checksum := ^header.Checksum(b, 0)
	igmpHeader.SetChecksum(initialChecksum)

	if got := header.IGMPCalculateChecksum(igmpHeader); got != checksum {
		t.Errorf("got IGMPCalculateChecksum = %x, want %x", got, checksum)
	}
}
