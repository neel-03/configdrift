package target

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/neel-03/configdrift/internal/config"
)

func TestK8sAdapter_Fetch(t *testing.T) {
	namespace := "test-ns"
	cmName := "test-cm"
	cmKey := "config.yaml"
	content := "foo: bar"

	tests := []struct {
		name          string
		cfg           config.TargetConfig
		existingCM    *corev1.ConfigMap
		expectedData  []byte
		expectedError string
	}{
		{
			name: "Success",
			cfg: config.TargetConfig{
				Namespace: namespace,
				ConfigMap: cmName,
				CMKey:     cmKey,
			},
			existingCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      cmName,
					Namespace: namespace,
				},
				Data: map[string]string{
					cmKey: content,
				},
			},
			expectedData: []byte(content),
		},
		{
			name: "ConfigMap NotFound",
			cfg: config.TargetConfig{
				Namespace: namespace,
				ConfigMap: "non-existent",
				CMKey:     cmKey,
			},
			expectedError: `configmaps "non-existent" not found`,
		},
		{
			name: "Key NotFound",
			cfg: config.TargetConfig{
				Namespace: namespace,
				ConfigMap: cmName,
				CMKey:     "wrong-key",
			},
			existingCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      cmName,
					Namespace: namespace,
				},
				Data: map[string]string{
					cmKey: content,
				},
			},
			expectedError: `key "wrong-key" not found in ConfigMap "test-cm"`,
		},
		{
			name: "ConfigMap No Data",
			cfg: config.TargetConfig{
				Namespace: namespace,
				ConfigMap: cmName,
				CMKey:     cmKey,
			},
			existingCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      cmName,
					Namespace: namespace,
				},
				Data: nil,
			},
			expectedError: `ConfigMap "test-cm" in namespace "test-ns" has no data`,
		},
		{
			name: "Invalid Timeout Ignored",
			cfg: config.TargetConfig{
				Namespace: namespace,
				ConfigMap: cmName,
				CMKey:     cmKey,
				Timeout:   "invalid",
			},
			existingCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      cmName,
					Namespace: namespace,
				},
				Data: map[string]string{
					cmKey: content,
				},
			},
			expectedData: []byte(content),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewSimpleClientset()
			if tt.existingCM != nil {
				_, err := client.CoreV1().ConfigMaps(tt.existingCM.Namespace).Create(context.TODO(), tt.existingCM, metav1.CreateOptions{})
				require.NoError(t, err)
			}

			adapter := &K8sAdapter{
				cfg:    tt.cfg,
				client: client,
			}

			data, err := adapter.Fetch(context.Background())

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, data)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedData, data)
			}
		})
	}
}

func TestK8sAdapter_Name(t *testing.T) {
	cfg := config.TargetConfig{Name: "my-k8s"}
	adapter := &K8sAdapter{cfg: cfg}
	assert.Equal(t, "my-k8s", adapter.Name())
}

func TestNewK8sAdapter_Error(t *testing.T) {
	cfg := config.TargetConfig{
		KubeConfig: "/non/existent/path",
	}
	adapter, err := NewK8sAdapter(cfg)
	assert.Error(t, err)
	assert.Nil(t, adapter)
}
