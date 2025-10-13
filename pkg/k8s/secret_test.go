package k8s

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// Helper function to create a test logger
func newTestLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise in tests
	return logger
}

// TestEqualByteMaps tests the equalByteMaps function
func TestEqualByteMaps(t *testing.T) {
	tests := []struct {
		name     string
		a        map[string][]byte
		b        map[string][]byte
		expected bool
	}{
		{
			name:     "both empty",
			a:        map[string][]byte{},
			b:        map[string][]byte{},
			expected: true,
		},
		{
			name:     "both nil",
			a:        nil,
			b:        nil,
			expected: true,
		},
		{
			name:     "equal maps",
			a:        map[string][]byte{"key1": []byte("value1"), "key2": []byte("value2")},
			b:        map[string][]byte{"key1": []byte("value1"), "key2": []byte("value2")},
			expected: true,
		},
		{
			name:     "different lengths",
			a:        map[string][]byte{"key1": []byte("value1")},
			b:        map[string][]byte{"key1": []byte("value1"), "key2": []byte("value2")},
			expected: false,
		},
		{
			name:     "different values",
			a:        map[string][]byte{"key1": []byte("value1")},
			b:        map[string][]byte{"key1": []byte("value2")},
			expected: false,
		},
		{
			name:     "missing key in b",
			a:        map[string][]byte{"key1": []byte("value1"), "key2": []byte("value2")},
			b:        map[string][]byte{"key1": []byte("value1")},
			expected: false,
		},
		{
			name:     "one empty one nil",
			a:        map[string][]byte{},
			b:        nil,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := equalByteMaps(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("equalByteMaps() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestEqualStringMaps tests the equalStringMaps function
func TestEqualStringMaps(t *testing.T) {
	tests := []struct {
		name     string
		a        map[string]string
		b        map[string]string
		expected bool
	}{
		{
			name:     "both empty",
			a:        map[string]string{},
			b:        map[string]string{},
			expected: true,
		},
		{
			name:     "both nil",
			a:        nil,
			b:        nil,
			expected: true,
		},
		{
			name:     "equal maps",
			a:        map[string]string{"key1": "value1", "key2": "value2"},
			b:        map[string]string{"key1": "value1", "key2": "value2"},
			expected: true,
		},
		{
			name:     "different lengths",
			a:        map[string]string{"key1": "value1"},
			b:        map[string]string{"key1": "value1", "key2": "value2"},
			expected: false,
		},
		{
			name:     "different values",
			a:        map[string]string{"key1": "value1"},
			b:        map[string]string{"key1": "value2"},
			expected: false,
		},
		{
			name:     "missing key in b",
			a:        map[string]string{"key1": "value1", "key2": "value2"},
			b:        map[string]string{"key1": "value1"},
			expected: false,
		},
		{
			name:     "one empty one nil",
			a:        map[string]string{},
			b:        nil,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := equalStringMaps(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("equalStringMaps() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestCompareSecrets tests the compareSecrets function
func TestCompareSecrets(t *testing.T) {
	tests := []struct {
		name     string
		existing *v1.Secret
		source   *v1.Secret
		expected bool
	}{
		{
			name: "identical secrets",
			existing: &v1.Secret{
				Data: map[string][]byte{"key1": []byte("value1")},
			},
			source: &v1.Secret{
				Data: map[string][]byte{"key1": []byte("value1")},
			},
			expected: true,
		},
		{
			name: "different data",
			existing: &v1.Secret{
				Data: map[string][]byte{"key1": []byte("value1")},
			},
			source: &v1.Secret{
				Data: map[string][]byte{"key1": []byte("value2")},
			},
			expected: false,
		},
		{
			name: "different stringdata",
			existing: &v1.Secret{
				Data:       map[string][]byte{"key1": []byte("value1")},
				StringData: map[string]string{"str1": "string1"},
			},
			source: &v1.Secret{
				Data:       map[string][]byte{"key1": []byte("value1")},
				StringData: map[string]string{"str1": "string2"},
			},
			expected: false,
		},
		{
			name: "empty secrets",
			existing: &v1.Secret{
				Data: map[string][]byte{},
			},
			source: &v1.Secret{
				Data: map[string][]byte{},
			},
			expected: true,
		},
		{
			name: "metadata differences ignored",
			existing: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "secret1",
					ResourceVersion: "123",
				},
				Data: map[string][]byte{"key1": []byte("value1")},
			},
			source: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "secret1",
					ResourceVersion: "456",
				},
				Data: map[string][]byte{"key1": []byte("value1")},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareSecrets(tt.existing, tt.source)
			if result != tt.expected {
				t.Errorf("compareSecrets() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestGetSourceSecrets tests the getSourceSecrets function
func TestGetSourceSecrets(t *testing.T) {
	logger := newTestLogger()

	tests := []struct {
		name          string
		namespace     string
		secrets       []v1.Secret
		expectedCount int
		expectError   bool
	}{
		{
			name:      "secrets found",
			namespace: "test-namespace",
			secrets: []v1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret1",
						Namespace: "test-namespace",
						Labels:    map[string]string{"push-to-k8s": "source"},
					},
					Data: map[string][]byte{"key1": []byte("value1")},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret2",
						Namespace: "test-namespace",
						Labels:    map[string]string{"push-to-k8s": "source"},
					},
					Data: map[string][]byte{"key2": []byte("value2")},
				},
			},
			expectedCount: 2,
			expectError:   false,
		},
		{
			name:          "no secrets found",
			namespace:     "empty-namespace",
			secrets:       []v1.Secret{},
			expectedCount: 0,
			expectError:   false,
		},
		{
			name:      "secrets without label ignored",
			namespace: "test-namespace",
			secrets: []v1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret-no-label",
						Namespace: "test-namespace",
					},
					Data: map[string][]byte{"key1": []byte("value1")},
				},
			},
			expectedCount: 0,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake clientset
			clientset := fake.NewSimpleClientset()

			// Add secrets with the correct label
			for _, secret := range tt.secrets {
				if secret.Labels != nil && secret.Labels["push-to-k8s"] == "source" {
					_, err := clientset.CoreV1().Secrets(tt.namespace).Create(context.TODO(), &secret, metav1.CreateOptions{})
					if err != nil {
						t.Fatalf("failed to create test secret: %v", err)
					}
				}
			}

			// Test getSourceSecrets
			result, err := getSourceSecrets(clientset, tt.namespace, logger)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if len(result) != tt.expectedCount {
				t.Errorf("expected %d secrets, got %d", tt.expectedCount, len(result))
			}
		})
	}
}

// TestSyncSecretToNamespace tests the syncSecretToNamespace function
func TestSyncSecretToNamespace(t *testing.T) {
	logger := newTestLogger()

	tests := []struct {
		name                  string
		sourceSecret          *v1.Secret
		targetNamespace       string
		existingSecret        *v1.Secret
		excludeLabel          string
		namespaceLabels       map[string]string
		expectCreate          bool
		expectUpdate          bool
		expectSkip            bool
		expectExclude         bool
	}{
		{
			name: "create new secret",
			sourceSecret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test-secret",
					Labels: map[string]string{"push-to-k8s": "source"},
				},
				Data: map[string][]byte{"key1": []byte("value1")},
			},
			targetNamespace: "target-ns",
			existingSecret:  nil,
			expectCreate:    true,
		},
		{
			name: "update existing different secret",
			sourceSecret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test-secret",
					Labels: map[string]string{"push-to-k8s": "source"},
				},
				Data: map[string][]byte{"key1": []byte("value2")},
			},
			targetNamespace: "target-ns",
			existingSecret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-secret",
					Namespace:       "target-ns",
					ResourceVersion: "100",
				},
				Data: map[string][]byte{"key1": []byte("value1")},
			},
			expectUpdate: true,
		},
		{
			name: "skip identical secret",
			sourceSecret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test-secret",
					Labels: map[string]string{"push-to-k8s": "source"},
				},
				Data: map[string][]byte{"key1": []byte("value1")},
			},
			targetNamespace: "target-ns",
			existingSecret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "target-ns",
				},
				Data: map[string][]byte{"key1": []byte("value1")},
			},
			expectSkip: true,
		},
		{
			name: "exclude namespace with label",
			sourceSecret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test-secret",
					Labels: map[string]string{"push-to-k8s": "source"},
				},
				Data: map[string][]byte{"key1": []byte("value1")},
			},
			targetNamespace: "excluded-ns",
			excludeLabel:    "push-to-k8s-exclude",
			namespaceLabels: map[string]string{"push-to-k8s-exclude": "true"},
			expectExclude:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake clientset
			clientset := fake.NewSimpleClientset()

			// Create target namespace with labels
			ns := &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   tt.targetNamespace,
					Labels: tt.namespaceLabels,
				},
			}
			_, err := clientset.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("failed to create test namespace: %v", err)
			}

			// Create existing secret if specified
			if tt.existingSecret != nil {
				_, err := clientset.CoreV1().Secrets(tt.targetNamespace).Create(context.TODO(), tt.existingSecret, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("failed to create existing secret: %v", err)
				}
			}

			// Test syncSecretToNamespace
			err = syncSecretToNamespace(clientset, tt.sourceSecret, tt.targetNamespace, tt.excludeLabel, logger)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Verify results
			if tt.expectExclude {
				// If excluded, secret should not be created
				_, err := clientset.CoreV1().Secrets(tt.targetNamespace).Get(context.TODO(), tt.sourceSecret.Name, metav1.GetOptions{})
				if err == nil && tt.existingSecret == nil {
					t.Error("expected secret to not be created in excluded namespace")
				}
				return
			}

			// Get the secret from target namespace
			resultSecret, err := clientset.CoreV1().Secrets(tt.targetNamespace).Get(context.TODO(), tt.sourceSecret.Name, metav1.GetOptions{})
			if err != nil {
				if tt.expectCreate || tt.expectUpdate || tt.expectSkip {
					t.Fatalf("failed to get result secret: %v", err)
				}
				return
			}

			// Verify secret data matches source
			if !equalByteMaps(resultSecret.Data, tt.sourceSecret.Data) {
				t.Error("secret data does not match source")
			}

			// Verify source label was removed
			if resultSecret.Labels != nil {
				if _, exists := resultSecret.Labels["push-to-k8s"]; exists {
					t.Error("source label 'push-to-k8s' should have been removed")
				}
			}

			// Verify namespace is correct
			if resultSecret.Namespace != tt.targetNamespace {
				t.Errorf("expected namespace %s, got %s", tt.targetNamespace, resultSecret.Namespace)
			}
		})
	}
}

