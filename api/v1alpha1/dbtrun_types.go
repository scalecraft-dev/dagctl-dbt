package v1alpha1

import (
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DbtRunSpec struct {
	ProjectRef              corev1.LocalObjectReference `json:"projectRef"`
	Type                    RunType                     `json:"type,omitempty"`
	Commands                []string                    `json:"commands,omitempty"`
	TTLSecondsAfterFinished *int32                      `json:"ttlSecondsAfterFinished,omitempty"`
}

type RunType string

const (
	RunTypeScheduled RunType = "Scheduled"
	RunTypeManual    RunType = "Manual"
	RunTypeWebhook   RunType = "Webhook"
)

type DbtRunStatus struct {
	Phase          RunPhase                `json:"phase,omitempty"`
	StartTime      *metav1.Time            `json:"startTime,omitempty"`
	CompletionTime *metav1.Time            `json:"completionTime,omitempty"`
	JobRef         *corev1.ObjectReference `json:"jobRef,omitempty"`
	Conditions     []metav1.Condition      `json:"conditions,omitempty"`
	JobStatus      *batchv1.JobStatus      `json:"jobStatus,omitempty"`
	Logs           string                  `json:"logs,omitempty"`
	Artifacts      map[string]string       `json:"artifacts,omitempty"`
}

type RunPhase string

const (
	RunPhasePending   RunPhase = "Pending"
	RunPhaseRunning   RunPhase = "Running"
	RunPhaseSucceeded RunPhase = "Succeeded"
	RunPhaseFailed    RunPhase = "Failed"
	RunPhaseError     RunPhase = "Error"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=dbtrun
// +kubebuilder:printcolumn:name="Project",type="string",JSONPath=".spec.projectRef.name"
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.type"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Started",type="date",JSONPath=".status.startTime"
// +kubebuilder:printcolumn:name="Completed",type="date",JSONPath=".status.completionTime"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

type DbtRun struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DbtRunSpec   `json:"spec,omitempty"`
	Status DbtRunStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type DbtRunList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DbtRun `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DbtRun{}, &DbtRunList{})
}
