// Copyright (c) 2017 Pani Networks
// All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

package client

import (
	"encoding/json"
	"net"
	"testing"

	"github.com/romana/core/common/api"
)

var (
	testSaver *TestSaver
	ipam      *IPAM
)

func initIpam(t *testing.T, conf string) *IPAM {
	ipam, err := NewIPAM(testSaver.save, nil)
	if err != nil {
		t.Fatal(err)
	}
	topoReq := api.TopologyUpdateRequest{}
	err = json.Unmarshal([]byte(conf), &topoReq)
	if err != nil {
		t.Fatalf("Cannot parse %s: %v", conf, err)
	}
	err = ipam.UpdateTopology(topoReq)
	if err != nil {
		t.Fatal(err)
	}
	return ipam
}

func init() {
	testSaver = &TestSaver{}
}

// TestSaver can be used as the Saver function for IPAM.
// It will store last saved data in lastJson field, which
// can be helpful for debugging.
type TestSaver struct {
	lastJson string
}

func (s *TestSaver) save(ipam *IPAM) error {
	b, err := json.MarshalIndent(ipam, "", "  ")
	if err != nil {
		return err
	}
	s.lastJson = string(b)

	return nil
}

func TestNewCIDR(t *testing.T) {
	cidr, err := NewCIDR("10.0.0.0/8")
	if err != nil {
		t.Fatal(err)
	}

	if cidr.StartIP.String() != "10.0.0.0" {
		t.Fatalf("Expected start to be 10.0.0.0, got %s", cidr.StartIP)
	}

	if cidr.EndIP.String() != "10.255.255.255" {
		t.Fatalf("Expected start to be 10.255.255.255 got %s", cidr.StartIP)
	}
}

func TestBlackout(t *testing.T) {
	conf := `{
  "networks":[
    {
      "name":"net1",
      "cidr":"10.0.0.0/30",
      "block_mask":30
    }
  ],
  "topologies":[
    {
      "networks":[
        "net1"
      ],
      "map":[
        {
          "routing":"foo",
          "groups":[{
            "name":"host1",
            "ip":"192.168.0.1"
          }]
        }
      ]
    }
  ]
}`
	ipam = initIpam(t, conf)

	// 1. Black out something random
	err = ipam.BlackOut("10.100.100.100/24")
	if err == nil {
		t.Fatal("TestChunkBlackout: Expected error that no network found")
	}

	// 2. Black out 10.0.0.0/30 - should be an error
	err = ipam.BlackOut("10.0.0.0/30")
	if err == nil {
		t.Fatal("TestChunkBlackout: Expected error because cannot contain entire network")
	}
	t.Logf("TestChunkBlackout: Received expected error: %s", err)

	// 3. Black out 10.0.0.0/32
	err = ipam.BlackOut("10.0.0.0/32")
	if err != nil {
		t.Fatal(err)
	}

	// 4. Black out 10.0.0.0/31 -- it should silently succeed but,
	// will replace /32
	err = ipam.BlackOut("10.0.0.0/31")
	if err != nil {
		t.Fatal(err)
	}

	// 4. Allocate IP - should start with 10.0.0.2
	ip, err := ipam.AllocateIP("1", "host1", "ten1", "seg1")
	t.Logf("TestChunkBlackout: 1. Allocated %s for ten1:seg1", ip)
	if err != nil {
		t.Fatal(err)
	}
	if ip.String() != "10.0.0.2" {
		t.Fatalf("Expected 10.0.0.2, got %s", ip)
	}

	ip, err = ipam.AllocateIP("2", "host1", "ten1", "seg1")
	t.Logf("TestChunkBlackout: 2. Allocated %s for ten1:seg1", ip)
	if err != nil {
		t.Fatal(err)
	}
	if ip.String() != "10.0.0.3" {
		t.Fatalf("Expected 10.0.0.3, got %s", ip)
	}

	// Now this should fail.
	ip, err = ipam.AllocateIP("3", "host1", "ten1", "seg1")
	if err == nil {
		t.Fatalf("Expected an error, received an IP: %s", ip)
	}

	if err.Error() != msgNoAvailableIP {
		t.Fatalf("Expected error \"%s\", got %s", msgNoAvailableIP, err)
	}

	// 6. Try to black out already allocated chunk, should get error.
	err = ipam.BlackOut("10.0.0.2/31")
	if err == nil {
		t.Fatalf("Expected error because trying to black out allocated IPs")
	} else {
		t.Logf("Received expected error: %s", err)
	}
	// 7. Remove blackout
	err = ipam.UnBlackOut("10.0.0.0/30")
	if err == nil {
		t.Fatalf("Expected error as no such CIDR to remove from blackout, got nothing")
	}
	t.Logf("Received expected error %s", err)

	err = ipam.UnBlackOut("10.0.0.0/31")
	if err != nil {
		t.Fatal(err)
	}
	// 8. Try allocating IPs again, will get them from the previously blacked out range.
	ip, err = ipam.AllocateIP("4", "host1", "ten1", "seg1")
	t.Logf("TestChunkBlackout: 4. Allocated %s for ten1:seg1", ip)
	if err != nil {
		t.Fatal(err)
	}
	if ip.String() != "10.0.0.0" {
		t.Fatalf("Expected 10.0.0.0, got %s", ip)
	}
	ip, err = ipam.AllocateIP("5", "host1", "ten1", "seg1")
	t.Logf("TestChunkBlackout: 5. Allocated %s for ten1:seg1", ip)
	if err != nil {
		t.Fatal(err)
	}
	if ip.String() != "10.0.0.1" {
		t.Fatalf("Expected 10.0.0.1, got %s", ip)
	}

	// 9. Now this should fail -- network is full
	t.Logf("Next allocation should fail - network is full.")
	ip, err = ipam.AllocateIP("6", "host1", "ten1", "seg1")
	if err == nil {
		t.Fatalf("Expected an error, received an IP: %s", ip)
	}

	if err.Error() != msgNoAvailableIP {
		t.Fatalf("Expected error \"%s\", got %s", msgNoAvailableIP, err)
	}
	t.Logf("TestChunkBlackout done.")
}

