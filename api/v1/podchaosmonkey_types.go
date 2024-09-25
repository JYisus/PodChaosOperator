/*
Copyright 2024.

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
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PodChaosMonkeySpec defines the desired state of PodChaosMonkey
type PodChaosMonkeySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// The namespace where PodChaosMonkey will delete pods from.
	TargetNamespace string `json:"targetNamespace,required"`
	// The format of the schedule. Formats supported are 'cron' and 'cron-seconds'.
	// +kubebuilder:validation:Enum=cron;cron-seconds
	ScheduleFormat string `json:"scheduleFormat,omitempty"`
	// The schedule to delete pods from the target namespace.
	Schedule  string     `json:"schedule,omitempty"`
	Blacklist *Blacklist `json:"blacklist,omitempty"`
	// The image of the PodChaosMonkey.
	// +kubebuilder:default="yisusisback/pod-chaos-monkey:latest"
	Image string `json:"image"`
	// Replicas of the PodChaosMonkey pod.
	// +kubebuilder:default=1
	Replicas int32 `json:"replicas"`
}

type Blacklist struct {
	Labels         map[string]string `json:"labels,omitempty"`
	FieldSelectors map[string]string `json:"fieldSelectors,omitempty"`
}

func (b *Blacklist) AsString() string {
	str, _ := json.Marshal(b)
	return string(str)
}

// PodChaosMonkeyStatus defines the observed state of PodChaosMonkey
type PodChaosMonkeyStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// PodChaosMonkey is the Schema for the podchaosmonkeys API
type PodChaosMonkey struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PodChaosMonkeySpec   `json:"spec,omitempty"`
	Status PodChaosMonkeyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PodChaosMonkeyList contains a list of PodChaosMonkey
type PodChaosMonkeyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PodChaosMonkey `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PodChaosMonkey{}, &PodChaosMonkeyList{})
}