// TestSyncSecretsToSingleNamespace tests the syncSecretsToSingleNamespace function
func TestSyncSecretsToSingleNamespace(t *testing.T) {
	logger := newTestLogger()

	t.Run("sync multiple secrets to single namespace", func(t *testing.T) {
		// Create fake clientset
		clientset := fake.NewSimpleClientset()

		// Create source namespace and secrets
		sourceNS := "source-ns"
		targetNS := "target-ns"

		// Create namespaces
		_, err := clientset.CoreV1().Namespaces().Create(context.TODO(), &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: sourceNS},
		}, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("failed to create source namespace: %v", err)
		}

		_, err = clientset.CoreV1().Namespaces().Create(context.TODO(), &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: targetNS},
		}, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("failed to create target namespace: %v", err)
		}

		// Create source secrets
		for i := 1; i <= 3; i++ {
			secret := &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret" + string(rune('0'+i)),
					Namespace: sourceNS,
					Labels:    map[string]string{"push-to-k8s": "source"},
				},
				Data: map[string][]byte{"key": []byte("value")},
			}
			_, err := clientset.CoreV1().Secrets(sourceNS).Create(context.TODO(), secret, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("failed to create source secret: %v", err)
			}
		}

		// Test syncSecretsToSingleNamespace
		err = syncSecretsToSingleNamespace(clientset, sourceNS, targetNS, "", logger)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Verify all secrets were synced to target namespace
		secrets, err := clientset.CoreV1().Secrets(targetNS).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			t.Fatalf("failed to list target secrets: %v", err)
		}

		if len(secrets.Items) != 3 {
			t.Errorf("expected 3 secrets in target namespace, got %d", len(secrets.Items))
		}
	})
}
