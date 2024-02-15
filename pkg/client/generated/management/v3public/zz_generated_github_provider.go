package client

const (
	GithubProviderType                 = "githubProvider"
	GithubProviderFieldAnnotations     = "annotations"
	GithubProviderFieldAuthURL         = "authUrl"
	GithubProviderFieldClientID        = "clientId"
	GithubProviderFieldCreated         = "created"
	GithubProviderFieldCreatorID       = "creatorId"
	GithubProviderFieldDeviceAuthURL   = "deviceAuthUrl"
	GithubProviderFieldLabels          = "labels"
	GithubProviderFieldName            = "name"
	GithubProviderFieldOwnerReferences = "ownerReferences"
	GithubProviderFieldRedirectURL     = "redirectUrl"
	GithubProviderFieldRemoved         = "removed"
	GithubProviderFieldScopes          = "scopes"
	GithubProviderFieldTokenURL        = "tokenUrl"
	GithubProviderFieldType            = "type"
	GithubProviderFieldUUID            = "uuid"
)

type GithubProvider struct {
	Annotations     map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	AuthURL         string            `json:"authUrl,omitempty" yaml:"authUrl,omitempty"`
	ClientID        string            `json:"clientId,omitempty" yaml:"clientId,omitempty"`
	Created         string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	DeviceAuthURL   string            `json:"deviceAuthUrl,omitempty" yaml:"deviceAuthUrl,omitempty"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name            string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	RedirectURL     string            `json:"redirectUrl,omitempty" yaml:"redirectUrl,omitempty"`
	Removed         string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	Scopes          []string          `json:"scopes,omitempty" yaml:"scopes,omitempty"`
	TokenURL        string            `json:"tokenUrl,omitempty" yaml:"tokenUrl,omitempty"`
	Type            string            `json:"type,omitempty" yaml:"type,omitempty"`
	UUID            string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}