// TestIPReuse tests that an IP can be reused.
func TestIPReuse(t *testing.T) {
	conf := `{
  "networks":[
    {
      "name":"net1",
      "cidr":"10.0.0.0/31",
      "block_mask":31
    }
  ],
  "topologies":[
    {
      "networks":[
        "net1"
      ],
      "map":[
        {
          "routing":"foo",
          "groups":[{
            "name":"host1",
            "ip":"192.168.0.1"
          }]
        }
      ]
    }
  ]
}`
	ipam = initIpam(t, conf)

	ip, err := ipam.AllocateIP("1", "host1", "ten1", "seg1")
	t.Logf("TestChunkIPReuse: Allocated %s for ten1:seg1", ip)
	if err != nil {
		t.Fatal(err)
	}
	if ip.String() != "10.0.0.0" {
		t.Fatalf("Expected 10.0.0.0, got %s", ip)
	}

	ip, err = ipam.AllocateIP("2", "host1", "ten1", "seg1")
	t.Logf("TestChunkIPReuse: Allocated %s for ten1:seg1", ip)
	if err != nil {
		t.Fatal(err)
	}
	if ip.String() != "10.0.0.1" {
		t.Fatalf("Expected 10.0.0.1, got %s", ip)
	}

	// Now this should fail.
	ip, err = ipam.AllocateIP("3", "host1", "ten1", "seg1")
	if err == nil {
		t.Fatalf("Expected an error, received an IP: %s", ip)
	}

	if err.Error() != msgNoAvailableIP {
		t.Fatalf("Expected error \"%s\", got %s", msgNoAvailableIP, err)
	}

	// Deallocate first IP
	err = ipam.DeallocateIP("1")
	if err != nil {
		t.Fatal(err)
	}

	// This should succeed
	ip, err = ipam.AllocateIP("4", "host1", "ten1", "seg1")
	t.Logf("TestChunkIPReuse: Allocated %s for ten1:seg1", ip)
	if err != nil {
		t.Fatal(err)
	}
	if ip.String() != "10.0.0.0" {
		t.Fatalf("Expected 10.0.0.0, got %s", ip)
	}
}

