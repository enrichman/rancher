package ldapuuid

import (
	"context"
	"crypto/x509"
	"encoding/base32"
	"fmt"
	"html"
	"os"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	ldapv3 "github.com/go-ldap/ldap/v3"
	"github.com/rancher/norman/httperror"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/common/ldap"
	v3client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"

	restclient "k8s.io/client-go/rest"

	"k8s.io/client-go/tools/clientcmd"
)

const migrateAdUserOperation = "XXX"

func Run(clientConfig *restclient.Config, dryRun bool) error {
	var err error

	if dryRun {
		logrus.Infof("[%v] dryRun is true, no objects will be deleted/modified", migrateAdUserOperation)
	}

	if clientConfig == nil {
		clientConfig, err = clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
		if err != nil {
			logrus.Errorf("[%v] failed to build the cluster config: %v", migrateAdUserOperation, err)
			return err
		}
	}

	sc, err := newScaledContext(clientConfig)
	if err != nil {
		return err
	}

	adConfig, err := getADConfiguration(sc)
	if err != nil {
		return err
	}
	openLdapConfig, err := getOpenLDAPConfiguration(sc)
	if err != nil {
		return err
	}
	fmt.Println(adConfig, openLdapConfig)

	users, err := sc.Management.Users("").List(metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("[%v] unable to fetch user list: %v", migrateAdUserOperation, err)
		return err
	}
	usersToExtend := filterUsers(users.Items, "openldap")

	lConn, err := newLdapConnFromLdap(openLdapConfig)
	if err != nil {
		logrus.Errorf("[%v] unable to fetch user list: %v", migrateAdUserOperation, err)
		return err
	}

	for i, user := range usersToExtend {
		logrus.Debugf("[%v] user %d %v", migrateAdUserOperation, i, user.Name)

		if user.UUID == "" {
			// todo if dn empty fail/skip
			uuid, err := getUserUUID(lConn, user.DN)
			if err != nil {
				return err
			}
			usersToExtend[i].UUID = uuid
		}

		// add label to users with same dn hash
		encodedPrincipalID := encodeBase32(fmt.Sprintf("%s_user://%s", "openldap", user.DN))
		set := labels.Set(map[string]string{encodedPrincipalID: "hashed-principal-name"})
		users, err := sc.Management.Users("").List(v1.ListOptions{LabelSelector: set.String()})
		if err != nil {
			logrus.Errorf("[%v] unable to fetch user list: %v", migrateAdUserOperation, err)
			return err
		}

		for _, u := range users.Items {
			encodedPrincipalID := encodeBase32(fmt.Sprintf("%s_user://entryUUID=%s", "openldap", user.UUID))

			// if not found, update
			if _, found := u.Labels[encodedPrincipalID]; !found {
				u.Labels[encodedPrincipalID] = "hashed-principal-name"

				userUpdated, err := sc.Management.Users("").Update(&u)
				if err != nil {
					logrus.Errorf("[%v] unable to userUpdated user list: %v", migrateAdUserOperation, err)
					return err
				}
				fmt.Println(userUpdated.Labels)
			}
		}
	}

	return nil
}

func newScaledContext(restConfig *restclient.Config) (*config.ScaledContext, error) {
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

func getADConfiguration(sc *config.ScaledContext) (*v3.ActiveDirectoryConfig, error) {
	adConfig, err := getAuthConfig(sc, "activedirectory", &v3.ActiveDirectoryConfig{})
	if err != nil {
		return nil, err
	}

	if adConfig.ServiceAccountPassword != "" {
		secrets := sc.Core.Secrets("")
		secretField := strings.ToLower(v3client.ActiveDirectoryConfigFieldServiceAccountPassword)
		value, err := common.ReadFromSecret(secrets, adConfig.ServiceAccountPassword, secretField)
		if err != nil {
			return nil, err
		}
		adConfig.ServiceAccountPassword = value
	}

	return adConfig, nil
}

func getOpenLDAPConfiguration(sc *config.ScaledContext) (*v3.OpenLdapConfig, error) {
	openLDAPConfig, err := getAuthConfig(sc, "openldap", &v3.OpenLdapConfig{})
	if err != nil {
		return nil, err
	}

	if openLDAPConfig.ServiceAccountPassword != "" {
		secrets := sc.Core.Secrets("")
		secretField := strings.ToLower(v3client.OpenLdapConfigFieldServiceAccountPassword)
		value, err := common.ReadFromSecret(secrets, openLDAPConfig.ServiceAccountPassword, secretField)
		if err != nil {
			return nil, err
		}
		openLDAPConfig.ServiceAccountPassword = value
	}

	return openLDAPConfig, nil
}

func getAuthConfig[T any](sc *config.ScaledContext, providerName string, providerConfig T) (T, error) {
	var zeroValue T
	authConfigs := sc.Management.AuthConfigs("")

	authConfigObj, err := authConfigs.ObjectClient().UnstructuredClient().Get(providerName, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("[%v] failed to obtain %s authConfigObj: %v", migrateAdUserOperation, providerName, err)
		return zeroValue, err
	}

	u, ok := authConfigObj.(runtime.Unstructured)
	if !ok {
		logrus.Errorf("[%v] failed to retrieve %s, cannot read k8s Unstructured data %v", migrateAdUserOperation, providerName, err)
		return zeroValue, err
	}
	configMap := u.UnstructuredContent()

	err = common.Decode(configMap, providerConfig)
	if err != nil {
		logrus.Errorf("[%v] errors while decoding stored %s config: %v", migrateAdUserOperation, providerName, err)
		return zeroValue, err
	}

	return providerConfig, nil
}

func newLdapConnFromAD(config *v3.ActiveDirectoryConfig) (*ldapv3.Conn, error) {
	servers := config.Servers
	TLS := config.TLS
	port := config.Port
	connectionTimeout := config.ConnectionTimeout
	startTLS := config.StartTLS

	caPool, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("unable to create caPool: %v", err)
	}
	caPool.AppendCertsFromPEM([]byte(config.Certificate))

	ldapConn, err := ldap.NewLDAPConn(servers, TLS, startTLS, port, connectionTimeout, caPool)
	if err != nil {
		return nil, err
	}

	serviceAccountUsername := ldap.GetUserExternalID(config.ServiceAccountUsername, config.DefaultLoginDomain)
	err = ldapConn.Bind(serviceAccountUsername, config.ServiceAccountPassword)
	if err != nil {
		return nil, err
	}
	return ldapConn, nil
}

