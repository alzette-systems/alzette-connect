package session

import "time"

type Metadata struct {
	Schema         string   `json:"schema"`
	Issuer         string   `json:"issuer"`
	OAuthClientID  string   `json:"oauth_client_id"`
	ControlOrigin  string   `json:"control_origin"`
	GatewayBaseURL string   `json:"gateway_base_url"`
	OAuthRedirect  string   `json:"oauth_redirect_uri"`
	LoginModes     []string `json:"login_modes"`
}

type Discovery struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	RevocationEndpoint    string `json:"revocation_endpoint,omitempty"`
}

type oauthTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

type Context struct {
	MembershipID string   `json:"membership_id"`
	Organisation string   `json:"organisation"`
	Project      string   `json:"project"`
	Environment  string   `json:"environment"`
	Relationship string   `json:"relationship"`
	ModelAliases []string `json:"model_aliases"`
}

type contextsResponse struct {
	Schema   string    `json:"schema"`
	Contexts []Context `json:"contexts"`
}

type MintInput struct {
	ClientInstanceID string   `json:"client_instance_id"`
	MembershipID     string   `json:"membership_id"`
	ModelAliases     []string `json:"model_aliases,omitempty"`
}

type mintResponse struct {
	Schema     string `json:"schema"`
	Credential struct {
		AccessToken string    `json:"access_token"`
		TokenType   string    `json:"token_type"`
		ExpiresAt   time.Time `json:"expires_at"`
		Scope       []string  `json:"scope"`
	} `json:"credential"`
	Context        Context  `json:"context"`
	GatewayBaseURL string   `json:"gateway_base_url"`
	ModelAliases   []string `json:"model_aliases"`
}
