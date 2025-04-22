package docusign

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
)

type DocusignUser string

const (
	DocusignTestUser DocusignUser = "4e3a9121-335b-48dc-a20e-a0f46f4a277c"
)

const DS_JWT_BUFFER = 5 * time.Minute

type DocusignUserInfo struct {
	Email    string                `json:"email"`
	Accounts []DocusignAccountInfo `json:"accounts"`
}

type DocusignAccountInfo struct {
	AccountID   string `json:"account_id"`
	IsDefault   bool   `json:"is_default"`
	AccountName string `json:"account_name"`
	BaseURI     string `json:"base_uri"`
}

type DocusignUserCacheEntry struct {
	AccessToken string
	AccountId   string
	BaseURI     string
	CreatedAt   time.Time
	TTL         time.Duration
}

type AccessToken struct {
	Token string `json:"access_token"`
	Type  string `json:"token_type"`
	Exp   int    `json:"expires_in"`
}

type APIClient struct {
	BaseURL    string
	AuthHeader http.Header
	Client     *http.Client
}

func NewAPIClient(header http.Header, client *http.Client) *APIClient {
	return &APIClient{
		AuthHeader: header,
		Client:     client,
	}
}

type AuthInfoUpdater interface {
	UpdateAuthInfo(user DocusignUser) (DocusignUserCacheEntry, error)
}

type AuthService struct {
	cache     TokenCache[DocusignUser, DocusignUserCacheEntry]
	apiClient *APIClient
	config    *DocusignConfig
}

func NewAuthService(cache TokenCache[DocusignUser, DocusignUserCacheEntry], client *APIClient, config *DocusignConfig) *AuthService {
	return &AuthService{
		cache:     cache,
		apiClient: client,
		config:    config,
	}
}

func (s *AuthService) UpdateAuthInfo(user DocusignUser) (DocusignUserCacheEntry, error) {
	currentAuthInfo, err := s.getAuthInfo(user)
	if err != nil {
		return DocusignUserCacheEntry{}, err
	}

	s.apiClient.BaseURL = currentAuthInfo.BaseURI
	s.apiClient.AuthHeader.Set("Authorization", "Bearer "+currentAuthInfo.AccessToken)

	return currentAuthInfo, nil
}

func (s *AuthService) getAuthInfo(user DocusignUser) (DocusignUserCacheEntry, error) {
	cacheEntry, ok := s.cache.Get(user)
	if ok {
		return cacheEntry, nil
	}

	token, err := s.makeDocusignToken(user)
	if err != nil {
		return DocusignUserCacheEntry{}, err
	}

	accessToken, err := s.getDocusignAccessToken(token)
	if err != nil {
		return DocusignUserCacheEntry{}, err
	}

	req, err := http.NewRequest("GET", s.config.BaseAuthURL+"/oauth/userinfo", nil)
	req.Header.Add("Authorization", "Bearer "+accessToken.Token)
	if err != nil {
		fmt.Printf("Error creating request: %s", err)
		return DocusignUserCacheEntry{}, err
	}

	resp, err := s.apiClient.Client.Do(req)
	if err != nil {
		fmt.Printf("Error sending request: %s", err)
		return DocusignUserCacheEntry{}, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Request failed with status code: %d, body: %s", resp.StatusCode, string(body))
		return DocusignUserCacheEntry{}, fmt.Errorf("request failed with status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var accountId DocusignUserInfo

	err = json.NewDecoder(resp.Body).Decode(&accountId)
	if err != nil {
		fmt.Printf("Error decoding account ID: %s", err)
		return DocusignUserCacheEntry{}, err
	}

	if len(accountId.Accounts) == 0 {
		return DocusignUserCacheEntry{}, fmt.Errorf("no accounts found for user %s", user)
	}

	newEntry := DocusignUserCacheEntry{
		AccessToken: accessToken.Token,
		AccountId:   accountId.Accounts[0].AccountID,
		BaseURI:     accountId.Accounts[0].BaseURI,
		CreatedAt:   time.Now(),
		TTL:         time.Duration(accessToken.Exp)*time.Second - DS_JWT_BUFFER,
	}

	s.cache.Set(user, newEntry)

	return newEntry, nil
}

func (s *AuthService) getDocusignAccessToken(jwtString string) (AccessToken, error) {
	resp, err := http.PostForm(s.config.BaseAuthURL+"/oauth/token", url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"assertion":  {jwtString},
	})

	if err != nil {
		fmt.Printf("Error getting access token: %s", err)
		return AccessToken{}, err
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Request failed with status code: %d, body: %s", resp.StatusCode, string(body))
		return AccessToken{}, fmt.Errorf("request failed with status code: %d, body: %s", resp.StatusCode, string(body))
	}

	defer resp.Body.Close()

	var token AccessToken

	err = json.NewDecoder(resp.Body).Decode(&token)
	if err != nil {
		fmt.Printf("Error decoding access token: %s", err)
		return AccessToken{}, err
	}

	return token, nil
}

func (s *AuthService) makeDocusignToken(user DocusignUser) (string, error) {
	rawJWT := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss":   s.config.ClientID,
		"sub":   string(user),
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Unix() + 3600,
		"aud":   strings.TrimPrefix(s.config.BaseAuthURL, "https://"),
		"scope": "signature impersonation",
	})

	pem := s.config.PrivateKey
	rsaPrivate, err := jwt.ParseRSAPrivateKeyFromPEM(pem)

	if err != nil {
		fmt.Printf("key update error for: %s", err)
		return "", err
	}

	tokenString, err := rawJWT.SignedString(rsaPrivate)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}
