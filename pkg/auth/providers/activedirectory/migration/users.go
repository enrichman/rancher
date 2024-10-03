package migration

import (
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	ad "github.com/rancher/rancher/pkg/auth/providers/activedirectory"
	gen "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetActiveDirectoryUsers(userInterface gen.UserInterface) ([]v3.User, error) {
	var users []v3.User

	allUsers, err := userInterface.List(v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, user := range allUsers.Items {
		for _, pID := range user.PrincipalIDs {
			if strings.HasPrefix(pID, ad.UserScope+"://") {
				users = append(users, user)
			}
		}
	}

	return users, nil
}
