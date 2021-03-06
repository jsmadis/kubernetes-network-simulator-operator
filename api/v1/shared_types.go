/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import v12 "k8s.io/api/networking/v1"

// ConnectionRule specifies Device, Network and ports used in the network policy
type ConnectionRule struct {
	// name of the device
	// +optional
	DeviceName string `json:"deviceName,omitempty"`

	// name of the network
	NetworkName string `json:"networkName"`

	// network policy ports
	// +optional
	NetworkPolicyPorts []v12.NetworkPolicyPort `json:"networkPolicyPorts"`
}
