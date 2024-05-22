package ldapuuid

import (
	"context"
	"encoding/base32"
	"fmt"
	"html"
	"os"
	"slices"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	ldapv3 "github.com/go-ldap/ldap/v3"
	"github.com/rancher/norman/httperror"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	v3client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	restclient "k8s.io/client-go/rest"

	"k8s.io/client-go/tools/clientcmd"
)

const migrateAdUserOperation = "XXX"

func Run(clientConfig *restclient.Config, dryRun bool) error {
	if dryRun {
		logrus.Infof("[%v] dryRun is true, no objects will be deleted/modified", migrateAdUserOperation)
	}

	sc, _, err := prepareClientContexts(clientConfig)
	if err != nil {
		return err
	}

	users, err := sc.Management.Users("").List(metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("[%v] unable to fetch user list: %v", migrateAdUserOperation, err)
		return err
	}

	//lConn := sharedLdapConnection{adConfig: adConfig}

	usersToMigrate := []v3.User{}
	for i, user := range users.Items {
		logrus.Debugf("[%v] user %d %v", migrateAdUserOperation, i, user.Name)
		for _, principalID := range user.PrincipalIDs {
			if strings.HasPrefix(principalID, "openldap_user") {
				usersToMigrate = append(usersToMigrate, user)
			}
		}
	}

	for i, user := range usersToMigrate {
		logrus.Debugf("[%v] user %d %v", migrateAdUserOperation, i, user.Name)

		var guid, dn string

		for _, principalID := range user.PrincipalIDs {
			if strings.HasPrefix(principalID, "openldap_user://") {
				principalID := strings.TrimPrefix(principalID, "openldap_user://")

				// TODO handle guid and entryUUID
				if strings.Contains(principalID, "entryUUID=") {
					guid = strings.Replace(principalID, "entryUUID=", "", -1)
				} else {
					dn = principalID
				}
			}
		}

		if guid == "" {
			// todo if dn empty fail/skip
			guid, err = getLdapUser(dn)
			if err != nil {
				return err
			}
		}

		// add label to users with same dn hash

		principalName := "openldap_user://" + dn
		encodedPrincipalID := base32.HexEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte(principalName))
		if len(encodedPrincipalID) > 63 {
			encodedPrincipalID = encodedPrincipalID[:63]
		}
		set := labels.Set(map[string]string{encodedPrincipalID: "hashed-principal-name"})
		users, err := sc.Management.Users("").List(v1.ListOptions{LabelSelector: set.String()})
		if err != nil {
			logrus.Errorf("[%v] unable to fetch user list: %v", migrateAdUserOperation, err)
			return err
		}

		for _, u := range users.Items {
			principalName = "openldap_user://entryUUID=" + guid
			encodedPrincipalID := base32.HexEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte(principalName))
			if len(encodedPrincipalID) > 63 {
				encodedPrincipalID = encodedPrincipalID[:63]
			}
			u.Labels[encodedPrincipalID] = "hashed-principal-name"

			if !slices.Contains(u.PrincipalIDs, principalName) {
				u.PrincipalIDs = append(u.PrincipalIDs, principalName)
			}

			userUpdated, err := sc.Management.Users("").Update(&u)
			if err != nil {
				logrus.Errorf("[%v] unable to userUpdated user list: %v", migrateAdUserOperation, err)
				return err
			}
			fmt.Println(userUpdated.Labels)
		}
	}

	return nil
}

// prepareClientContexts sets up a scaled context with the ability to read users and AD configuration data
func prepareClientContexts(clientConfig *restclient.Config) (*config.ScaledContext, *v3.ActiveDirectoryConfig, error) {
	var restConfig *restclient.Config
	var err error
	if clientConfig != nil {
		restConfig = clientConfig
	} else {
		restConfig, err = clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
		if err != nil {
			logrus.Errorf("[%v] failed to build the cluster config: %v", migrateAdUserOperation, err)
			return nil, nil, err
		}
	}

	sc, err := scaledContext(restConfig)
	if err != nil {
		logrus.Errorf("[%v] failed to create scaled context: %v", migrateAdUserOperation, err)
		return nil, nil, err
	}
	adConfig, err := adConfiguration(sc)
	if err != nil {
		logrus.Errorf("[%v] failed to acquire ad configuration: %v", migrateAdUserOperation, err)
		return nil, nil, err
	}

	return sc, adConfig, nil
}

func scaledContext(restConfig *restclient.Config) (*config.ScaledContext, error) {
	sc, err := config.NewScaledContext(*restConfig, nil)
	if err != nil {
		logrus.Errorf("[%v] failed to create scaledContext: %v", migrateAdUserOperation, err)
		return nil, err
	}

	ctx := context.Background()
	err = sc.Start(ctx)
	if err != nil {
		logrus.Errorf("[%v] failed to start scaled context: %v", migrateAdUserOperation, err)
		return nil, err
	}

	return sc, nil
}

