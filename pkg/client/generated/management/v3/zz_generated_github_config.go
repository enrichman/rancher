package client

const (
	GithubConfigType                     = "githubConfig"
	GithubConfigFieldAccessMode          = "accessMode"
	GithubConfigFieldAdditionalClientIDs = "additionalClientIds"
	GithubConfigFieldAllowedPrincipalIDs = "allowedPrincipalIds"
	GithubConfigFieldAnnotations         = "annotations"
	GithubConfigFieldAuthEndpoint        = "authEndpoint"
	GithubConfigFieldClientID            = "clientId"
	GithubConfigFieldClientSecret        = "clientSecret"
	GithubConfigFieldCreated             = "created"
	GithubConfigFieldCreatorID           = "creatorId"
	GithubConfigFieldDeviceAuthEndpoint  = "deviceAuthEndpoint"
	GithubConfigFieldEnabled             = "enabled"
	GithubConfigFieldHostname            = "hostname"
	GithubConfigFieldHostnameToClientID  = "hostnameToClientId"
	GithubConfigFieldLabels              = "labels"
	GithubConfigFieldName                = "name"
	GithubConfigFieldOwnerReferences     = "ownerReferences"
	GithubConfigFieldRemoved             = "removed"
	GithubConfigFieldStatus              = "status"
	GithubConfigFieldTLS                 = "tls"
	GithubConfigFieldTokenEndpoint       = "tokenEndpoint"
	GithubConfigFieldType                = "type"
	GithubConfigFieldUUID                = "uuid"
)

type GithubConfig struct {
	AccessMode          string            `json:"accessMode,omitempty" yaml:"accessMode,omitempty"`
	AdditionalClientIDs map[string]string `json:"additionalClientIds,omitempty" yaml:"additionalClientIds,omitempty"`
	AllowedPrincipalIDs []string          `json:"allowedPrincipalIds,omitempty" yaml:"allowedPrincipalIds,omitempty"`
	Annotations         map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	AuthEndpoint        string            `json:"authEndpoint,omitempty" yaml:"authEndpoint,omitempty"`
	ClientID            string            `json:"clientId,omitempty" yaml:"clientId,omitempty"`
	ClientSecret        string            `json:"clientSecret,omitempty" yaml:"clientSecret,omitempty"`
	Created             string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID           string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	DeviceAuthEndpoint  string            `json:"deviceAuthEndpoint,omitempty" yaml:"deviceAuthEndpoint,omitempty"`
	Enabled             bool              `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Hostname            string            `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	HostnameToClientID  map[string]string `json:"hostnameToClientId,omitempty" yaml:"hostnameToClientId,omitempty"`
	Labels              map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences     []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed             string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	Status              *AuthConfigStatus `json:"status,omitempty" yaml:"status,omitempty"`
	TLS                 bool              `json:"tls,omitempty" yaml:"tls,omitempty"`
	TokenEndpoint       string            `json:"tokenEndpoint,omitempty" yaml:"tokenEndpoint,omitempty"`
	Type                string            `json:"type,omitempty" yaml:"type,omitempty"`
	UUID                string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}