// TestBlockReuse tests that a block can be reused.
func TestBlockReuse(t *testing.T) {
	conf := `{
  "networks":[
    {
      "name":"net1",
      "cidr":"10.0.0.0/31",
      "block_mask":32
    }
  ],
  "topologies":[
    {
      "networks":[
        "net1"
      ],
      "map":[
        {
          "routing":"foo",
          "groups":[{
            "name":"host1",
            "ip":"192.168.0.1"
          }]
        }
      ]
    }
  ]
}`
	ipam = initIpam(t, conf)

	ip, err := ipam.AllocateIP("1", "host1", "ten1", "seg1")
	t.Logf("TestChunkBlockReuse: Allocated %s for ten1:seg1", ip)
	if err != nil {
		t.Fatal(err)
	}
	if ip.String() != "10.0.0.0" {
		t.Fatalf("Expected 10.0.0.0, got %s", ip)
	}

	ip, err = ipam.AllocateIP("2", "host1", "ten1", "seg1")
	t.Logf("TestChunkBlockReuse: Allocated %s for ten1:seg1", ip)
	if err != nil {
		t.Fatal(err)
	}
	if ip.String() != "10.0.0.1" {
		t.Fatalf("Expected 10.0.0.1, got %s", ip)
	}

	// Now this should fail.
	ip, err = ipam.AllocateIP("3", "host1", "ten1", "seg1")
	t.Logf("TestChunkBlockReuse: Allocated %s for ten1:seg1", ip)
	if err == nil {
		t.Fatalf("Expected an error, received an IP: %s", ip)
	}

	if err.Error() != msgNoAvailableIP {
		t.Fatalf("Expected error \"%s\", got %s", msgNoAvailableIP, err)
	}

	// Deallocate first IP
	err = ipam.DeallocateIP("1")
	if err != nil {
		t.Fatal(err)
	}

	// This should succeed
	ip, err = ipam.AllocateIP("4", "host1", "ten1", "seg1")
	t.Logf("TestChunkBlockReuse: Allocated %s for ten1:seg1", ip)
	if err != nil {
		t.Fatal(err)
	}
	if ip.String() != "10.0.0.0" {
		t.Fatalf("Expected 10.0.0.0, got %s", ip)
	}
}

// Test32 tests bitmask size 32 - as a corner case.
func Test32(t *testing.T) {
	// Part 1. Simple /32 block size test
	conf := `{
  "networks":[
    {
      "name":"net1",
      "cidr":"10.0.0.0/24",
      "block_mask":32
    }
  ],
  "topologies":[
    {
      "networks":[
        "net1"
      ],
      "map":[
        {
          "routing":"foo",
          "groups":[{
            "name":"host1",
            "ip":"192.168.0.1"
          }]
        }
      ]
    }
  ]
}`
	ipam = initIpam(t, conf)

	ip, err := ipam.AllocateIP("1", "host1", "ten1", "seg1")
	t.Logf("TestChunkSegments: Allocated %s for ten1:seg1", ip)
	if err != nil {
		t.Fatal(err)
	}
	if ip.String() != "10.0.0.0" {
		t.Fatalf("Expected 10.0.0.0, got %s", ip)
	}

	ip, err = ipam.AllocateIP("2", "host1", "ten1", "seg1")
	t.Logf("TestChunkSegments: Allocated %s for ten1:seg1", ip)
	if err != nil {
		t.Fatal(err)
	}
	if ip.String() != "10.0.0.1" {
		t.Fatalf("Expected 10.0.0.1, got %s", ip)
	}

	// Part 2. Here we add a /32 block size to a /32 CIDR.
	conf = `{
  "networks":[
    {
      "name":"net1",
      "cidr":"10.0.0.0/32",
      "block_mask":32
    }
  ],
  "topologies":[
    {
      "networks":[
        "net1"
      ],
      "map":[
        {
          "routing":"foo",
          "groups": [{
            "name":"host1",
            "ip":"192.168.0.1"
          }]
        }
      ]
    }
  ]
}`
	ipam = initIpam(t, conf)

	ip, err = ipam.AllocateIP("2", "host1", "ten1", "seg1")
	t.Logf("TestChunkSegments: Allocated %s for ten1:seg1", ip)
	if err != nil {
		t.Fatal(err)
	}
	if ip.String() != "10.0.0.0" {
		t.Fatalf("Expected 10.0.0.0, got %s", ip)
	}

	// Now this should fail - only one /32 block can be there on a /32 net.
	ip, err = ipam.AllocateIP("3", "host1", "ten1", "seg1")
	t.Logf("TestChunkSegments: Allocated %s for ten1:seg1", ip)
	if err == nil {
		t.Fatalf("Expected an error, received an IP: %s", ip)
	}

	if err.Error() != msgNoAvailableIP {
		t.Fatalf("Expected error \"%s\", got %s", msgNoAvailableIP, err)
	}
}

