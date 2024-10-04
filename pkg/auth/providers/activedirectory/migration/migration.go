package migration

import (
	"context"
	"fmt"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/maps"
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

	// TODO handle UserAttributes
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
		logrus.Info("[ActiveDirectory MIGRATION] Migration is disabled. Stop.")
		return
	}

	// if migration is running, stop
	if config.Status == StatusRunning {
		logrus.Info("[ActiveDirectory MIGRATION] Migration already running. Stop.")
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
					DN:          principal.ExtraInfo["dn"],
					ObjectGUID:  principal.ExtraInfo[ad.ObjectGUIDAttribute],
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

	// if we are running a check we can then simply return
	if config.Action == ActionCheck {
		Check(maps.Values(adUsersDN), maps.Values(adUsersGUID))
		return
	}

	// set the ConfigMap as running

	switch config.Action {
	case ActionMigrate:
		err = Migrate(management, maps.Values(adUsersDN))
	case ActionRollback:
		err = Rollback(management, maps.Values(adUsersGUID))
	}

	// set the ConfigMap as not running

	if err != nil {
		panic(err)
	}
}

func Check(usersDNCtx []UserContext, usersGUIDCtx []UserContext) {

	logrus.Infof("[ActiveDirectory MIGRATION] Found %d users to migrate", len(usersDNCtx))
	logrus.Infof("[ActiveDirectory MIGRATION] Found %d users already migrated", len(usersGUIDCtx))

	for _, userCtx := range usersDNCtx {
		logrus.Infof("[ActiveDirectory MIGRATION] Migrating user %s: %s -> %s", userCtx.User.Name, userCtx.DN, userCtx.ObjectGUID)
		logrus.Infof("[ActiveDirectory MIGRATION] [user %s] Found %d PRTBs", userCtx.User.Name, len(userCtx.PRTBs))
	}
}

func Migrate(management *config.ManagementContext, usersCtx []UserContext) error {
	for _, userCtx := range usersCtx {
		user := userCtx.User
		for i, pID := range user.PrincipalIDs {
			if strings.HasPrefix(pID, ad.UserScope+"://") {
				user.PrincipalIDs[i] = fmt.Sprintf("%s://%s=%s", ad.UserScope, ad.ObjectGUIDAttribute, userCtx.ObjectGUID)
			}
		}
		updatedUser, err := management.Management.Users("").Update(user)
		if err != nil {
			return err
		}
		fmt.Println(updatedUser)
	}
	return nil
}

func Rollback(management *config.ManagementContext, usersCtx []UserContext) error {
	for _, userCtx := range usersCtx {
		user := userCtx.User
		for i, pID := range user.PrincipalIDs {
			if strings.HasPrefix(pID, ad.UserScope+"://") {
				user.PrincipalIDs[i] = fmt.Sprintf("%s://%s", ad.UserScope, userCtx.DN)
			}
		}
		updatedUser, err := management.Management.Users("").Update(user)
		if err != nil {
			return err
		}
		fmt.Println(updatedUser)
	}
	return nil
}
