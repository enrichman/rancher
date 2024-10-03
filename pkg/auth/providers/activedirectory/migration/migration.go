package migration

import (
	"context"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/rancher/rancher/pkg/auth/providers"
	ad "github.com/rancher/rancher/pkg/auth/providers/activedirectory"
	"github.com/rancher/rancher/pkg/types/config"
)

type UserContext struct {
	PrincipalID string
	DN          string
	ObjectGUID  string

	User *v3.User
	// related bindings
	Tokens []*v3.Token
	CRTBs  []*v3.ClusterRoleTemplateBinding
	PRTBs  []*v3.ProjectRoleTemplateBinding
}

// Run will start the job to handle the migration
func Run(ctx context.Context, management *config.ManagementContext) {
	logrus.Info("[ActiveDirectory MIGRATION] Start")

	// check if the AD prvider is enabled
	provider, err := providers.GetProvider("activedirectory")
	if err != nil {
		panic(err)
	}

	// disabled, err := provider.IsDisabledProvider()
	// if err != nil || disabled {
	// 	panic(err)
	// }

	// Get the configuration
	configMapInterface := management.K8sClient.CoreV1().ConfigMaps("cattle-system")
	config, err := GetOrCreateConfig(ctx, configMapInterface)
	if err != nil {
		panic(err)
	}

	// if config is disabled, stop
	if !config.Enabled {
		return
	}

	// if migration is running, stop
	if config.Status == StatusRunning {
		return
	}

	usersInterface := management.Management.Users("")
	adUsers, err := GetActiveDirectoryUsers(usersInterface)
	if err != nil {
		panic(err)
	}

	// if users > 0 we need to get only those
	if len(config.Users) > 0 {
		usersSet := sets.NewString(config.Users...)

		filtered := []v3.User{}
		for _, adUser := range adUsers {
			if usersSet.Has(adUser.Name) {
				filtered = append(filtered, adUser)
			}
		}
		adUsers = filtered
	}

	if config.Limit > 0 {
		limit := min(config.Limit, len(adUsers))
		adUsers = adUsers[:limit]
	}

	userContextMap := map[string]UserContext{}

	for _, user := range adUsers {
		for _, pID := range user.PrincipalIDs {
			if strings.HasPrefix(pID, ad.UserScope+"://") {
				principal, err := provider.GetPrincipal(pID, v3.Token{})
				if err != nil {
					panic(err)
				}

				userContextMap[pID] = UserContext{
					User:        &user,
					PrincipalID: pID,
					DN:          principal.GetLabels()["dn"],
					ObjectGUID:  principal.GetLabels()["objectGUID"],
				}
			}
		}
	}

	prtbsMap, err := GetPRTBs(management.Management.ProjectRoleTemplateBindings(""))
	if err != nil {
		panic(err)
	}

	for principalID, prtbs := range prtbsMap {
		userCtx, found := userContextMap[principalID]
		if !found {
			// what??
		}
		userCtx.PRTBs = append(userCtx.PRTBs, prtbs...)
		userContextMap[principalID] = userCtx
	}

	// split

	adUsersGUID := map[string]UserContext{}
	adUsersDN := map[string]UserContext{}

	for pID, userCtx := range userContextMap {
		if strings.HasPrefix(pID, ad.UserScope+"://objectGUID") {
			adUsersGUID[pID] = userCtx
		} else {
			adUsersDN[pID] = userCtx
		}
	}

	logrus.Infof("[ActiveDirectory MIGRATION] Found %d users to migrate", len(adUsersDN))
	for _, userCtx := range adUsersDN {
		logrus.Infof("[ActiveDirectory MIGRATION] Migrating user %s: %s -> %s", userCtx.User.Name, userCtx.DN, userCtx.ObjectGUID)
		logrus.Infof("[ActiveDirectory MIGRATION] [user %s] Found %d PRTBs", userCtx.User.Name, len(userCtx.PRTBs))
	}

	logrus.Infof("[ActiveDirectory MIGRATION] Found %d users already migrated", len(adUsersGUID))

	// do this in migrate/rollback
	// check if a job is already running (only the check action can run)
}
