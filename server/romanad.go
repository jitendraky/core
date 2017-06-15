// Copyright (c) 2016-2017 Pani Networks
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

package server

import (
	"github.com/romana/core/common"
	"github.com/romana/core/common/api"
)

type Romanad struct {
	addr   string
	client *Client
	routes common.Route
}

// Name provides name of this service.
func (r *Romanad) GetAddress() string {
	return r.addr
}

func (r *Romanad) Name() string {
	return "romanad"
}

func (r *Romanad) Initialize(clientConfig client.Config) error {
	var err error
	r.client, err = client.NewClient(clientConfig)
	if err != nil {
		return err
	}
	return nil
}

// Routes provided by ipam.
func (r *Romanad) Routes() common.Routes {
	routes := common.Routes{
		common.Route{
			Method:  "GET",
			Pattern: "/networks/{network}/blocks/",
			Handler: r.listBlocks,
		},
		common.Route{
			Method:      "POST",
			Pattern:     "/address",
			Handler:     r.allocateIP,
			MakeMessage: func() interface{} { return &api.IPAMAddressRequest{} },
		},
		common.Route{
			Method:  "DELETE",
			Pattern: "/address",
			Handler: r.deallocateIP,
		},
		common.Route{
			Method:  "GET",
			Pattern: "/networks",
			Handler: r.listNetworks,
		},

		common.Route{
			Method:      "POST",
			Pattern:     "/topology",
			Handler:     r.updateTopology,
			MakeMessage: func() interface{} { return &api.TopologyUpdateRequest{} },
		},
		common.Route{
			Method:  "GET",
			Pattern: "/hosts",
			Handler: r.listHosts,
		},
		common.Route{
			Method:          "POST",
			Pattern:         policiesPath,
			Handler:         r.addPolicy,
			MakeMessage:     func() interface{} { return &api.Policy{} },
			UseRequestToken: false,
		},
		common.Route{
			Method:          "DELETE",
			Pattern:         policiesPath,
			Handler:         r.deletePolicy,
			MakeMessage:     func() interface{} { return &api.Policy{} },
			UseRequestToken: false,
		},
		common.Route{
			Method:          "DELETE",
			Pattern:         policiesPath + "/{policyID}",
			Handler:         r.deletePolicy,
			MakeMessage:     func() interface{} { return &api.Policy{} },
			UseRequestToken: false,
		},
		common.Route{
			Method:          "GET",
			Pattern:         policiesPath,
			Handler:         r.listPolicies,
			MakeMessage:     nil,
			UseRequestToken: false,
		},
		common.Route{
			Method:          "GET",
			Pattern:         policiesPath + "/{policyID}",
			Handler:         r.getPolicy,
			MakeMessage:     nil,
			UseRequestToken: false,
		},
		common.Route{
			Method:  "GET",
			Pattern: findPath + policiesPath + "/{policyName}",
			Handler: r.findPolicyByName,
		},
	}
	return routes
}
