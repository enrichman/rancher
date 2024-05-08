package auth

import (
	"context"
	"encoding/json"
	"reflect"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/norman/condition"
	apisv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	wranglerv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	rrbacv1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	"github.com/rancher/rancher/pkg/systemaccount"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	v12 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	creatorIDAnn                  = "field.cattle.io/creatorId"
	creatorOwnerBindingAnnotation = "authz.management.cattle.io/creator-owner-binding"
	projectCreateController       = "mgmt-project-rbac-create"
	clusterCreateController       = "mgmt-cluster-rbac-delete" // TODO the word delete here is wrong, but changing it would break backwards compatibility
	projectRemoveController       = "mgmt-project-rbac-remove"
	clusterRemoveController       = "mgmt-cluster-rbac-remove"
	roleTemplatesRequired         = "authz.management.cattle.io/creator-role-bindings"
)

var defaultProjectLabels = labels.Set(map[string]string{"authz.management.cattle.io/default-project": "true"})
var systemProjectLabels = labels.Set(map[string]string{"authz.management.cattle.io/system-project": "true"})
var crtbCreatorOwnerAnnotations = map[string]string{creatorOwnerBindingAnnotation: "true"}

func newPandCLifecycles(management *config.ManagementContext) (*projectLifecycle, *clusterLifecycle) {
	m := &mgr{
		mgmt:                 management,
		nsLister:             management.Core.Namespaces("").Controller().Lister(),
		prtbLister:           management.Management.ProjectRoleTemplateBindings("").Controller().Lister(),
		crtbLister:           management.Management.ClusterRoleTemplateBindings("").Controller().Lister(),
		crtbClient:           management.Management.ClusterRoleTemplateBindings(""),
		projectLister:        management.Management.Projects("").Controller().Lister(),
		projects:             management.Wrangler.Mgmt.Project(),
		roleTemplateLister:   management.Management.RoleTemplates("").Controller().Lister(),
		systemAccountManager: systemaccount.NewManager(management),
		rbLister:             management.RBAC.RoleBindings("").Controller().Lister(),
		roleBindings:         management.RBAC.RoleBindings(""),
	}

	p := newProjectLifecycle(m)
	c := newClusterLifecycle(m)

	return p, c
}

type mgr struct {
	mgmt          *config.ManagementContext
	nsLister      corev1.NamespaceLister
	projectLister v3.ProjectLister
	projects      wranglerv3.ProjectClient

	prtbLister           v3.ProjectRoleTemplateBindingLister
	crtbLister           v3.ClusterRoleTemplateBindingLister
	crtbClient           v3.ClusterRoleTemplateBindingInterface
	roleTemplateLister   v3.RoleTemplateLister
	clusterRoleClient    rrbacv1.ClusterRoleInterface
	systemAccountManager *systemaccount.Manager
	rbLister             rrbacv1.RoleBindingLister
	roleBindings         rrbacv1.RoleBindingInterface
}

