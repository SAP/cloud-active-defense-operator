package util

import (
	"crypto/rand"
	"fmt"
	"math/big"

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
