package migration

import (
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	ad "github.com/rancher/rancher/pkg/auth/providers/activedirectory"
	mv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
