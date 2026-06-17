package util

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/SAP/cad-operator/api/v1alpha1"
)

func LabelsForComponent(crName, component string) map[string]string {
	return map[string]string{
		"app":                          component,
		"app.kubernetes.io/name":       component,
		"app.kubernetes.io/instance":   crName,
		"app.kubernetes.io/managed-by": "cad-operator",
	}
}

func SelectorLabels(component string) map[string]string {
	return map[string]string{
		"app": component,
	}
}

func ClusterResourceName(cad *operatorv1alpha1.CloudActiveDefense, name string) string {
	return fmt.Sprintf("%s-%s", cad.Namespace, name)
}

func DefaultString(val, def string) string {
	if val != "" {
		return val
	}
	return def
}

func DefaultInt32(val, def int32) int32 {
	if val != 0 {
		return val
	}
	return def
}

// GetClusterDomain attempts to fetch the Kyma cluster domain from standard locations.
// It first checks the shoot-info ConfigMap (for Gardener/BTP clusters),
// then falls back to the provided domain if available.
func GetClusterDomain(ctx context.Context, client client.Client, providedDomain string) (string, error) {
	// If domain is explicitly provided, use it
	if providedDomain != "" {
		return providedDomain, nil
	}

	// Try to fetch from shoot-info ConfigMap (Gardener/BTP standard location)
	cm := &corev1.ConfigMap{}
	if err := client.Get(ctx, types.NamespacedName{
		Namespace: "kube-system",
		Name:      "shoot-info",
	}, cm); err == nil {
		if domain, ok := cm.Data["domain"]; ok && domain != "" {
			return domain, nil
		}
	}

	// Could not determine domain
	return "", fmt.Errorf("unable to determine cluster domain: please specify 'domain' in the CloudActiveDefense spec")
}

func GeneratePassword(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		b[i] = charset[n.Int64()]
	}
	return string(b), nil
}

// GenerateUsername creates a random username with the given prefix.
// The username format is: <prefix>_<8_random_chars>
func GenerateUsername(prefix string) (string, error) {
	randomPart, err := GeneratePassword(8)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%s", prefix, randomPart), nil
}

// IsUsernameField checks if a secret key represents a username field.
func IsUsernameField(key string) bool {
	// Check for common username field patterns
	return key == "POSTGRES_USER" || 
		   key == "KC_BOOTSTRAP_ADMIN_USERNAME" ||
		   key == "DEPLOYMENT_MANAGER_USER"
}