// TestSegments tests that segments get different blocks.
func TestSegments(t *testing.T) {

	conf := `{
  "networks":[
    {
      "name":"net1",
      "cidr":"10.0.0.0/24",
      "block_mask":30
    }
  ],
  "topologies":[
    {
      "networks":[
        "net1"
      ],
      "map":[
        {
          "routing":"foo",
          "groups": [{
            "name":"host1",
            "ip":"192.168.0.1"
          }]
        }
      ]
    }
  ]
}`
	ipam = initIpam(t, conf)

	ip, err := ipam.AllocateIP("x1", "host1", "ten1", "seg1")
	t.Logf("TestChunkSegments: Allocated %s for ten1:seg1", ip)
	if err != nil {
		t.Fatal(err)
	}
	if ip.String() != "10.0.0.0" {
		t.Fatalf("Expected 10.0.0.0, got %s", ip)
	}

	ip, err = ipam.AllocateIP("x2", "host1", "ten1", "seg1")
	t.Logf("TestSegments: Allocated %s for ten1:seg1", ip)
	if err != nil {
		t.Fatal(err)
	}
	if ip.String() != "10.0.0.1" {
		t.Fatalf("Expected 10.0.0.0, got %s", ip)
	}

	// This should go into a separate chunk
	ip, err = ipam.AllocateIP("x3", "host1", "ten1", "seg2")
	t.Logf("TestChunkSegments: Allocated %s for ten1:seg2", ip)
	if err != nil {
		t.Fatal(err)
	}
	if ip.String() != "10.0.0.4" {
		t.Fatalf("Expected 10.0.0.4, got %s", ip)
	}
}

// TestTenants tests that addresses are allocated from networks
// on which provided tenants are allowed.
func TestTenants(t *testing.T) {
	conf := `{
  "networks":[
    {
      "name":"net1",
      "cidr":"10.200.0.0/16",
      "block_mask":29,
      "tenants":[
        "tenant1",
        "tenant2"
      ]
    },
    {
      "name":"net2",
      "cidr":"10.220.0.0/16",
      "block_mask":28,
      "tenants":[
        "tenant3"
      ]
    },
    {
      "name":"net3",
      "cidr":"10.240.0.0/16",
      "block_mask":28
    }
  ],
  "topologies":[
    {
      "networks":[
        "net1",
        "net2",
        "net3"
      ],
      "map":[
        {
          "routing":"foo",
          "groups": [{
            "name":"host1",
            "ip":"192.168.0.1"
          }]
        }
      ]
    }
  ]
}`
	ipam = initIpam(t, conf)

	ip, err := ipam.AllocateIP("x1", "host1", "tenant1", "")
	if err != nil {
		t.Fatal(err)
	}
	if ip.String() != "10.200.0.0" {
		t.Fatalf("Expected 10.200.0.0, got %s", ip.String())
	}

	ip, err = ipam.AllocateIP("x2", "host1", "tenant2", "")
	if err != nil {
		t.Fatal(err)
	}
	if ip.String() != "10.200.0.8" {
		t.Fatalf("Expected 10.200.0.8, got %s", ip.String())
	}

	ip, err = ipam.AllocateIP("x3", "host1", "tenant3", "")
	if err != nil {
		t.Fatal(err)
	}
	if ip.String() != "10.220.0.0" {
		t.Fatalf("Expected 10.220.0.0, got %s", ip.String())
	}

	// This one should get allocate from net3 - wildcard network
	ip, err = ipam.AllocateIP("x4", "host1", "someothertenant", "")
	if err != nil {
		t.Fatal(err)
	}
	if ip.String() != "10.240.0.0" {
		t.Fatalf("Expected 10.240.0.0, got %s", ip.String())
	}

	// TODO allocate no host
	ip, err = ipam.AllocateIP("x5", "no.such.host", "someothertenant", "")
	if err == nil {
		t.Fatalf("Expected an error")
	}
	if ip != nil {
		t.Fatalf("Expected a nil ip, got %v", ip)
	}
	t.Logf("Got %s", err)
}

