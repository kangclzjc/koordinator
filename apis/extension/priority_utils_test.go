/*
Copyright 2022 The Koordinator Authors.

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

package extension

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

func TestGetPriorityClass(t *testing.T) {
	testCases := []struct {
		pod      *v1.Pod
		expected PriorityClass
	}{
		{
			pod: &v1.Pod{
				Spec: v1.PodSpec{
					Priority: pointer.Int32((PriorityProdValueMin + PriorityProdValueMax) / 2),
				},
			},
			expected: PriorityProd,
		},
		{
			pod: &v1.Pod{
				Spec: v1.PodSpec{
					Priority: pointer.Int32((PriorityBatchValueMin + PriorityBatchValueMax) / 2),
				},
			},
			expected: PriorityBatch,
		},
		{
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						LabelPodPriorityClass: string(PriorityProd),
					},
				},
				Spec: v1.PodSpec{
					Priority: pointer.Int32((PriorityBatchValueMin + PriorityBatchValueMax) / 2),
				},
			},
			expected: PriorityProd,
		},
		{
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						LabelPodPriorityClass: "unknown",
					},
				},
				Spec: v1.PodSpec{
					Priority: pointer.Int32((PriorityBatchValueMin + PriorityBatchValueMax) / 2),
				},
			},
			expected: PriorityNone,
		},
	}

	for _, tc := range testCases {
		p := GetPodPriorityClassRaw(tc.pod)
		if p != tc.expected {
			t.Errorf("unexpected priority class, expected %v actual %v", tc.expected, p)
		}
	}
}

func TestGetPodPriorityClassWithDefault(t *testing.T) {
	testCases := []struct {
		pod      *v1.Pod
		expected PriorityClass
	}{
		{
			pod: &v1.Pod{
				Spec: v1.PodSpec{
					Priority: pointer.Int32((PriorityProdValueMin + PriorityProdValueMax) / 2),
				},
			},
			expected: PriorityProd,
		},
		{
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						LabelPodPriorityClass: string(PriorityProd),
					},
				},
				Spec: v1.PodSpec{
					Priority: pointer.Int32((PriorityBatchValueMin + PriorityBatchValueMax) / 2),
				},
			},
			expected: PriorityProd,
		},
		{
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						LabelPodQoS: string(QoSLSR),
					},
				},
				Spec: v1.PodSpec{
					Priority: pointer.Int32((PriorityBatchValueMin + PriorityBatchValueMax) / 2),
				},
			},
			expected: PriorityBatch,
		},
		{
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						LabelPodQoS: string(QoSLSR),
					},
				},
			},
			expected: PriorityProd,
		},
		{
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						LabelPodQoS: string(QoSBE),
					},
				},
			},
			expected: PriorityBatch,
		},
		{
			pod: &v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{
									v1.ResourceCPU: resource.MustParse("100"),
								},
								Limits: v1.ResourceList{
									v1.ResourceCPU: resource.MustParse("200"),
								},
							},
						},
					},
				},
			},
			expected: PriorityProd,
		},
		{
			pod: &v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{
									v1.ResourceCPU: resource.MustParse("100"),
								},
								Limits: v1.ResourceList{
									v1.ResourceCPU: resource.MustParse("100"),
								},
							},
						},
					},
				},
			},
			expected: PriorityProd,
		},
		{
			pod: &v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name: "abc",
						},
					},
				},
			},
			expected: PriorityBatch,
		},
	}

	for _, tc := range testCases {
		p := GetPodPriorityClassWithDefault(tc.pod)
		if p != tc.expected {
			t.Errorf("unexpected priority class, expected %v actual %v", tc.expected, p)
		}
	}
}

func TestGetPodPriorityValueWithDefault(t *testing.T) {
	defaultPriorityValue := PriorityNoneValueDefault
	tests := []struct {
		name string
		arg  *v1.Pod
		want *int32
	}{
		{
			name: "pod is nil",
			arg:  nil,
			want: &defaultPriorityValue,
		},
		{
			name: "pod has non-default priority",
			arg: &v1.Pod{
				Spec: v1.PodSpec{
					Priority: pointer.Int32(PriorityBatchValueMax),
				},
			},
			want: pointer.Int32(PriorityBatchValueMax),
		},
		{
			name: "pod has non-default priority",
			arg: &v1.Pod{
				Spec: v1.PodSpec{
					Priority: pointer.Int32(PriorityBatchValueMax),
				},
			},
			want: pointer.Int32(PriorityBatchValueMax),
		},
		{
			name: "get default priority pod by its class",
			arg: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						LabelPodQoS: string(QoSLSR),
					},
				},
			},
			want: pointer.Int32(PriorityProdValueDefault),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetPodPriorityValueWithDefault(tt.arg)
			assert.Equal(t, tt.want, got)
		})
	}
}
