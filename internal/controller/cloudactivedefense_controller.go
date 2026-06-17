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
	"context"
	"fmt"

	kymagwv2 "github.com/kyma-project/api-gateway/apis/gateway/v2"
	istiosecv1b1 "istio.io/api/security/v1beta1"
	istioclientv1 "istio.io/client-go/pkg/apis/security/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/SAP/cad-operator/api/v1alpha1"
	"github.com/SAP/cad-operator/internal/util"
)

const finalizerName = "operator.sundew.com/finalizer"

// CloudActiveDefenseReconciler reconciles a CloudActiveDefense object
type CloudActiveDefenseReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=operator.sundew.com,resources=cloudactivedefenses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.sundew.com,resources=cloudactivedefenses/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.sundew.com,resources=cloudactivedefenses/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list
// +kubebuilder:rbac:groups=gateway.kyma-project.io,resources=apirules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=security.istio.io,resources=authorizationpolicies,verbs=get;list;watch;create;update;patch;delete

// Reconcile ensures the cluster state matches the desired state specified
// by the CloudActiveDefense custom resource.
func (r *CloudActiveDefenseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	cad := &operatorv1alpha1.CloudActiveDefense{}
	if err := r.Get(ctx, req.NamespacedName, cad); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Resource not found, ignoring")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !cad.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(cad, finalizerName) {
			log.Info("Cleaning up cluster-scoped resources")
			if err := r.cleanupClusterResources(ctx, cad); err != nil {
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(cad, finalizerName)
			if err := r.Update(ctx, cad); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Add finalizer for cluster-scoped resource cleanup
	if !controllerutil.ContainsFinalizer(cad, finalizerName) {
		controllerutil.AddFinalizer(cad, finalizerName)
		if err := r.Update(ctx, cad); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Resolve cluster domain if not provided
	domain, err := util.GetClusterDomain(ctx, r.Client, cad.Spec.Domain)
	if err != nil {
		log.Error(err, "Failed to resolve cluster domain")
		meta.SetStatusCondition(&cad.Status.Conditions, metav1.Condition{
			Type:               "Degraded",
			Status:             metav1.ConditionTrue,
			Reason:             "DomainResolutionFailed",
			Message:            err.Error(),
			ObservedGeneration: cad.Generation,
		})
		_ = r.Status().Update(ctx, cad)
		return ctrl.Result{}, err
	}

	// Store resolved domain in status for visibility
	if cad.Status.ResolvedDomain != domain {
		cad.Status.ResolvedDomain = domain
		if err := r.Status().Update(ctx, cad); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("Resolved cluster domain", "domain", domain)
	}

	// Use resolved domain in the cad spec for resource creation
	cad.Spec.Domain = domain

	// Reconcile all components in dependency order
	reconcilers := []struct {
		name string
		fn   func(context.Context, *operatorv1alpha1.CloudActiveDefense) error
	}{
		{"Secrets", r.reconcileSecrets},
		{"PVCs", r.reconcilePVCs},
		{"RBAC", r.reconcileRBAC},
		{"Deployments", r.reconcileDeployments},
		{"Services", r.reconcileServices},
		{"APIRules", r.reconcileAPIRules},
		{"AuthPolicies", r.reconcileAuthPolicies},
	}

	for _, rec := range reconcilers {
		if err := rec.fn(ctx, cad); err != nil {
			log.Error(err, "Failed to reconcile "+rec.name)
			meta.SetStatusCondition(&cad.Status.Conditions, metav1.Condition{
				Type:               "Degraded",
				Status:             metav1.ConditionTrue,
				Reason:             rec.name + "Failed",
				Message:            err.Error(),
				ObservedGeneration: cad.Generation,
			})
			_ = r.Status().Update(ctx, cad)
			return ctrl.Result{}, err
		}
	}

	// All resources reconciled successfully
	meta.SetStatusCondition(&cad.Status.Conditions, metav1.Condition{
		Type:               "Available",
		Status:             metav1.ConditionTrue,
		Reason:             "Reconciled",
		Message:            "All resources reconciled successfully",
		ObservedGeneration: cad.Generation,
	})
	meta.SetStatusCondition(&cad.Status.Conditions, metav1.Condition{
		Type:               "Degraded",
		Status:             metav1.ConditionFalse,
		Reason:             "Reconciled",
		Message:            "All resources healthy",
		ObservedGeneration: cad.Generation,
	})
	if err := r.Status().Update(ctx, cad); err != nil {
		return ctrl.Result{}, err
	}

	log.Info("Reconciliation complete")
	return ctrl.Result{}, nil
}

// cleanupClusterResources deletes cluster-scoped resources that cannot use owner references.
func (r *CloudActiveDefenseReconciler) cleanupClusterResources(ctx context.Context, cad *operatorv1alpha1.CloudActiveDefense) error {
	log := logf.FromContext(ctx)
	resources := []client.Object{
		&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: util.ClusterResourceName(cad, "deployment-manager-cr")}},
		&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: util.ClusterResourceName(cad, "deployment-manager-crb")}},
		&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: util.ClusterResourceName(cad, "keycloak-cr")}},
		&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: util.ClusterResourceName(cad, "keycloak-crb")}},
	}
	for _, obj := range resources {
		if err := r.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) {
			log.Error(err, "Failed to delete cluster resource", "name", obj.GetName())
			return err
		}
	}
	return nil
}

