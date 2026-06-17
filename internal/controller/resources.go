/*
Copyright 2026.

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

package controller

import (
	"fmt"
	"strconv"

	kymagwv2 "github.com/kyma-project/api-gateway/apis/gateway/v2"
	istiosecv1b1 "istio.io/api/security/v1beta1"
	istiotypev1b1 "istio.io/api/type/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	operatorv1alpha1 "github.com/SAP/cad-operator/api/v1alpha1"
	"github.com/SAP/cad-operator/internal/util"
)

const (
	kymaGateway = "kyma-system/kyma-gateway"
)

var replicas int32 = 1

// --- Controlpanel DB ---

func controlpanelDBDeploymentSpec(cad *operatorv1alpha1.CloudActiveDefense) appsv1.DeploymentSpec {
	return appsv1.DeploymentSpec{
		Replicas: &replicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: util.SelectorLabels("controlpanel-db"),
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: util.SelectorLabels("controlpanel-db"),
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:            "controlpanel-db",
					Image:           "postgres:17.5-alpine",
					ImagePullPolicy: corev1.PullAlways,
					Ports:           []corev1.ContainerPort{{ContainerPort: 5432}},
					EnvFrom: []corev1.EnvFromSource{{
						SecretRef: &corev1.SecretEnvSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: "controlpanel-db-secrets"},
						},
					}},
					Env: []corev1.EnvVar{
						{Name: "POSTGRES_DB", Value: "cad"},
						{Name: "PGDATA", Value: "/var/lib/postgresql/data/cad"},
					},
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "postgres-data",
						MountPath: "/var/lib/postgresql/data",
					}},
				}},
				Volumes: []corev1.Volume{{
					Name: "postgres-data",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: "controlpanel-pvc",
						},
					},
				}},
			},
		},
	}
}

// --- Controlpanel API ---

func controlpanelAPIDeploymentSpec(cad *operatorv1alpha1.CloudActiveDefense) appsv1.DeploymentSpec {
	return appsv1.DeploymentSpec{
		Replicas: &replicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: util.SelectorLabels("controlpanel-api"),
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"app":                     "controlpanel-api",
					"sidecar.istio.io/inject": "true",
				},
			},
			Spec: corev1.PodSpec{
				ImagePullSecrets: []corev1.LocalObjectReference{
					{Name: "artifactory-creds"},
				},
				InitContainers: []corev1.Container{{
					Name:    "wait-for-db",
					Image:   "postgres:16-alpine",
					Command: []string{"sh", "-c", "until pg_isready -h controlpanel-db-service -p 5432 -U $(POSTGRES_USER); do echo waiting for db; sleep 2; done"},
					Env: []corev1.EnvVar{{
						Name: "POSTGRES_USER",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "controlpanel-db-secrets"},
								Key:                  "POSTGRES_USER",
							},
						},
					}},
				}},
				Containers: []corev1.Container{{
					Name:            "controlpanel-api",
					Image:           util.DefaultString(cad.Spec.ControlpanelAPI.Image, "ghcr.io/sap/controlpanel-api:latest"),
					ImagePullPolicy: corev1.PullAlways,
					Ports:           []corev1.ContainerPort{{ContainerPort: 8050}},
					Env: []corev1.EnvVar{
						{Name: "DB_PORT", Value: strconv.Itoa(int(util.DefaultInt32(cad.Spec.Database.Port, 5432)))},
						{Name: "DB_HOST", Value: "controlpanel-db-service"},
						{Name: "CONTROLPANEL_FRONTEND_URL", Value: fmt.Sprintf("https://controlpanel-front.%s", cad.Spec.Domain)},
						{Name: "DEPLOYMENT_MANAGER_URL", Value: "http://deployment-manager-service"},
						{Name: "KEYCLOAK_URL", Value: fmt.Sprintf("https://keycloak.%s", cad.Spec.Domain)},
					},
					EnvFrom: []corev1.EnvFromSource{
						{SecretRef: &corev1.SecretEnvSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: "controlpanel-db-secrets"},
						}},
						{SecretRef: &corev1.SecretEnvSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: "deployment-manager-db-secrets"},
						}},
					},
				}},
			},
		},
	}
}

// --- Controlpanel Front ---

func controlpanelFrontDeploymentSpec(cad *operatorv1alpha1.CloudActiveDefense) appsv1.DeploymentSpec {
	return appsv1.DeploymentSpec{
		Replicas: &replicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: util.SelectorLabels("controlpanel-front"),
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"app":                     "controlpanel-front",
					"sidecar.istio.io/inject": "true",
				},
				Annotations: map[string]string{
					"traffic.sidecar.istio.io/excludeOutboundPorts": "9000",
				},
			},
			Spec: corev1.PodSpec{
				InitContainers: []corev1.Container{{
					Name:    "wait-for-keycloak",
					Image:   "busybox",
					Command: []string{"sh", "-c", "until wget -q -O /dev/null keycloak-service:9000/health/ready 2>/dev/null; do echo waiting for keycloak; sleep 8; done"},
				}},
				Containers: []corev1.Container{{
					Name:            "controlpanel-front",
					Image:           util.DefaultString(cad.Spec.ControlpanelFront.Image, "ghcr.io/sap/controlpanel-frontend:latest"),
					ImagePullPolicy: corev1.PullAlways,
					Ports:           []corev1.ContainerPort{{ContainerPort: 80}},
					Env: []corev1.EnvVar{
						{Name: "CONTROLPANEL_API_URL", Value: fmt.Sprintf("https://controlpanel-api.%s", cad.Spec.Domain)},
						{Name: "KEYCLOAK_URL", Value: fmt.Sprintf("https://keycloak.%s", cad.Spec.Domain)},
					},
				}},
			},
		},
	}
}

// --- Deployment Manager ---

func deploymentManagerDeploymentSpec(cad *operatorv1alpha1.CloudActiveDefense) appsv1.DeploymentSpec {
	return appsv1.DeploymentSpec{
		Replicas: &replicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: util.SelectorLabels("deployment-manager"),
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: util.SelectorLabels("deployment-manager"),
			},
			Spec: corev1.PodSpec{
				ServiceAccountName: "deployment-manager-sa",
				InitContainers: []corev1.Container{
					{
						Name:    "wait-for-db",
						Image:   "postgres:16-alpine",
						Command: []string{"sh", "-c", "until pg_isready -h controlpanel-db-service -p 5432 -U $(POSTGRES_USER); do echo waiting for db; sleep 2; done"},
						Env: []corev1.EnvVar{{
							Name: "POSTGRES_USER",
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: "controlpanel-db-secrets"},
									Key:                  "POSTGRES_USER",
								},
							},
						}},
					},
					{
						Name:    "wait-for-api",
						Image:   "busybox",
						Command: []string{"sh", "-c", "until nc -z -v controlpanel-api-service 80; do echo waiting for api; sleep 2; done"},
					},
				},
				Containers: []corev1.Container{{
					Name:            "deployment-manager",
					Image:           util.DefaultString(cad.Spec.DeploymentManager.Image, "ghcr.io/sap/deployment-manager:latest"),
					ImagePullPolicy: corev1.PullAlways,
					Ports:           []corev1.ContainerPort{{ContainerPort: 3000}},
					Env: []corev1.EnvVar{
						{Name: "DB_PORT", Value: strconv.Itoa(int(util.DefaultInt32(cad.Spec.Database.Port, 5432)))},
						{Name: "DB_HOST", Value: "controlpanel-db-service"},
						{Name: "POD_NAMESPACE", ValueFrom: &corev1.EnvVarSource{
							FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"},
						}},
					},
					EnvFrom: []corev1.EnvFromSource{
						{SecretRef: &corev1.SecretEnvSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: "deployment-manager-db-secrets"},
						}},
					},
				}},
			},
		},
	}
}

// --- Keycloak DB ---

func keycloakDBDeploymentSpec(cad *operatorv1alpha1.CloudActiveDefense) appsv1.DeploymentSpec {
	return appsv1.DeploymentSpec{
		Replicas: &replicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: util.SelectorLabels("keycloak-db"),
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: util.SelectorLabels("keycloak-db"),
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:            "keycloak-db",
					Image:           "postgres:17.5-alpine",
					ImagePullPolicy: corev1.PullAlways,
					Ports:           []corev1.ContainerPort{{ContainerPort: 5432}},
					EnvFrom: []corev1.EnvFromSource{{
						SecretRef: &corev1.SecretEnvSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: "keycloak-db-secrets"},
						},
					}},
					Env: []corev1.EnvVar{
						{Name: "POSTGRES_DB", Value: "keycloak"},
						{Name: "PGDATA", Value: "/var/lib/postgresql/data/keycloak"},
					},
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "keycloak-data",
						MountPath: "/var/lib/postgresql/data",
					}},
				}},
				Volumes: []corev1.Volume{{
					Name: "keycloak-data",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: "keycloak-pvc",
						},
					},
				}},
			},
		},
	}
}

// --- Keycloak ---

func keycloakDeploymentSpec(cad *operatorv1alpha1.CloudActiveDefense) appsv1.DeploymentSpec {
	return appsv1.DeploymentSpec{
		Replicas: &replicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: util.SelectorLabels("keycloak"),
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"app":                     "keycloak",
					"sidecar.istio.io/inject": "true",
				},
				Annotations: map[string]string{
					"traffic.sidecar.istio.io/excludeInboundPorts": "9000",
				},
			},
			Spec: corev1.PodSpec{
				ServiceAccountName: "keycloak-sa",
				InitContainers: []corev1.Container{
					{
						Name:    "wait-for-db",
						Image:   "busybox",
						Command: []string{"sh", "-c", "until nc -z -v keycloak-db-service 5432; do echo waiting for db; sleep 2; done"},
					},
					{
						Name:  "wait-for-secret",
						Image: "bitnami/kubectl:latest",
						Command: []string{"sh", "-c", fmt.Sprintf(
							`until [ "$(kubectl get secret keycloak-secrets -n %s -o jsonpath='{.data.KEYCLOAK_API_KEY}' | base64 -d)" != "" ]; do echo "Waiting for secret"; sleep 5; done`,
							cad.Namespace,
						)},
					},
				},
				Containers: []corev1.Container{{
					Name:            "keycloak",
					Image:           util.DefaultString(cad.Spec.Keycloak.Image, "ghcr.io/sap/keycloak-controlpanel:latest"),
					ImagePullPolicy: corev1.PullAlways,
					Ports: []corev1.ContainerPort{
						{ContainerPort: 8080},
						{ContainerPort: 8443},
					},
					Env: []corev1.EnvVar{
						{Name: "CONTROLPANEL_API_URL", Value: "http://controlpanel-api-service"},
						{Name: "APP_URL", Value: fmt.Sprintf("https://controlpanel-front.%s/*", cad.Spec.Domain)},
						{Name: "KC_DB_URL_HOST", Value: "keycloak-db-service"},
						{Name: "KC_DB_USERNAME", ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "keycloak-db-secrets"},
								Key:                  "POSTGRES_USER",
							},
						}},
						{Name: "KC_DB_PASSWORD", ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "keycloak-db-secrets"},
								Key:                  "POSTGRES_PASSWORD",
							},
						}},
						{Name: "KC_HOSTNAME", Value: fmt.Sprintf("https://keycloak.%s", cad.Spec.Domain)},
						{Name: "KC_HTTP_ENABLED", Value: "true"},
					},
					EnvFrom: []corev1.EnvFromSource{
						{SecretRef: &corev1.SecretEnvSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: "keycloak-db-secrets"},
						}},
						{SecretRef: &corev1.SecretEnvSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: "keycloak-secrets"},
						}},
					},
				}},
			},
		},
	}
}

// --- Services ---

func controlpanelDBServiceSpec(cad *operatorv1alpha1.CloudActiveDefense) corev1.ServiceSpec {
	return corev1.ServiceSpec{
		Type: corev1.ServiceTypeClusterIP,
		Ports: []corev1.ServicePort{{
			Name: "tcp-postgres", Port: util.DefaultInt32(cad.Spec.Database.Port, 5432),
			Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt32(5432),
		}},
		Selector: util.SelectorLabels("controlpanel-db"),
	}
}

func controlpanelAPIServiceSpec(_ *operatorv1alpha1.CloudActiveDefense) corev1.ServiceSpec {
	return corev1.ServiceSpec{
		Type: corev1.ServiceTypeClusterIP,
		Ports: []corev1.ServicePort{{
			Name: "http", Port: 80, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt32(8050),
		}},
		Selector: util.SelectorLabels("controlpanel-api"),
	}
}

func controlpanelFrontServiceSpec(_ *operatorv1alpha1.CloudActiveDefense) corev1.ServiceSpec {
	return corev1.ServiceSpec{
		Type: corev1.ServiceTypeClusterIP,
		Ports: []corev1.ServicePort{{
			Name: "http", Port: 80, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt32(80),
		}},
		Selector: util.SelectorLabels("controlpanel-front"),
	}
}

func deploymentManagerServiceSpec(_ *operatorv1alpha1.CloudActiveDefense) corev1.ServiceSpec {
	return corev1.ServiceSpec{
		Type: corev1.ServiceTypeClusterIP,
		Ports: []corev1.ServicePort{{
			Name: "http", Port: 80, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt32(3000),
		}},
		Selector: util.SelectorLabels("deployment-manager"),
	}
}

func keycloakDBServiceSpec(cad *operatorv1alpha1.CloudActiveDefense) corev1.ServiceSpec {
	return corev1.ServiceSpec{
		Type: corev1.ServiceTypeClusterIP,
		Ports: []corev1.ServicePort{{
			Name: "tcp-postgres", Port: util.DefaultInt32(cad.Spec.Keycloak.Database.Port, 5432),
			Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt32(5432),
		}},
		Selector: util.SelectorLabels("keycloak-db"),
	}
}

func keycloakServiceSpec(_ *operatorv1alpha1.CloudActiveDefense) corev1.ServiceSpec {
	return corev1.ServiceSpec{
		Type: corev1.ServiceTypeClusterIP,
		Ports: []corev1.ServicePort{
			{Name: "https", Port: 443, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt32(8443)},
			{Name: "http", Port: 80, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt32(8080)},
			{Name: "tcp-health", Port: 9000, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt32(9000)},
		},
		Selector: util.SelectorLabels("keycloak"),
	}
}

// --- PVCs ---

func pvcSpec() corev1.PersistentVolumeClaimSpec {
	return corev1.PersistentVolumeClaimSpec{
		AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("10Gi"),
			},
		},
	}
}

// --- RBAC ---

func deploymentManagerClusterRoleRules() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{Verbs: []string{"get"}, APIGroups: []string{""}, Resources: []string{"namespaces"}},
		{Verbs: []string{"patch"}, APIGroups: []string{""}, Resources: []string{"secrets"}},
	}
}

func keycloakClusterRoleRules() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{Verbs: []string{"get"}, APIGroups: []string{""}, Resources: []string{"secrets"}},
	}
}

// --- AuthorizationPolicies ---

// keycloakToAPIAuthPolicySpec allows the keycloak SA to POST /customer on controlpanel-api.
func keycloakToAPIAuthPolicySpec(cad *operatorv1alpha1.CloudActiveDefense) istiosecv1b1.AuthorizationPolicy {
	return istiosecv1b1.AuthorizationPolicy{
		Selector: &istiotypev1b1.WorkloadSelector{
			MatchLabels: map[string]string{"app": "controlpanel-api"},
		},
		Rules: []*istiosecv1b1.Rule{{
			From: []*istiosecv1b1.Rule_From{{
				Source: &istiosecv1b1.Source{
					Principals: []string{fmt.Sprintf("cluster.local/ns/%s/sa/keycloak-sa", cad.Namespace)},
				},
			}},
			To: []*istiosecv1b1.Rule_To{{
				Operation: &istiosecv1b1.Operation{
					Methods: []string{"POST"},
					Paths:   []string{"/customer"},
				},
			}},
		}},
	}
}

// telemetryToAPIAuthPolicySpec allows Kyma telemetry to POST /logs on controlpanel-api.
func telemetryToAPIAuthPolicySpec(_ *operatorv1alpha1.CloudActiveDefense) istiosecv1b1.AuthorizationPolicy {
	return istiosecv1b1.AuthorizationPolicy{
		Selector: &istiotypev1b1.WorkloadSelector{
			MatchLabels: map[string]string{"app": "controlpanel-api"},
		},
		Rules: []*istiosecv1b1.Rule{{
			From: []*istiosecv1b1.Rule_From{{
				Source: &istiosecv1b1.Source{
					Principals: []string{"cluster.local/ns/kyma-system/sa/telemetry-fluent-bit"},
				},
			}},
			To: []*istiosecv1b1.Rule_To{{
				Operation: &istiosecv1b1.Operation{
					Methods: []string{"POST"},
					Paths:   []string{"/logs"},
				},
			}},
		}},
	}
}

// wasmToAPIAuthPolicySpec allows any workload (WASM sidecar) to GET/POST /configmanager/* on controlpanel-api.
func wasmToAPIAuthPolicySpec(_ *operatorv1alpha1.CloudActiveDefense) istiosecv1b1.AuthorizationPolicy {
	return istiosecv1b1.AuthorizationPolicy{
		Selector: &istiotypev1b1.WorkloadSelector{
			MatchLabels: map[string]string{"app": "controlpanel-api"},
		},
		Rules: []*istiosecv1b1.Rule{{
			From: []*istiosecv1b1.Rule_From{{
				Source: &istiosecv1b1.Source{
					Principals: []string{"*"},
				},
			}},
			To: []*istiosecv1b1.Rule_To{{
				Operation: &istiosecv1b1.Operation{
					Methods: []string{"GET", "POST"},
					Paths:   []string{"/configmanager/*"},
				},
			}},
		}},
	}
}

// --- APIRules ---

func controlpanelAPIAPIRuleSpec(cad *operatorv1alpha1.CloudActiveDefense) kymagwv2.APIRuleSpec {
	noAuth := true
	allowCredentials := true
	host := kymagwv2.Host("controlpanel-api")
	gateway := kymaGateway
	svcName := "controlpanel-api-service"
	svcPort := uint32(80)
	frontURL := fmt.Sprintf("https://controlpanel-front.%s", cad.Spec.Domain)
	return kymagwv2.APIRuleSpec{
		Hosts:   []*kymagwv2.Host{&host},
		Gateway: &gateway,
		Service: &kymagwv2.Service{
			Name: &svcName,
			Port: &svcPort,
		},
		CorsPolicy: &kymagwv2.CorsPolicy{
			AllowOrigins:     kymagwv2.StringMatch{{"prefix": frontURL}},
			AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowHeaders:     []string{"Authorization", "Content-Type"},
			ExposeHeaders:    []string{"Content-Type"},
			AllowCredentials: &allowCredentials,
		},
		Rules: []kymagwv2.Rule{{
			Path:    "/*",
			Methods: []kymagwv2.HttpMethod{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			NoAuth:  &noAuth,
		}},
	}
}

func controlpanelFrontAPIRuleSpec(_ *operatorv1alpha1.CloudActiveDefense) kymagwv2.APIRuleSpec {
	noAuth := true
	host := kymagwv2.Host("controlpanel-front")
	gateway := kymaGateway
	svcName := "controlpanel-front-service"
	svcPort := uint32(80)
	return kymagwv2.APIRuleSpec{
		Hosts:   []*kymagwv2.Host{&host},
		Gateway: &gateway,
		Service: &kymagwv2.Service{
			Name: &svcName,
			Port: &svcPort,
		},
		Rules: []kymagwv2.Rule{{
			Path:    "/*",
			Methods: []kymagwv2.HttpMethod{"GET"},
			NoAuth:  &noAuth,
		}},
	}
}

func keycloakAPIRuleSpec(cad *operatorv1alpha1.CloudActiveDefense) kymagwv2.APIRuleSpec {
	noAuth := true
	allowCredentials := true
	host := kymagwv2.Host("keycloak")
	gateway := kymaGateway
	svcName := "keycloak-service"
	svcPort := uint32(80)
	frontURL := fmt.Sprintf("https://controlpanel-front.%s", cad.Spec.Domain)
	return kymagwv2.APIRuleSpec{
		Hosts:   []*kymagwv2.Host{&host},
		Gateway: &gateway,
		Service: &kymagwv2.Service{
			Name: &svcName,
			Port: &svcPort,
		},
		CorsPolicy: &kymagwv2.CorsPolicy{
			AllowOrigins:     kymagwv2.StringMatch{{"prefix": frontURL}},
			AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowHeaders:     []string{"Authorization", "Content-Type"},
			ExposeHeaders:    []string{"Content-Type"},
			AllowCredentials: &allowCredentials,
		},
		Rules: []kymagwv2.Rule{{
			Path:    "/*",
			Methods: []kymagwv2.HttpMethod{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			NoAuth:  &noAuth,
		}},
	}
}
