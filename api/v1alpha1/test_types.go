/*


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

package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestSpec is the definition for running a test.
type TestSpec struct {
	// Image is the container image for the test.
	Image string `json:"image"`

	// Entrypoint is the command to run in the image
	// to execute the test.
	Entrypoint []string `json:"entrypoint"`

	// Labels are key/value pairs that can be used to
	// group and select tests.
	Labels map[string]string `json:"labels,omitempty"`

	// BundleConfigMap is the name of a configmap containing
	// the contents of an operator bundle as a tgz file. It must
	// be in the same namespace as the Test.
	// +optional
	BundleConfigMap string `json:"bundleConfigMap,omitempty"`

	// ServiceAccount is the service account name to use to run
	// the test.
	// +optional
	ServiceAccount string `json:"serviceAccount,omitempty"`
}

// TestStatus defines the observed state of Test
type TestStatus struct {
	// Phase is the phase of the pod that is running the test.
	Phase v1.PodPhase `json:"phase,omitempty"`

	// Results is the results from running the test defined by Spec.
	Results []TestResult `json:"results,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Test defines a test specification. If the test has completed, Status
// contains the results from running the test.
type Test struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TestSpec   `json:"spec,omitempty"`
	Status TestStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TestList contains a list of Test
type TestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Test `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Test{}, &TestList{})
}

// State is a type used to indicate the result state of a Test.
type State string

const (
	// PassState occurs when a test passes.
	PassState State = "pass"
	// FailState occurs when a test fails.
	FailState State = "fail"
	// ErrorState occurs when a test encounters a fatal error.
	ErrorState State = "error"
)

// TestResult is the result of running a scorecard test.
type TestResult struct {
	// Name is the name of the test.
	Name string `json:"name"`

	// Description describes what the test does.
	Description string `json:"description,omitempty"`

	// State is the final state of the test.
	State State `json:"state"`

	// Errors is a list of the errors that occurred during the test (this can include both fatal and non-fatal errors).
	Errors []string `json:"errors,omitempty"`

	// Details holds any further details from the test run (if applicable).
	Details string `json:"details,omitempty"`
}