func TestHostAllocation(t *testing.T) {
	conf := `{
  "networks":[
    {
      "name":"net1",
      "cidr":"10.0.0.0/8",
      "block_mask":30
    }
  ],
  "topologies":[
    {
      "networks":[
        "net1"
      ],
      "map":[
        {
          "routing":"test",
          "groups":[
            {
              "name":"ip-192-168-99-10",
              "ip":"192.168.99.10"
            },
            {
              "name":"ip-192-168-99-11",
              "ip":"192.168.99.11"
            }
          ]
        }
      ]
    }
  ]
}`
	ipam = initIpam(t, conf)

	ip, err := ipam.AllocateIP("x1", "ip-192-168-99-10", "tenant1", "")
	if err != nil {
		t.Fatal(err)
	}
	if ip.String() != "10.0.0.0" {
		t.Fatalf("Expected 10.0.0.0, got %s", ip.String())
	}

	ip, err = ipam.AllocateIP("x2", "ip-192-168-99-11", "tenant1", "")
	if err != nil {
		t.Fatal(err)
	}
	if ip.String() != "10.0.0.4" {
		t.Fatalf("Expected 10.0.0.4, got %s", ip.String())
	}
	t.Logf("Saved state: %s", testSaver.lastJson)
}

func TestTmp(t *testing.T) {
	var conf string
	// Slide 15: Example 2: Prefix per host
	t.Logf("Example 2: Prefix per host (a)")
	conf = `{
  "networks":[
    {
      "name":"vlanA",
      "cidr":"10.1.0.0/16",
      "block_mask": 30
    }
  ],
  "topologies":[
    {
      "networks":[
        "vlanA"
      ],
      "map":[
        {
          "routing":"prefix-on-host",
          "groups":[
            { "name" : "h1", "ip" : "1.1.1.1" }
          ]
        },
        {
          "routing":"prefix-on-host",
          "groups":[
            { "name" : "h1", "ip" : "1.1.1.1" }
          ]
        },
        {
          "routing":"prefix-on-host",
          "groups":[ { "name" : "h1", "ip" : "1.1.1.1" } ]
        },
        {
          "routing":"prefix-on-host",
          "groups":[ { "name" : "h1", "ip" : "1.1.1.1" } ]
        }
      ]
    }
  ]
}`
	initIpam(t, conf)
	t.Logf("Slide 15: Example 2: Prefix per host JSON:\n%s\n", testSaver.lastJson)

}

func TestVPCExample(t *testing.T) {
	var conf string
	t.Logf("TestVPCExample")
	conf = `{
    "networks" : [ "romana-us-west-2a" ],
    "map" :  {
        "groups"  : [
            { "routing" : "xyz", "groups" : [] },
            { "routing" : "xyz", "groups" : [] },
        ]
    }
},
{
    "networks" : [ "romana-us-west-2b" ],
    "map" :  {
        "assignment" : { "key1 : "value1" },
        "groups"  : [
            {  "assignment" : { "key2 : "value2" },  
               "routing" : "xyz", 
               "groups" : [] },
            { "routing" : "xyz", "groups" : [] },
        ]
    }
},`
	ipam := initIpam(t, conf)
	host1 := api.Host{Name: "host1",
		IP: net.ParseIP("10.10.10.10"),
	}
	err := ipam.AddHost(host1)
	if err != nil {
		t.Fatal(err)
	}

}

