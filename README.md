# Kubernetes Network Simulator Operator

The main goal of this project is to create a Kubernetes Operator which creates network simulation on top of Kubernetes.
The operator is built with [operator-sdk](https://sdk.operatorframework.io/).

The operator defines 2 CRDs that are used for the network simulation.
* **Network CRD** - represents a simulated network. It contains attributes that define the desired state of the simulated network.
* **Device CRD** - represents a device/application deployed inside the simulated network.

By default, the simulated networks are isolated, and devices inside them can not communicate with other devices from other networks.
The connection between networks/devices and devices/networks is defined within the attributes of the CRDs.

## Running operator

To simulate the network inside Kubernetes, the Kubernetes need to be running with the CNI plugin.
In this example, we are using the Calico plugin. Please read the quick start for [Calico on minikube](https://docs.projectcalico.org/getting-started/kubernetes/minikube).

Start minikube server with the calico plugin:
```shell
$ minikube start --network-plugin=cni --cni=calico
```

Start the operator:
```shell
$ make install && make run
```

It will take a couple of minutes to start all necessary services in the Kubernetes cluster (especially the calico objects).


You can check if everything is up and running with the following command:
```shell
$ kubectl get pods --all-namespaces
```