// --- Secret reconciliation ---

func (r *CloudActiveDefenseReconciler) reconcileSecrets(ctx context.Context, cad *operatorv1alpha1.CloudActiveDefense) error {
	secrets := []struct {
		name string
		data map[string]string
	}{
		{
			name: "controlpanel-db-secrets",
			data: map[string]string{
				"POSTGRES_USER":     cad.Spec.Database.User,
				"POSTGRES_PASSWORD": cad.Spec.Database.Password,
			},
		},
		{
			name: "deployment-manager-db-secrets",
			data: map[string]string{
				"DEPLOYMENT_MANAGER_PASSWORD": cad.Spec.ControlpanelAPI.DeploymentManagerDBPassword,
			},
		},
		{
			name: "keycloak-db-secrets",
			data: map[string]string{
				"POSTGRES_USER":     cad.Spec.Keycloak.Database.User,
				"POSTGRES_PASSWORD": cad.Spec.Keycloak.Database.Password,
			},
		},
		{
			name: "keycloak-secrets",
			data: map[string]string{
				"KC_BOOTSTRAP_ADMIN_USERNAME": cad.Spec.Keycloak.AdminUsername,
				"KC_BOOTSTRAP_ADMIN_PASSWORD": cad.Spec.Keycloak.AdminPassword,
			},
		},
	}

	for _, s := range secrets {
		if err := r.ensureSecret(ctx, cad, s.name, s.data); err != nil {
			return fmt.Errorf("secret %s: %w", s.name, err)
		}
	}
	return nil
}

// ensureSecret creates or updates a Secret. For keys where the desired value is empty,
// a random password or username is generated on first creation and preserved on subsequent reconciliations.
// Usernames are identified by keys containing "USER" or "USERNAME" and are generated with a prefix.
func (r *CloudActiveDefenseReconciler) ensureSecret(ctx context.Context, cad *operatorv1alpha1.CloudActiveDefense, name string, desiredData map[string]string) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cad.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, secret, func() error {
		if err := ctrl.SetControllerReference(cad, secret, r.Scheme); err != nil {
			return err
		}
		secret.Labels = util.LabelsForComponent(cad.Name, name)
		if secret.Data == nil {
			secret.Data = make(map[string][]byte)
		}
		for k, v := range desiredData {
			if v != "" {
				// User explicitly provided a value
				secret.Data[k] = []byte(v)
			} else if _, exists := secret.Data[k]; !exists {
				// Not provided and not yet generated: create a random value
				var generatedValue string
				var err error

				// Check if this is a username field
				if util.IsUsernameField(k) {
					generatedValue, err = util.GenerateUsername("")
				} else {
					// Generate password
					generatedValue, err = util.GeneratePassword(32)
				}

				if err != nil {
					return fmt.Errorf("generating value for %s: %w", k, err)
				}
				secret.Data[k] = []byte(generatedValue)
			}
			// If empty and already exists, preserve the existing value
		}
		return nil
	})
	return err
}

// --- PVC reconciliation ---