func TestJsonParsing(t *testing.T) {
	var conf string

	// Slide 12: Example 1: Simple, flat network
	t.Log("Example 1: Simple, flat network, (a)")
	conf = `{
  "networks":[
    {
      "name":"vlanA",
      "cidr":"10.1.0.0/16",
      "block_mask": 28
    }
  ],
  "topologies":[
    {
      "networks":[
        "vlanA"
      ],
      "map":[
        {
          "routing":"block-on-host",
          "groups":[
            { "name" : "h1", "ip" : "1.1.1.1" },
            { "name" : "h1", "ip" : "1.1.1.1" },
            { "name" : "h1", "ip" : "1.1.1.1" },
            { "name" : "h1", "ip" : "1.1.1.1" }
          ]
        }
      ]
    }
  ]
}`
	initIpam(t, conf)
	t.Logf("Slide 12: Example 1: Simple, flat network JSON:\n%s\n", testSaver.lastJson)

	// Slide 13: Example 1: Simple, flat network
	t.Logf("Example 1: Simple, flat network, (b)")
	conf = `{
  "networks":[
    {
      "name":"vlanA",
      "cidr":"10.1.0.0/16",
      "block_mask": 28
   
    }
  ],
  "topologies":[
    {
      "networks":[
        "vlanA"
      ],
      "map":[
        {
          "routing":"block-announce-bgp:peerxxxxx",
          "groups":[
            { "name" : "h1", "ip" : "1.1.1.1" },
            { "name" : "h2", "ip" : "1.1.1.1" },
            { "name" : "h3", "ip" : "1.1.1.1" },
            { "name" : "h4", "ip" : "1.1.1.1" }
          ]
        }
      ]
    }
  ]
}`
	initIpam(t, conf)
	t.Logf("Slide 13: Example 1: Simple, flat network:\n%s\n", testSaver.lastJson)

	// Slide 14: Example 1: Simple, flat network
	t.Logf("Example 1: Simple, flat network, (c)")
	conf = `{
  "networks":[
    {
      "name":"vlanA",
      "cidr":"10.1.0.0/16",
      "block_mask" : 28
    }
  ],
  "topologies":[
    {
      "networks":[
        "vlanA"
      ],
      "map":[
        {
          "routing":"block-on-host,block - announce - bgp: peerxxxxx",
          "groups":[
            { "name" : "h1", "ip" : "1.1.1.1" },
            { "name" : "h1", "ip" : "1.1.1.1" },
            { "name" : "h1", "ip" : "1.1.1.1" },
            { "name" : "h1", "ip" : "1.1.1.1" }
          ]
        }
      ]
    }
  ]
}`
	initIpam(t, conf)
	t.Logf("Slide 14: Example 1: Simple, flat network JSON:\n%s\n", testSaver.lastJson)

	// Slide 15: Example 2: Prefix per host
	t.Logf("Example 2: Prefix per host (a)")
	conf = `{
  "networks":[
    {
      "name":"vlanA",
      "cidr":"10.1.0.0/16",
      "block_mask" : 28
    }
  ],
  "topologies":[
    {
      "networks":[
        "vlanA"
      ],
      "map":[
        {
          "routing":"prefix-on-host",
          "groups":[
            { "name" : "h1", "ip" : "1.1.1.1" }
          ]
        },
        {
          "routing":"prefix-on-host",
          "groups":[
            { "name" : "h1", "ip" : "1.1.1.1" }
          ]
        },
        {
          "routing":"prefix-on-host",
          "groups":[ { "name" : "h1", "ip" : "1.1.1.1" } ]
        },
        {
          "routing":"prefix-on-host",
          "groups":[ { "name" : "h1", "ip" : "1.1.1.1" } ]
        }
      ]
    }
  ]
}`
	initIpam(t, conf)
	t.Logf("Slide 15: Example 2: Prefix per host JSON:\n%s\n", testSaver.lastJson)

	// Slide 16: Example 2: Prefix per host
	t.Logf("Example 2: Prefix per host (b)")
	conf = `{
  "networks":[
    {
      "name":"vlanA",
      "cidr":"10.1.0.0/16",
      "block_mask" : 28
    }
  ],
  "topologies":[
    {
      "networks":[
        "vlanA"
      ],
      "map":[
            {
              "routing":"prefix-announce-bgp:peerxxxx",
              "groups":[ { "name" : "h1", "ip" : "1.1.1.1" } ]
            },
            {
              "routing":"prefix-announce-bgp:peerxxxx",
              "groups":[ { "name" : "h1", "ip" : "1.1.1.1" } ]
            },
            {
              "routing":"prefix-announce-bgp:peerxxxx",
              "groups":[ { "name" : "h1", "ip" : "1.1.1.1" } ]
            },
            {
              "routing":"prefix-announce-bgp:peerxxxx",
              "groups":[ { "name" : "h1", "ip" : "1.1.1.1" }  ]
            }
      ]
    }
  ]
}`
	initIpam(t, conf)
	t.Logf("Slide 16: Example 2: Prefix per host JSON:\n%s\n", testSaver.lastJson)

	// Slide 17: Example 3: Multi-host groups + prefix
	t.Logf("Example 3: Multi-host groups + prefix")
	conf = `{
  "networks":[
    {
      "name":"vlanA",
      "cidr":"10.1.0.0/16",
      "block_mask" : 28
    }
  ],
  "topologies":[
    {
      "networks":[
        "vlanA"
      ],
      "map":[
        {
          "routing":"block-host-routes, prefix-announce-bgp:peerxxxx",
          "groups":[ { "name" : "h1", "ip" : "1.1.1.1" }, { "name" : "h1", "ip" : "1.1.1.1" } ]
        },
        {
          "routing":"block-host-routes, prefix-announce-bgp:peerxxxx",
          "groups":[ { "name" : "h1", "ip" : "1.1.1.1" }, { "name" : "h1", "ip" : "1.1.1.1" } ]
        },
        {
          "routing":"block-host-routes, prefix-announce-bgp:peerxxxx",
          "groups":[ { "name" : "h1", "ip" : "1.1.1.1" }, { "name" : "h1", "ip" : "1.1.1.1" } ]
        },
        {
          "routing":"block-host-routes, prefix-announce-bgp:peerxxxx",
          "groups":[ { "name" : "h1", "ip" : "1.1.1.1" }, { "name" : "h1", "ip" : "1.1.1.1" } ]
        }
      ]
    }
  ]
}`
	initIpam(t, conf)
	t.Logf("Slide 17: Example 3: Multi-host groups + prefix JSON:\n%s\n", testSaver.lastJson)

	// Slide 18: Example 4: VPC routing for two AZs
	t.Logf("Example 4: VPC routing for two AZs")
	conf = `{
  "networks":[
    {
      "name":"subnetA",
      "cidr":"10.1.0.0/16",
      "block_mask" : 28
    },
    {
      "name":"subnetB",
      "cidr":"10.2.0.0/16",
      "block_mask" : 28
    }
  ],
  "topologies":[
    {
      "networks":[
        "subnetA"
      ],
      "map":[
        {
          "routing":"block-host-routes,prefix-announce-vpc",
          "groups":[]
        },
        {
          "routing":"block-host-routes,prefix-announce-vpc",
          "groups":[]
        }
      ]
    },
    {
      "networks":[
        "subnetB"
      ],
      "map":[
        {
          "routing":"block-host-routes,prefix-announce-vpc",
          "groups":[]
        },
        {
          "routing":"block-host-routes,prefix-announce-vpc",
          "groups":[]
        }
      ]
    }
  ]
}`
	initIpam(t, conf)
	t.Logf("Slide 18: Example 4: VPC routing for two AZs JSON:\n%s\n", testSaver.lastJson)

}