func newLdapConnFromLdap(config *v3.OpenLdapConfig) (*ldapv3.Conn, error) {
	servers := config.Servers
	TLS := config.TLS
	port := config.Port
	connectionTimeout := config.ConnectionTimeout
	startTLS := config.StartTLS

	caPool, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("unable to create caPool: %v", err)
	}
	caPool.AppendCertsFromPEM([]byte(config.Certificate))

	ldapConn, err := ldap.NewLDAPConn(servers, TLS, startTLS, port, connectionTimeout, caPool)
	if err != nil {
		return nil, err
	}

	serviceAccountPassword := config.ServiceAccountPassword
	serviceAccountUserName := config.ServiceAccountDistinguishedName

	err = ldap.AuthenticateServiceAccountUser(serviceAccountPassword, serviceAccountUserName, "", ldapConn)
	if err != nil {
		return nil, err
	}
	return ldapConn, nil
}

type ldapSearchConfig struct {
	userObjectClass     string
	userLoginAttribute  string
	userNameAttribute   string
	userMemberAttribute string
}

func getUserUUID(lConn *ldapv3.Conn, dn string) (string, error) {
	// server := "172.20.0.2"
	// port := 389
	//userSearchBase := "ou=users,dc=example,dc=org"
	objectClass := "objectClass"
	userObjectClass := "inetOrgPerson"
	userLoginAttribute := "uid"
	userNameAttribute := "cn"
	userMemberAttribute := "memberOf"

	// lConn, err := ldapv3.Dial("tcp", fmt.Sprintf("%s:%d", server, port))
	// if err != nil {
	// 	return "", fmt.Errorf("Error creating connection: %v", err)
	// }
	// defer lConn.Close()

	// err = lConn.Bind("cn=admin,dc=example,dc=org", "admin")
	// if err != nil {
	// 	return "", fmt.Errorf("Error binding service account user connection: %v", err)
	// }

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

	// query := fmt.Sprintf("(&(%v=%v)(%v=%v))", AttributeObjectClass, adConfig.UserObjectClass, AttributeObjectGUID, escapeUUID(guid))
	// search := ldapv3.NewSearchRequest(adConfig.UserSearchBase, ldapv3.ScopeWholeSubtree, ldapv3.NeverDerefAliases,
	// 	0, 0, false,
	// 	query, ldap.GetUserSearchAttributes("memberOf", "objectClass", adConfig), nil)

	searchRequest := ldapv3.NewSearchRequest(
		dn,
		//"cn=test,ou=emea,ou=users,dc=example,dc=org",
		ldapv3.ScopeBaseObject,
		ldapv3.NeverDerefAliases,
		0, 0, false,
		fmt.Sprintf("(objectClass=%v)", userObjectClass),
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

type ExtendedUser struct {
	Name string
	UUID string
	DN   string
}

// check if has UUID in labels
func filterUsers(users []v3.User, provider string) []ExtendedUser {
	usersToExtend := []ExtendedUser{}
	providerScope := fmt.Sprintf("%s_user://", provider)

	for i, user := range users {
		logrus.Debugf("[%v] user %d %v", migrateAdUserOperation, i, user.Name)

		var guid, dn string

		for _, principalID := range user.PrincipalIDs {
			if strings.HasPrefix(principalID, providerScope) {
				principalID = strings.TrimPrefix(principalID, providerScope)

				// TODO handle guid and entryUUID
				if strings.Contains(principalID, "entryUUID=") {
					guid = strings.Replace(principalID, "entryUUID=", "", -1)
				} else {
					dn = principalID
				}
			}
		}

		if guid == "" && dn == "" {
			continue
		}

		usersToExtend = append(usersToExtend, ExtendedUser{
			Name: user.Name,
			UUID: guid,
			DN:   dn,
		})
	}

	return usersToExtend
}

func encodeBase32(principalName string) string {
	encodedPrincipalID := base32.HexEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte(principalName))
	if len(encodedPrincipalID) > 63 {
		encodedPrincipalID = encodedPrincipalID[:63]
	}
	return encodedPrincipalID
}