func adConfiguration(sc *config.ScaledContext) (*v3.ActiveDirectoryConfig, error) {
	authConfigs := sc.Management.AuthConfigs("")
	secrets := sc.Core.Secrets("")

	authConfigObj, err := authConfigs.ObjectClient().UnstructuredClient().Get("activedirectory", metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("[%v] failed to obtain activedirectory authConfigObj: %v", migrateAdUserOperation, err)
		return nil, err
	}

	u, ok := authConfigObj.(runtime.Unstructured)
	if !ok {
		logrus.Errorf("[%v] failed to retrieve ActiveDirectoryConfig, cannot read k8s Unstructured data %v", migrateAdUserOperation, err)
		return nil, err
	}
	storedADConfigMap := u.UnstructuredContent()

	storedADConfig := &v3.ActiveDirectoryConfig{}
	err = common.Decode(storedADConfigMap, storedADConfig)
	if err != nil {
		logrus.Errorf("[%v] errors while decoding stored AD config: %v", migrateAdUserOperation, err)
		return nil, err
	}

	metadataMap, ok := storedADConfigMap["metadata"].(map[string]interface{})
	if !ok {
		logrus.Errorf("[%v] failed to retrieve ActiveDirectoryConfig, (second step), cannot read k8s Unstructured data %v", migrateAdUserOperation, err)
		return nil, err
	}

	typemeta := &metav1.ObjectMeta{}
	err = common.Decode(metadataMap, typemeta)
	if err != nil {
		logrus.Errorf("[%v] errors while decoding typemeta: %v", migrateAdUserOperation, err)
		return nil, err
	}

	storedADConfig.ObjectMeta = *typemeta

	if storedADConfig.ServiceAccountPassword != "" {
		value, err := common.ReadFromSecret(secrets, storedADConfig.ServiceAccountPassword,
			strings.ToLower(v3client.ActiveDirectoryConfigFieldServiceAccountPassword))
		if err != nil {
			return nil, err
		}
		storedADConfig.ServiceAccountPassword = value
	}

	return storedADConfig, nil
}

func getLdapUser(dn string) (string, error) {
	server := "172.20.0.2"
	port := 389
	//userSearchBase := "ou=users,dc=example,dc=org"
	objectClass := "objectClass"
	userObjectClass := "inetOrgPerson"
	userLoginAttribute := "uid"
	userNameAttribute := "cn"
	userMemberAttribute := "memberOf"

	lConn, err := ldapv3.Dial("tcp", fmt.Sprintf("%s:%d", server, port))
	if err != nil {
		return "", fmt.Errorf("Error creating connection: %v", err)
	}
	defer lConn.Close()

	err = lConn.Bind("cn=admin,dc=example,dc=org", "admin")
	if err != nil {
		return "", fmt.Errorf("Error binding service account user connection: %v", err)
	}

	// search by cn/login name
	// filter := fmt.Sprintf(
	// 	"(&(%s=%v)(%v=%v))",
	// 	objectClass,
	// 	userObjectClass,
	// 	userLoginAttribute,
	// 	ldapv3.EscapeFilter("enrico"),
	// )

	// searchRequest := ldapv3.NewSearchRequest(
	// 	userSearchBase,
	// 	ldapv3.ScopeWholeSubtree,
	// 	ldapv3.NeverDerefAliases,
	// 	0, 0, false,
	// 	filter,
	// 	[]string{
	// 		"dn", objectClass, "objectGUID", "entryUUID", userMemberAttribute,
	// 		userObjectClass, userLoginAttribute, userNameAttribute, userLoginAttribute,
	// 	},
	// 	nil,
	// )

	// result, err := lConn.Search(searchRequest)
	// if err != nil {
	// 	return httperror.WrapAPIError(err, httperror.Unauthorized, "authentication failed") // need to reload this error
	// }

	// DN search
	// err = lConn.Bind("cn=admin,dc=example,dc=org", "admin")
	// if err != nil {
	// 	return fmt.Errorf("Error binding service account user connection: %v", err)
	// }

	searchRequest := ldapv3.NewSearchRequest(
		dn,
		//"cn=test,ou=emea,ou=users,dc=example,dc=org",
		ldapv3.ScopeBaseObject,
		ldapv3.NeverDerefAliases,
		0, 0, false,
		fmt.Sprintf("(%v=%v)", objectClass, userObjectClass),
		[]string{
			"dn", objectClass, "objectGUID", "entryUUID", userMemberAttribute,
			userObjectClass, userLoginAttribute, userNameAttribute, userLoginAttribute,
		},
		nil,
	)

	result, err := lConn.Search(searchRequest)
	if err != nil {
		return "", httperror.WrapAPIError(err, httperror.Unauthorized, "authentication failed") // need to reload this error
	}

	if len(result.Entries) < 1 {
		return "", httperror.WrapAPIError(err, httperror.Unauthorized, "Cannot locate user information for "+searchRequest.Filter)
	} else if len(result.Entries) > 1 {
		return "", fmt.Errorf("ldap user search found more than one result")
	}

	entry := result.Entries[0]
	guidString := html.EscapeString(fmt.Sprintf("%x", entry.GetRawAttributeValue("objectGUID")))
	uuidString := string(entry.GetRawAttributeValue("entryUUID"))
	uuidEscaped := html.EscapeString(fmt.Sprintf("%x", entry.GetRawAttributeValue("entryUUID")))
	fmt.Println(guidString, uuidString, uuidEscaped)

	return string(entry.GetRawAttributeValue("entryUUID")), nil
}
