package util

import (
	networksimulatorv1 "github.com/jsmadis/kubernetes-network-simulator-operator/api/v1"
	v12 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// processIngressNetworkPolicy
// returns array of network policy ingress rules that are created from device.Spec.DeviceIngressRules
func ProcessIngressNetworkPolicy(ports []networksimulatorv1.ConnectionRule) []v12.NetworkPolicyIngressRule {
	var ingress []v12.NetworkPolicyIngressRule
	for _, deviceIngressPort := range ports {
		var ingressRule = v12.NetworkPolicyIngressRule{
			Ports: deviceIngressPort.NetworkPolicyPorts,
			From:  processNetworkPolicyPeer(deviceIngressPort),
		}

		if deviceIngressPort.NetworkPolicyPorts != nil {
			ingressRule.Ports = deviceIngressPort.NetworkPolicyPorts
		}

		ingress = append(ingress, ingressRule)
	}
	return ingress
}

// processEgressNetworkPolicy
//returns array of network policy egress rules that are created from device.Spec.DeviceEgressRules
func ProcessEgressNetworkPolicy(ports []networksimulatorv1.ConnectionRule) []v12.NetworkPolicyEgressRule {
	var egress []v12.NetworkPolicyEgressRule
	for _, deviceEgressPort := range ports {
		var egressRule = v12.NetworkPolicyEgressRule{
			To: processNetworkPolicyPeer(deviceEgressPort),
		}

		if deviceEgressPort.NetworkPolicyPorts != nil {
			egressRule.Ports = deviceEgressPort.NetworkPolicyPorts
		}

		egress = append(egress, egressRule)
	}
	return egress
}

//processNetworkPolicyPeer creates array of network policy pear from device port struct
func processNetworkPolicyPeer(ports networksimulatorv1.ConnectionRule) []v12.NetworkPolicyPeer {
	var peers []v12.NetworkPolicyPeer

	// If DeviceName is empty select every pod --> device should see every pod from other network
	podSelectorLabelSelector := &metav1.LabelSelector{}
	if ports.DeviceName != "" {
		podSelectorLabelSelector.MatchLabels = map[string]string{"Patriot-Device": ports.DeviceName}
	}

	peers = append(peers, v12.NetworkPolicyPeer{
		PodSelector: podSelectorLabelSelector,
		NamespaceSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"Patriot-Network": ports.NetworkName},
		},
	})
	return peers
}
