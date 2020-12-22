// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client

// ContainerPort
type ContainerPort struct {
	Name          string `json:"name,omitempty"`
	HostPort      int    `json:"hostPort,omitempty"`
	ContainerPort int    `json:"containerPort"`
	Protocol      string `json:"protocol,omitempty"`
}

// EnvVar is environment variable
type EnvVar struct {
	Name      string        `json:"name"`
	Value     string        `json:"value,omitempty"`
	ValueFrom *EnvVarSource `json:"valueFrom,omitempty"`
}

// EnvVarSource represents a source for the value of an EnvVar.
type EnvVarSource struct {
	SecretKeyRef *SecretKeySelector `json:"secretKeyRef,omitempty"`
}

// SecretKeySelector selects a key of a Secret.
type SecretKeySelector struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	Optional bool   `json:"optional,omitempty"`
}

type Condition struct {
	Started string `json:"startedAt,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// Container defined container runtime values
type Container struct {
	Name           string                `json:"name"`
	Image          string                `json:"image"`
	Env            []EnvVar              `json:"env,omitempty"`
	Command        []string              `json:"command,omitempty"`
	Args           []string              `json:"args,omitempty"`
	Ports          []ContainerPort       `json:"ports,omitempty"`
	ReadinessProbe *Probe                `json:"readinessProbe,omitempty"`
	Resources      *ResourceRequirements `json:"resources,omitempty"`
	VolumeMounts   []VolumeMount         `json:"volumeMounts,omitempty"`
}

// DeploymentSpec defines vine deployment spec
type DeploymentSpec struct {
	Replicas int            `json:"replicas,omitempty"`
	Selector *LabelSelector `json:"selector"`
	Template *Template      `json:"template,omitempty"`
}

// DeploymentCondition describes the state of deployment
type DeploymentCondition struct {
	LastUpdateTime string `json:"lastUpdateTime"`
	Type           string `json:"type"`
	Reason         string `json:"reason,omitempty"`
	Message        string `json:"message,omitempty"`
}

// DeploymentStatus is returned when querying deployment
type DeploymentStatus struct {
	Replicas            int                   `json:"replicas,omitempty"`
	UpdatedReplicas     int                   `json:"updatedReplicas,omitempty"`
	ReadyReplicas       int                   `json:"readyReplicas,omitempty"`
	AvailableReplicas   int                   `json:"availableReplicas,omitempty"`
	UnavailableReplicas int                   `json:"unavailableReplicas,omitempty"`
	Conditions          []DeploymentCondition `json:"conditions,omitempty"`
}

// Deployment is Kubernetes deployment
type Deployment struct {
	Metadata *Metadata         `json:"metadata"`
	Spec     *DeploymentSpec   `json:"spec,omitempty"`
	Status   *DeploymentStatus `json:"status,omitempty"`
}

// DeploymentList
type DeploymentList struct {
	Items []Deployment `json:"items"`
}

// LabelSelector is a label query over a set of resources
// NOTE: we do not support MatchExpressions at the moment
type LabelSelector struct {
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
}

type LoadBalancerIngress struct {
	IP       string `json:"ip,omitempty"`
	Hostname string `json:"hostname,omitempty"`
}

type LoadBalancerStatus struct {
	Ingress []LoadBalancerIngress `json:"ingress,omitempty"`
}

// Metadata defines api object metadata
type Metadata struct {
	Name        string            `json:"name,omitempty"`
	Namespace   string            `json:"namespace,omitempty"`
	Version     string            `json:"version,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// PodSpec is a pod
type PodSpec struct {
	Containers         []Container `json:"containers"`
	RuntimeClassName   string      `json:"runtimeClassName"`
	ServiceAccountName string      `json:"serviceAccountName"`
	Volumes            []Volume    `json:"volumes"`
}

// PodList
type PodList struct {
	Items []Pod `json:"items"`
}

// Pod is the top level item for a pod
type Pod struct {
	Metadata *Metadata  `json:"metadata"`
	Spec     *PodSpec   `json:"spec,omitempty"`
	Status   *PodStatus `json:"status"`
}

// PodStatus
type PodStatus struct {
	Conditions []PodCondition    `json:"conditions,omitempty"`
	Containers []ContainerStatus `json:"containerStatuses"`
	PodIP      string            `json:"podIP"`
	Phase      string            `json:"phase"`
	Reason     string            `json:"reason"`
}

// PodCondition describes the state of pod
type PodCondition struct {
	Type    string `json:"type"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

type ContainerStatus struct {
	State ContainerState `json:"state"`
}

type ContainerState struct {
	Running    *Condition `json:"running"`
	Terminated *Condition `json:"terminated"`
	Waiting    *Condition `json:"waiting"`
}

// Resource is API resource
type Resource struct {
	Name  string
	Kind  string
	Value interface{}
}

// ServicePort configures service ports
type ServicePort struct {
	Name     string `json:"name,omitempty"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol,omitempty"`
}

// ServiceSpec provides service configuration
type ServiceSpec struct {
	ClusterIP string            `json:"clusterIP"`
	Type      string            `json:"type,omitempty"`
	Selector  map[string]string `json:"selector,omitempty"`
	Ports     []ServicePort     `json:"ports,omitempty"`
}

// ServiceStatus
type ServiceStatus struct {
	LoadBalancer LoadBalancerStatus `json:"loadBalancer,omitempty"`
}

// Service is kubernetes service
type Service struct {
	Metadata *Metadata      `json:"metadata"`
	Spec     *ServiceSpec   `json:"spec,omitempty"`
	Status   *ServiceStatus `json:"status,omitempty"`
}

// ServiceList
type ServiceList struct {
	Items []Service `json:"items"`
}

// Template is vine deployment template
type Template struct {
	Metadata *Metadata `json:"metadata,omitempty"`
	PodSpec  *PodSpec  `json:"spec,omitempty"`
}

// Namespace is a Kubernetes Namespace
type Namespace struct {
	Metadata *Metadata `json:"metadata,omitempty"`
}

// NamespaceList
type NamespaceList struct {
	Items []Namespace `json:"items"`
}

// ImagePullSecret
type ImagePullSecret struct {
	Name string `json:"name"`
}

// Secret
type Secret struct {
	Type     string            `json:"type,omitempty"`
	Data     map[string]string `json:"data"`
	Metadata *Metadata         `json:"metadata,omitempty"`
}

// ServiceAccount
type ServiceAccount struct {
	Metadata         *Metadata         `json:"metadata,omitempty"`
	ImagePullSecrets []ImagePullSecret `json:"imagePullSecrets,omitempty"`
}

// Probe describes a health check to be performed against a container to determine whether it is alive or ready to receive traffic.
type Probe struct {
	TCPSocket           *TCPSocketAction `json:"tcpSocket,omitempty"`
	PeriodSeconds       int              `json:"periodSeconds"`
	InitialDelaySeconds int              `json:"initialDelaySeconds"`
}

// TCPSocketAction describes an action based on opening a socket
type TCPSocketAction struct {
	Host string      `json:"host,omitempty"`
	Port interface{} `json:"port,omitempty"`
}

// ResourceRequirements describes the compute resource requirements.
type ResourceRequirements struct {
	Limits   *ResourceLimits `json:"limits,omitempty"`
	Requests *ResourceLimits `json:"requests,omitempty"`
}

// ResourceLimits describes the limits for a service
type ResourceLimits struct {
	Memory           string `json:"memory,omitempty"`
	CPU              string `json:"cpu,omitempty"`
	EphemeralStorage string `json:"ephemeral-storage,omitempty"`
}

// Volume describes a volume which can be mounted to a pod
type Volume struct {
	Name                  string                            `json:"name"`
	PersistentVolumeClaim PersistentVolumeClaimVolumeSource `json:"persistentVolumeClaim,omitempty"`
}

// PersistentVolumeClaimVolumeSource represents a reference to a PersistentVolumeClaim in the same namespace
type PersistentVolumeClaimVolumeSource struct {
	ClaimName string `json:"claimName"`
}

// VolumeMount describes a mounting of a Volume within a container.
type VolumeMount struct {
	Name      string `json:"name"`
	MountPath string `json:"mountPath"`
}

// NetworkPolicy defines label-based filtering for network ingress
type NetworkPolicy struct {
	Metadata *Metadata          `json:"metadata,omitempty"`
	Spec     *NetworkPolicySpec `json:"spec,omitempty"`
}

// NetworkPolicySpec is the spec for a NetworkPolicy
type NetworkPolicySpec struct {
	Ingress     []NetworkPolicyRule `json:"ingress,omitempty"`
	Egress      []NetworkPolicyRule `json:"egress,omitempty"`
	PodSelector *Selector           `json:"podSelector,omitempty"`
	PolicyTypes []string            `json:"policyTypes,omitempty"`
}

// NetworkPolicyRule defines egress or ingress
type NetworkPolicyRule struct {
	From []IngressRuleSelector `json:"from,omitempty"`
	To   []IngressRuleSelector `json:"to,omitempty"`
}

// IngressRuleSelector defines a namespace or pod selector for ingress
type IngressRuleSelector struct {
	NamespaceSelector *Selector `json:"namespaceSelector,omitempty"`
	PodSelector       *Selector `json:"podSelector,omitempty"`
}

// Selector
type Selector struct {
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
}

// ResourceQuota defines resource limits for a namespace
type ResourceQuota struct {
	Metadata *Metadata          `json:"metadata,omitempty"`
	Spec     *ResourceQuotaSpec `json:"spec,omitempty"`
}

// ResourceQuotaSpec
type ResourceQuotaSpec struct {
	Hard *ResourceQuotaSpecs `json:"hard,omitempty"`
}

// ResourceQuotaSpecs defines requests and limits
type ResourceQuotaSpecs struct {
	LimitsCPU                string `json:"limits.cpu,omitempty"`
	LimitsEphemeralStorage   string `json:"limits.ephemeral-storage,omitempty"`
	LimitsMemory             string `json:"limits.memory,omitempty"`
	RequestsCPU              string `json:"requests.cpu,omitempty"`
	RequestsEphemeralStorage string `json:"requests.ephemeral-storage,omitempty"`
	RequestsMemory           string `json:"requests.memory,omitempty"`
}
