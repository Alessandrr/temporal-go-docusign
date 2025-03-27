package docusign

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
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

type DocusignAPIClient struct {
	BaseURL    string
	AuthHeader http.Header
	Client     *http.Client
}

func NewDocusignAPIClient(header http.Header, client *http.Client) *DocusignAPIClient {
	return &DocusignAPIClient{
		AuthHeader: header,
		Client:     client,
	}
}

type DocusignAuthInfoUpdater interface {
	UpdateAuthInfo(user DocusignUser) (DocusignUserCacheEntry, error)
}

type DocusignAuthService struct {
	cache     TokenCache[DocusignUser, DocusignUserCacheEntry]
	apiClient *DocusignAPIClient
}

func NewDocusignAuthService(cache TokenCache[DocusignUser, DocusignUserCacheEntry], client *DocusignAPIClient) *DocusignAuthService {
	return &DocusignAuthService{
		cache:     cache,
		apiClient: client,
	}
}

func (s *DocusignAuthService) UpdateAuthInfo(user DocusignUser) (DocusignUserCacheEntry, error) {
	currentAuthInfo, err := s.getAuthInfo(user)
	if err != nil {
		return DocusignUserCacheEntry{}, err
	}

	s.apiClient.BaseURL = currentAuthInfo.BaseURI
	s.apiClient.AuthHeader.Set("Authorization", "Bearer "+currentAuthInfo.AccessToken)

	return currentAuthInfo, nil
}

func (s *DocusignAuthService) getAuthInfo(user DocusignUser) (DocusignUserCacheEntry, error) {
	cacheEntry, ok := s.cache.Get(user)
	if ok {
		return cacheEntry, nil
	}

	token, err := makeDocusignToken(user)
	if err != nil {
		return DocusignUserCacheEntry{}, err
	}

	accessToken, err := getDocusignAccessToken(token)
	if err != nil {
		return DocusignUserCacheEntry{}, err
	}

	req, err := http.NewRequest("GET", "https://account-d.docusign.com/oauth/userinfo", nil)
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

	var accountId DocusignUserInfo

	err = json.NewDecoder(resp.Body).Decode(&accountId)
	if err != nil {
		fmt.Printf("Error decoding account ID: %s", err)
		return DocusignUserCacheEntry{}, err
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

func getDocusignAccessToken(jwtString string) (AccessToken, error) {
	resp, err := http.PostForm("https://account-d.docusign.com/oauth/token", url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"assertion":  {jwtString},
	})

	if err != nil {
		fmt.Printf("Error getting access token: %s", err)
		return AccessToken{}, err
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

func makeDocusignToken(user DocusignUser) (string, error) {
	rawJWT := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss":   "bab1a688-7783-4b90-92ac-d0c80dbbc9e5",
		"sub":   string(user),
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Unix() + 3600,
		"aud":   "account-d.docusign.com",
		"scope": "signature impersonation",
	})

	RSAPrivateKey, err := os.ReadFile("../private.pem")
	if err != nil {
		fmt.Printf("Error opening file: %s", err)
		return "", err
	}

	rsaPrivate, err := jwt.ParseRSAPrivateKeyFromPEM(RSAPrivateKey)

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