func (r *CloudActiveDefenseReconciler) reconcilePVCs(ctx context.Context, cad *operatorv1alpha1.CloudActiveDefense) error {
	pvcs := []struct {
		name      string
		component string
	}{
		{"controlpanel-pvc", "controlpanel-db"},
		{"keycloak-pvc", "keycloak-db"},
	}

	for _, p := range pvcs {
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      p.name,
				Namespace: cad.Namespace,
			},
		}
		_, err := controllerutil.CreateOrUpdate(ctx, r.Client, pvc, func() error {
			if err := ctrl.SetControllerReference(cad, pvc, r.Scheme); err != nil {
				return err
			}
			pvc.Labels = util.LabelsForComponent(cad.Name, p.component)
			// PVC spec is immutable after creation
			if pvc.CreationTimestamp.IsZero() {
				pvc.Spec = pvcSpec()
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("PVC %s: %w", p.name, err)
		}
	}
	return nil
}

// --- RBAC reconciliation ---

func (r *CloudActiveDefenseReconciler) reconcileRBAC(ctx context.Context, cad *operatorv1alpha1.CloudActiveDefense) error {
	// ServiceAccounts (namespace-scoped, owner references work)
	for _, saName := range []string{"deployment-manager-sa", "keycloak-sa"} {
		sa := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      saName,
				Namespace: cad.Namespace,
			},
		}
		_, err := controllerutil.CreateOrUpdate(ctx, r.Client, sa, func() error {
			return ctrl.SetControllerReference(cad, sa, r.Scheme)
		})
		if err != nil {
			return fmt.Errorf("ServiceAccount %s: %w", saName, err)
		}
	}

	// ClusterRoles (cluster-scoped, cleaned up via finalizer)
	clusterRoles := []struct {
		name  string
		rules func() []rbacv1.PolicyRule
	}{
		{util.ClusterResourceName(cad, "deployment-manager-cr"), deploymentManagerClusterRoleRules},
		{util.ClusterResourceName(cad, "keycloak-cr"), keycloakClusterRoleRules},
	}

	for _, cr := range clusterRoles {
		role := &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: cr.name},
		}
		_, err := controllerutil.CreateOrUpdate(ctx, r.Client, role, func() error {
			role.Labels = util.LabelsForComponent(cad.Name, cr.name)
			role.Rules = cr.rules()
			return nil
		})
		if err != nil {
			return fmt.Errorf("ClusterRole %s: %w", cr.name, err)
		}
	}

	// ClusterRoleBindings
	bindings := []struct {
		name     string
		roleName string
		saName   string
	}{
		{
			name:     util.ClusterResourceName(cad, "deployment-manager-crb"),
			roleName: util.ClusterResourceName(cad, "deployment-manager-cr"),
			saName:   "deployment-manager-sa",
		},
		{
			name:     util.ClusterResourceName(cad, "keycloak-crb"),
			roleName: util.ClusterResourceName(cad, "keycloak-cr"),
			saName:   "keycloak-sa",
		},
	}

	for _, b := range bindings {
		binding := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: b.name},
		}
		_, err := controllerutil.CreateOrUpdate(ctx, r.Client, binding, func() error {
			binding.Labels = util.LabelsForComponent(cad.Name, b.name)
			binding.Subjects = []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      b.saName,
					Namespace: cad.Namespace,
				},
			}
			binding.RoleRef = rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     b.roleName,
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("ClusterRoleBinding %s: %w", b.name, err)
		}
	}
	return nil
}

// --- Deployment reconciliation ---

func (r *CloudActiveDefenseReconciler) reconcileDeployments(ctx context.Context, cad *operatorv1alpha1.CloudActiveDefense) error {
	deployments := []struct {
		name string
		spec func(*operatorv1alpha1.CloudActiveDefense) appsv1.DeploymentSpec
	}{
		{"controlpanel-db", controlpanelDBDeploymentSpec},
		{"controlpanel-api", controlpanelAPIDeploymentSpec},
		{"controlpanel-front", controlpanelFrontDeploymentSpec},
		{"deployment-manager", deploymentManagerDeploymentSpec},
		{"keycloak-db", keycloakDBDeploymentSpec},
		{"keycloak", keycloakDeploymentSpec},
	}

	for _, d := range deployments {
		deploy := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      d.name,
				Namespace: cad.Namespace,
			},
		}
		_, err := controllerutil.CreateOrUpdate(ctx, r.Client, deploy, func() error {
			if err := ctrl.SetControllerReference(cad, deploy, r.Scheme); err != nil {
				return err
			}
			deploy.Labels = util.LabelsForComponent(cad.Name, d.name)
			deploy.Spec = d.spec(cad)
			return nil
		})
		if err != nil {
			return fmt.Errorf("deployment %s: %w", d.name, err)
		}
	}
	return nil
}