func (m *mgr) reconcileCreatorRTB(obj runtime.Object) (runtime.Object, error) {
	return v32.CreatorMadeOwner.DoUntilTrue(obj, func() (runtime.Object, error) {
		metaAccessor, err := meta.Accessor(obj)
		if err != nil {
			return obj, err
		}

		typeAccessor, err := meta.TypeAccessor(obj)
		if err != nil {
			return obj, err
		}

		creatorID, ok := metaAccessor.GetAnnotations()[creatorIDAnn]
		if !ok || creatorID == "" {
			logrus.Warnf("%v %v has no creatorId annotation. Cannot add creator as owner", typeAccessor.GetKind(), metaAccessor.GetName())
			return obj, nil
		}

		switch typeAccessor.GetKind() {
		case v3.ProjectGroupVersionKind.Kind:
			project := obj.(*apisv3.Project)

			if v32.ProjectConditionInitialRolesPopulated.IsTrue(project) {
				// The projectRoleBindings are already completed, no need to check
				break
			}

			// If the project does not have the annotation it indicates the
			// project is from a previous rancher version so don't add the
			// default bindings.
			roleJSON, ok := project.Annotations[roleTemplatesRequired]
			if !ok {
				return project, nil
			}

			roleMap := make(map[string][]string)
			err = json.Unmarshal([]byte(roleJSON), &roleMap)
			if err != nil {
				return obj, err
			}

			var createdRoles []string

			for _, role := range roleMap["required"] {
				rtbName := "creator-" + role

				if rtb, _ := m.prtbLister.Get(metaAccessor.GetName(), rtbName); rtb != nil {
					createdRoles = append(createdRoles, role)
					// This projectRoleBinding exists, need to check all of them so keep going
					continue
				}

				// The projectRoleBinding doesn't exist yet so create it
				om := v1.ObjectMeta{
					Name:      rtbName,
					Namespace: metaAccessor.GetName(),
				}

				logrus.Infof("[%v] Creating creator projectRoleTemplateBinding for user %v for project %v", projectCreateController, creatorID, metaAccessor.GetName())
				if _, err := m.mgmt.Management.ProjectRoleTemplateBindings(metaAccessor.GetName()).Create(&apisv3.ProjectRoleTemplateBinding{
					ObjectMeta:       om,
					ProjectName:      metaAccessor.GetNamespace() + ":" + metaAccessor.GetName(),
					RoleTemplateName: role,
					UserName:         creatorID,
				}); err != nil && !apierrors.IsAlreadyExists(err) {
					return obj, err
				}
				createdRoles = append(createdRoles, role)
			}

			project = project.DeepCopy()

			roleMap["created"] = createdRoles
			d, err := json.Marshal(roleMap)
			if err != nil {
				return obj, err
			}

			project.Annotations[roleTemplatesRequired] = string(d)

			if reflect.DeepEqual(roleMap["required"], createdRoles) {
				v32.ProjectConditionInitialRolesPopulated.True(project)
				logrus.Infof("[%v] Setting InitialRolesPopulated condition on project %v", ctrbMGMTController, project.Name)
			}
			if _, err := m.mgmt.Management.Projects("").Update(project); err != nil {
				return obj, err
			}

		case v3.ClusterGroupVersionKind.Kind:
			cluster := obj.(*apisv3.Cluster)

			if v32.ClusterConditionInitialRolesPopulated.IsTrue(cluster) {
				// The clusterRoleBindings are already completed, no need to check
				break
			}

			roleJSON, ok := cluster.Annotations[roleTemplatesRequired]
			if !ok {
				return cluster, nil
			}

			roleMap := make(map[string][]string)
			err = json.Unmarshal([]byte(roleJSON), &roleMap)
			if err != nil {
				return obj, err
			}

			var createdRoles []string

			for _, role := range roleMap["required"] {
				rtbName := "creator-" + role

				if rtb, _ := m.crtbLister.Get(metaAccessor.GetName(), rtbName); rtb != nil {
					createdRoles = append(createdRoles, role)
					// This clusterRoleBinding exists, need to check all of them so keep going
					continue
				}

				// The clusterRoleBinding doesn't exist yet so create it
				om := v1.ObjectMeta{
					Name:      rtbName,
					Namespace: metaAccessor.GetName(),
				}
				om.Annotations = crtbCreatorOwnerAnnotations

				logrus.Infof("[%v] Creating creator clusterRoleTemplateBinding for user %v for cluster %v", projectCreateController, creatorID, metaAccessor.GetName())
				if _, err := m.mgmt.Management.ClusterRoleTemplateBindings(metaAccessor.GetName()).Create(&apisv3.ClusterRoleTemplateBinding{
					ObjectMeta:       om,
					ClusterName:      metaAccessor.GetName(),
					RoleTemplateName: role,
					UserName:         creatorID,
				}); err != nil && !apierrors.IsAlreadyExists(err) {
					return obj, err
				}
				createdRoles = append(createdRoles, role)
			}

			roleMap["created"] = createdRoles
			d, err := json.Marshal(roleMap)
			if err != nil {
				return obj, err
			}

			updateCondition := reflect.DeepEqual(roleMap["required"], createdRoles)

			err = m.updateClusterAnnotationandCondition(cluster, string(d), updateCondition)
			if err != nil {
				return obj, err
			}
		}

		return obj, nil
	})
}

func (m *mgr) deleteNamespace(obj runtime.Object, controller string) error {
	o, err := meta.Accessor(obj)
	if err != nil {
		return condition.Error("MissingMetadata", err)
	}

	nsClient := m.mgmt.K8sClient.CoreV1().Namespaces()
	ns, err := nsClient.Get(context.TODO(), o.GetName(), v1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if ns.Status.Phase != v12.NamespaceTerminating {
		logrus.Infof("[%s] Deleting namespace %s", controller, o.GetName())
		err = nsClient.Delete(context.TODO(), o.GetName(), v1.DeleteOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
	}
	return err
}

func (m *mgr) reconcileResourceToNamespace(obj runtime.Object, controller string) (runtime.Object, error) {
	return v32.NamespaceBackedResource.Do(obj, func() (runtime.Object, error) {
		o, err := meta.Accessor(obj)
		if err != nil {
			return obj, condition.Error("MissingMetadata", err)
		}
		t, err := meta.TypeAccessor(obj)
		if err != nil {
			return obj, condition.Error("MissingTypeMetadata", err)
		}

		ns, _ := m.nsLister.Get("", o.GetName())
		if ns == nil {
			nsClient := m.mgmt.K8sClient.CoreV1().Namespaces()
			logrus.Infof("[%v] Creating namespace %v", controller, o.GetName())
			_, err := nsClient.Create(context.TODO(), &v12.Namespace{
				ObjectMeta: v1.ObjectMeta{
					Name: o.GetName(),
					Annotations: map[string]string{
						"management.cattle.io/system-namespace": "true",
					},
				},
			}, v1.CreateOptions{})
			if err != nil {
				return obj, condition.Error("NamespaceCreationFailure", errors.Wrapf(err, "failed to create namespace for %v %v", t.GetKind(), o.GetName()))
			}
		}

		return obj, nil
	})
}

func (m *mgr) updateClusterAnnotationandCondition(cluster *apisv3.Cluster, anno string, updateCondition bool) error {
	sleep := 100
	for i := 0; i <= 3; i++ {
		c, err := m.mgmt.Management.Clusters("").Get(cluster.Name, v1.GetOptions{})
		if err != nil {
			return err
		}

		c.Annotations[roleTemplatesRequired] = anno

		if updateCondition {
			v32.ClusterConditionInitialRolesPopulated.True(c)
		}
		_, err = m.mgmt.Management.Clusters("").Update(c)
		if err != nil {
			if apierrors.IsConflict(err) {
				time.Sleep(time.Duration(sleep) * time.Millisecond)
				sleep *= 2
				continue
			}
			return err
		}
		// Only log if we successfully updated the cluster
		if updateCondition {
			logrus.Infof("[%v] Setting InitialRolesPopulated condition on cluster %v", ctrbMGMTController, cluster.Name)
		}
		return nil
	}
	return nil
}
