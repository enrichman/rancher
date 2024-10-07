package migration

import (
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	ad "github.com/rancher/rancher/pkg/auth/providers/activedirectory"
	mv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// principal to CRTBs map
func GetCRTBs(crtbInterface mv3.ClusterRoleTemplateBindingInterface) (map[string][]*v3.ClusterRoleTemplateBinding, error) {
	crtbsMap := make(map[string][]*v3.ClusterRoleTemplateBinding)

	crtbs, err := crtbInterface.List(v1.ListOptions{})
	if err != nil {
		panic(err)
	}

	for _, crtb := range crtbs.Items {
		if strings.HasPrefix(crtb.UserPrincipalName, ad.UserScope+"://") {
			bindings, found := crtbsMap[crtb.UserPrincipalName]
			if !found {
				bindings = []*v3.ClusterRoleTemplateBinding{}
			}

			crtbsMap[crtb.UserPrincipalName] = append(bindings, &crtb)
		}
	}

	return crtbsMap, nil
}

func UpdateCRTBPrincipal(crtbInterface mv3.ClusterRoleTemplateBindingInterface, crtb *v3.ClusterRoleTemplateBinding, principalID string) (*v3.ClusterRoleTemplateBinding, error) {
	// generate a new CRTB
	oldCRTBName := crtb.Name
	crtb.UserPrincipalName = principalID
	crtb.Name = ""
	crtb.ResourceVersion = ""

	newCRTB, err := crtbInterface.Create(crtb)
	if err != nil {
		return nil, err
	}

	if err := crtbInterface.Delete(oldCRTBName, &v1.DeleteOptions{}); err != nil {
		return nil, err
	}

	return newCRTB, nil
}
