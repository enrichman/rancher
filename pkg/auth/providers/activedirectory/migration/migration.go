package migration

import (
	"context"
	"fmt"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	mv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"

	"github.com/rancher/rancher/pkg/auth/providers"
	ad "github.com/rancher/rancher/pkg/auth/providers/activedirectory"
	"github.com/rancher/rancher/pkg/types/config"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	// check if the AD prvider is enabled
	provider, err := providers.GetProvider("activedirectory")
	if err != nil {
		panic(err)
	}

	// disabled, err := provider.IsDisabledProvider()
	// if err != nil || disabled {
	// 	panic(err)
	// }

	// TODO check if a configuration exists

	allUsers, err := management.Management.Users("").List(v1.ListOptions{})
	if err != nil {
		panic(err)
	}

	adUsers := map[string]UserContext{}

	for _, user := range allUsers.Items {
		for _, pID := range user.PrincipalIDs {
			if strings.HasPrefix(pID, "activedirectory_user://") {
				principal, err := provider.GetPrincipal(pID, v3.Token{})
				if err != nil {
					panic(err)
				}

				adUsers[pID] = UserContext{
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
		userCtx, found := adUsers[principalID]
		if !found {
			// what??
		}
		userCtx.PRTBs = append(userCtx.PRTBs, prtbs...)
		adUsers[principalID] = userCtx
	}

	fmt.Println("done")

	// do this in migrate/rollback
	// check if a job is already running (only the check action can run)
}

// principal to PRTBs map
func GetPRTBs(prtbInterface mv3.ProjectRoleTemplateBindingInterface) (map[string][]*v3.ProjectRoleTemplateBinding, error) {
	prtbsMap := make(map[string][]*v3.ProjectRoleTemplateBinding)

	prtbs, err := prtbInterface.List(v1.ListOptions{})
	if err != nil {
		panic(err)
	}

	for _, prtb := range prtbs.Items {
		if strings.HasPrefix(prtb.UserPrincipalName, ad.UserScope+"://") {
			bindings, found := prtbsMap[prtb.UserPrincipalName]
			if !found {
				bindings = []*v3.ProjectRoleTemplateBinding{}
			}

			prtbsMap[prtb.UserPrincipalName] = append(bindings, &prtb)
		}
	}

	return prtbsMap, nil
}
