package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DbtProjectSpec struct {
	Git                        GitConfig                      `json:"git"`
	Schedule                   string                         `json:"schedule,omitempty"`
	Image                      string                         `json:"image,omitempty"`
	ProfilesConfigMap          string                         `json:"profilesConfigMap,omitempty"`
	ProfilesSecret             string                         `json:"profilesSecret,omitempty"`
	Commands                   []string                       `json:"commands,omitempty"`
	Env                        []corev1.EnvVar                `json:"env,omitempty"`
	Resources                  corev1.ResourceRequirements    `json:"resources,omitempty"`
	ServiceAccountName         string                         `json:"serviceAccountName,omitempty"`
	SuccessfulJobsHistoryLimit *int32                         `json:"successfulJobsHistoryLimit,omitempty"`
	FailedJobsHistoryLimit     *int32                         `json:"failedJobsHistoryLimit,omitempty"`
	Suspend                    bool                           `json:"suspend,omitempty"`
	VolumeClaimTemplates       []corev1.PersistentVolumeClaim `json:"volumeClaimTemplates,omitempty"`
	VolumeMounts               []corev1.VolumeMount           `json:"volumeMounts,omitempty"`
}

type GitConfig struct {
	Repository   string `json:"repository"`
	Ref          string `json:"ref,omitempty"`
	Path         string `json:"path,omitempty"`
	SSHKeySecret string `json:"sshKeySecret,omitempty"`
	AuthSecret   string `json:"authSecret,omitempty"`
}

type DbtProjectStatus struct {
	LastScheduledTime  *metav1.Time             `json:"lastScheduledTime,omitempty"`
	LastSuccessfulTime *metav1.Time             `json:"lastSuccessfulTime,omitempty"`
	ActiveRuns         []corev1.ObjectReference `json:"activeRuns,omitempty"`
	Phase              DbtProjectPhase          `json:"phase,omitempty"`
	Conditions         []metav1.Condition       `json:"conditions,omitempty"`
	ObservedGeneration int64                    `json:"observedGeneration,omitempty"`
}

type DbtProjectPhase string

const (
	DbtProjectPhaseReady     DbtProjectPhase = "Ready"
	DbtProjectPhaseSuspended DbtProjectPhase = "Suspended"
	DbtProjectPhaseError     DbtProjectPhase = "Error"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=dbt
// +kubebuilder:printcolumn:name="Schedule",type="string",JSONPath=".spec.schedule"
// +kubebuilder:printcolumn:name="Suspend",type="boolean",JSONPath=".spec.suspend"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Last Scheduled",type="date",JSONPath=".status.lastScheduledTime"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

type DbtProject struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DbtProjectSpec   `json:"spec,omitempty"`
	Status DbtProjectStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type DbtProjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DbtProject `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DbtProject{}, &DbtProjectList{})
}