// --- Service reconciliation ---

func (r *CloudActiveDefenseReconciler) reconcileServices(ctx context.Context, cad *operatorv1alpha1.CloudActiveDefense) error {
	services := []struct {
		name      string
		component string
		spec      func(*operatorv1alpha1.CloudActiveDefense) corev1.ServiceSpec
	}{
		{"controlpanel-db-service", "controlpanel-db", controlpanelDBServiceSpec},
		{"controlpanel-api-service", "controlpanel-api", controlpanelAPIServiceSpec},
		{"controlpanel-front-service", "controlpanel-front", controlpanelFrontServiceSpec},
		{"deployment-manager-service", "deployment-manager", deploymentManagerServiceSpec},
		{"keycloak-db-service", "keycloak-db", keycloakDBServiceSpec},
		{"keycloak-service", "keycloak", keycloakServiceSpec},
	}

	for _, s := range services {
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      s.name,
				Namespace: cad.Namespace,
			},
		}
		_, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
			if err := ctrl.SetControllerReference(cad, svc, r.Scheme); err != nil {
				return err
			}
			svc.Labels = util.LabelsForComponent(cad.Name, s.component)
			svc.Spec = s.spec(cad)
			return nil
		})
		if err != nil {
			return fmt.Errorf("service %s: %w", s.name, err)
		}
	}
	return nil
}

// --- AuthorizationPolicy reconciliation ---

func (r *CloudActiveDefenseReconciler) reconcileAuthPolicies(ctx context.Context, cad *operatorv1alpha1.CloudActiveDefense) error {
	policies := []struct {
		name      string
		component string
		spec      func(*operatorv1alpha1.CloudActiveDefense) istiosecv1b1.AuthorizationPolicy
	}{
		{"keycloak-to-controlpanel-api", "controlpanel-api", keycloakToAPIAuthPolicySpec},
		{"telemetry-to-controlpanel-api", "controlpanel-api", telemetryToAPIAuthPolicySpec},
		{"wasm-to-controlpanel-api", "controlpanel-api", wasmToAPIAuthPolicySpec},
	}

	for _, p := range policies {
		ap := &istioclientv1.AuthorizationPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      p.name,
				Namespace: cad.Namespace,
			},
		}
		_, err := controllerutil.CreateOrUpdate(ctx, r.Client, ap, func() error {
			if err := ctrl.SetControllerReference(cad, ap, r.Scheme); err != nil {
				return err
			}
			ap.Labels = util.LabelsForComponent(cad.Name, p.component)
			ap.Spec = p.spec(cad)
			return nil
		})
		if err != nil {
			return fmt.Errorf("AuthorizationPolicy %s: %w", p.name, err)
		}
	}
	return nil
}

// --- APIRule reconciliation ---

func (r *CloudActiveDefenseReconciler) reconcileAPIRules(ctx context.Context, cad *operatorv1alpha1.CloudActiveDefense) error {
	apiRules := []struct {
		name      string
		component string
		spec      func(*operatorv1alpha1.CloudActiveDefense) kymagwv2.APIRuleSpec
	}{
		{"controlpanel-api-apirule", "controlpanel-api", controlpanelAPIAPIRuleSpec},
		{"controlpanel-front-apirule", "controlpanel-front", controlpanelFrontAPIRuleSpec},
		{"keycloak-apirule", "keycloak", keycloakAPIRuleSpec},
	}

	for _, a := range apiRules {
		apiRule := &kymagwv2.APIRule{
			ObjectMeta: metav1.ObjectMeta{
				Name:      a.name,
				Namespace: cad.Namespace,
			},
		}
		_, err := controllerutil.CreateOrUpdate(ctx, r.Client, apiRule, func() error {
			if err := ctrl.SetControllerReference(cad, apiRule, r.Scheme); err != nil {
				return err
			}
			apiRule.Labels = util.LabelsForComponent(cad.Name, a.component)
			apiRule.Spec = a.spec(cad)
			return nil
		})
		if err != nil {
			return fmt.Errorf("APIRule %s: %w", a.name, err)
		}
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CloudActiveDefenseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.CloudActiveDefense{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&kymagwv2.APIRule{}).
		Owns(&istioclientv1.AuthorizationPolicy{}).
		Named("cloudactivedefense").
		Complete(r)
}
