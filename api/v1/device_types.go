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

import (
	v1 "k8s.io/api/core/v1"
	v12 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DevicePorts specifies Device, network and ports used in the network policy
type DevicePorts struct {
	// name of the device
	// +optional
	DeviceName string `json:"deviceName,omitempty"`

	// name of the network
	NetworkName string `json:"networkName"`

	// network policy pods
	NetworkPolicyPorts []v12.NetworkPolicyPort `json:"networkPolicyPorts"`
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DeviceSpec defines the desired state of Device
type DeviceSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Name of the network where to deploy device
	NetworkName string `json:"networkName"`

	// Specifies the pod that contains container of wanted device
	PodSpec v1.PodSpec `json:"podSpec"`

	// Specifies a service of the device
	ServiceSpec v1.ServiceSpec `json:"serviceSpec"`

	// Device ingress ports, specifies devices from which can this device receive connection
	// +optional
	DeviceIngressPorts []DevicePorts `json:"deviceIngressPorts"`

	// Device egress ports, specifies devices to which can this device create connection
	// +optional
	DeviceEgressPorts []DevicePorts `json:"deviceEgressPorts"`

	// EnableInternet enables internet connection for the device.
	// It enables egress connection to everything, in the cluster. However, you wont be able to access another device
	// because another device needs ingress enabled to be able to receive connection.
	// + optional
	EnableInternet bool `json:"enableInternet"`
}

// DeviceStatus defines the observed state of Device
type DeviceStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Name of the network where device is deployed
	NetworkName string `json:"networkName"`

	// Name of the pod in the network
	PodName string `json:"podName"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster

// Device is the Schema for the devices API
type Device struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DeviceSpec   `json:"spec,omitempty"`
	Status DeviceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DeviceList contains a list of Device
type DeviceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Device `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Device{}, &DeviceList{})
}

func (in *Device) PodName() string {
	return in.Name + "-pod"
}

func (in Device) NetworkName() string {
	return in.Name + "-network-policy"
}

func (in Device) ServiceName() string {
	return in.Name + "-service"
}
