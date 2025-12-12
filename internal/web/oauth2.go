// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/oauth2"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/config"
)

const (
	oauth2LoginStateGCMNonceSize = 12 // 96-bit nonce for AES-GCM
	oauth2LoginStateAESKeySize   = 32 // 32 bytes = AES-256
)

// oauth2Authenticator implements OAuth2 authentication.
type oauth2Authenticator struct {
	conf       *config.ConfigSpec
	kubeClient *Client
	provider   *oauth2Provider
	gcm        cipher.AEAD
}

// oauth2Provider has methods for implementing the OAuth2 protocol.
type oauth2Provider struct {
	verifyAccessToken func(ctx context.Context, accessToken string) (string, []string, error)
	verifyToken       func(ctx context.Context, token *oauth2.Token) (string, []string, *authStorage, error)
	config            func(ctx context.Context) (*oauth2.Config, error)
}

// newOAuth2Authenticator creates a new OAuth2 authenticator.
func newOAuth2Authenticator(ctx context.Context, conf *config.ConfigSpec, kubeClient *Client,
	provider *oauth2Provider) (*oauth2Authenticator, error) {

	// Validate that the provider can fetch a configuration.
	if _, err := provider.config(ctx); err != nil {
		return nil, fmt.Errorf("failed to fetch OAuth2 provider configuration: %w", err)
	}

	// Build encryptor/decryptor for login state cookies.
	hash := sha256.New
	secret := []byte(conf.Authentication.OAuth2.ClientSecret)
	var salt []byte // No salt since we need the derived key to be deterministic.
	const info = "oauth2 login state cookie encryption"
	key, err := hkdf.Key(hash, secret, salt, info, oauth2LoginStateAESKeySize)
	if err != nil {
		return nil, fmt.Errorf("failed to derive encryption key from client secret: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher for login state cookie: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM for login state cookie: %w", err)
	}

	return &oauth2Authenticator{
		conf:       conf,
		kubeClient: kubeClient,
		provider:   provider,
		gcm:        gcm,
	}, nil
}

// ServeAuthorize serves the OAuth2 authorize endpoint.
func (o *oauth2Authenticator) ServeAuthorize(w http.ResponseWriter, r *http.Request) {
	// Build OAuth2 config.
	oauth2Conf, err := o.config(r.Context())
	if err != nil {
		setAuthErrorCookie(w, err, http.StatusInternalServerError)
		http.Redirect(w, r, originalURL(r.URL.Query()), http.StatusSeeOther)
		return
	}
	oauth2Conf.ClientSecret = "" // No need for client secret in this part of the flow.

	// Build and set state.
	pkceVerifier := oauth2.GenerateVerifier()
	pkceChallenge := oauth2.S256ChallengeFromVerifier(pkceVerifier)
	csrfToken := oauth2.GenerateVerifier() // Can also be used to generate CSRF tokens.
	state, err := o.encodeState(oauth2LoginState{
		PKCEVerifier: pkceVerifier,
		CSRFToken:    csrfToken,
		URLQuery:     r.URL.Query(),
		ExpiresAt:    time.Now().Add(cookieDurationShortLived),
	})
	if err != nil {
		setAuthErrorCookie(w, err, http.StatusInternalServerError)
		http.Redirect(w, r, originalURL(r.URL.Query()), http.StatusSeeOther)
		return
	}
	setSecureCookie(w, cookieNameOAuth2LoginState, cookiePathOAuth2LoginState,
		state, cookieDurationShortLived, !o.conf.Insecure)

	// Redirect to authorization URL.
	authCodeURL := oauth2Conf.AuthCodeURL(state,
		oauth2.SetAuthURLParam("code_challenge", pkceChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"))
	http.Redirect(w, r, authCodeURL, http.StatusSeeOther)
}

// ServeCallback serves the OAuth2 callback endpoint.
func (o *oauth2Authenticator) ServeCallback(w http.ResponseWriter, r *http.Request) {
	// Parse state.
	queryState, cookieState := consumeOAuth2LoginStates(w, r)
	if queryState == "" {
		respondAuthError(w, r, fmt.Errorf("OAuth2 callback did not have state"), http.StatusBadRequest)
		return
	}
	if cookieState != "" && cookieState != queryState {
		respondAuthError(w, r, fmt.Errorf("OAuth2 callback state mismatch between cookie and query parameter"), http.StatusBadRequest)
		return
	}
	state, err := o.decodeState(queryState)
	if err != nil {
		respondAuthError(w, r, err, http.StatusBadRequest)
		return
	}
	if cookieState == "" {
		err := fmt.Errorf("OAuth login state cookie has expired")
		setAuthErrorCookie(w, err, http.StatusUnauthorized)
		http.Redirect(w, r, state.redirectURL(), http.StatusSeeOther)
		return
	}
	if state.ExpiresAt.Before(time.Now()) {
		err := fmt.Errorf("OAuth login state has expired")
		setAuthErrorCookie(w, err, http.StatusUnauthorized)
		http.Redirect(w, r, state.redirectURL(), http.StatusSeeOther)
		return
	}

	// Exchange code for token.
	oauth2Conf, err := o.config(r.Context())
	if err != nil {
		setAuthErrorCookie(w, err, http.StatusInternalServerError)
		http.Redirect(w, r, state.redirectURL(), http.StatusSeeOther)
		return
	}
	token, err := oauth2Conf.Exchange(r.Context(), r.URL.Query().Get("code"),
		oauth2.SetAuthURLParam("code_verifier", state.PKCEVerifier))
	if err != nil {
		setAuthErrorCookie(w, err, http.StatusUnauthorized)
		http.Redirect(w, r, state.redirectURL(), http.StatusSeeOther)
		return
	}

	// Try to authenticate the token and set the auth storage.
	if _, _, err := o.verifyTokenAndSetAuthStorage(w, r, token); err != nil {
		return
	}

	// Authentication successful. Set the auth provider cookie.
	const authenticated = true
	setAuthProviderCookie(w, o.conf.Authentication.OAuth2.Provider, oauth2PathAuthorize, authenticated)

	http.Redirect(w, r, state.redirectURL(), http.StatusSeeOther)
}

// ServeAPI serves API requests enforcing OAuth2 authentication.
// It retrieves the authentication storage from the request,
// verifies the access token, and refreshes it if necessary.
func (o *oauth2Authenticator) ServeAPI(w http.ResponseWriter, r *http.Request, api http.Handler) {
	// Set the auth provider cookie to indicate OAuth2 is in use and not yet authenticated.
	authenticated := false
	setAuthProviderCookie(w, o.conf.Authentication.OAuth2.Provider, oauth2PathAuthorize, authenticated)

	// Try to authenticate the request.
	as, err := getAuthStorage(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	username, groups, err := o.provider.verifyAccessToken(r.Context(), as.AccessToken)
	if err != nil {
		if as.RefreshToken == "" {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		oauth2Conf, err := o.config(r.Context())
		if err != nil {
			respondAuthError(w, r, err, http.StatusInternalServerError)
			return
		}
		token, err := oauth2Conf.
			TokenSource(r.Context(), &oauth2.Token{RefreshToken: as.RefreshToken}).
			Token()
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		if username, groups, err = o.verifyTokenAndSetAuthStorage(w, r, token); err != nil {
			return
		}
	}

	// Authentication successful. Set the auth provider cookie.
	w.Header().Del("Set-Cookie")
	authenticated = true
	setAuthProviderCookie(w, o.conf.Authentication.OAuth2.Provider, oauth2PathAuthorize, authenticated)

	// Build and store user session.
	client, err := o.kubeClient.getUserClientFromCache(username, groups)
	if err != nil {
		respondAuthError(w, r, err, http.StatusInternalServerError)
		return
	}
	ctx := storeUserSession(r.Context(), username, groups, client)
	r = r.WithContext(ctx)

	// Serve the API request.
	api.ServeHTTP(w, r)
}

// ServeAssets serves asset requests enhancing them with the auth provider cookie.
func (o *oauth2Authenticator) ServeAssets(w http.ResponseWriter, r *http.Request, assets http.Handler) {
	defer assets.ServeHTTP(w, r)

	// Set the auth provider cookie to indicate OAuth2 is in use and not yet authenticated.
	authenticated := false
	setAuthProviderCookie(w, o.conf.Authentication.OAuth2.Provider, oauth2PathAuthorize, authenticated)

	// Try to authenticate the request.
	as, err := getAuthStorage(r)
	if err != nil {
		return
	}
	if _, _, err := o.provider.verifyAccessToken(r.Context(), as.AccessToken); err != nil {
		if as.RefreshToken == "" {
			return
		}
		oauth2Conf, err := o.config(r.Context())
		if err != nil {
			return
		}
		token, err := oauth2Conf.
			TokenSource(r.Context(), &oauth2.Token{RefreshToken: as.RefreshToken}).
			Token()
		if err != nil {
			return
		}
		if _, _, as, err = o.provider.verifyToken(r.Context(), token); err != nil {
			return
		}
		if err := setAuthStorage(o.conf, w, *as); err != nil {
			return
		}
	}

	// Authentication successful. Set the auth provider cookie.
	w.Header().Del("Set-Cookie")
	authenticated = true
	setAuthProviderCookie(w, o.conf.Authentication.OAuth2.Provider, oauth2PathAuthorize, authenticated)
}

// config builds the OAuth2 configuration from the
// provider base and from the web server configuration.
func (o *oauth2Authenticator) config(ctx context.Context) (*oauth2.Config, error) {
	cfg, err := o.provider.config(ctx)
	if err != nil {
		return nil, err
	}
	cfg.ClientID = o.conf.Authentication.OAuth2.ClientID
	cfg.ClientSecret = o.conf.Authentication.OAuth2.ClientSecret
	cfg.RedirectURL = o.conf.BaseURL + oauth2PathCallback
	if s := o.conf.Authentication.OAuth2.Scopes; len(s) > 0 {
		cfg.Scopes = s
	}
	return cfg, nil
}

// verifyTokenAndSetAuthStorage verifies the OAuth2 token and sets
// the authentication storage in a cookie, or responds on any errors.
func (o *oauth2Authenticator) verifyTokenAndSetAuthStorage(
	w http.ResponseWriter, r *http.Request, token *oauth2.Token) (string, []string, error) {
	username, groups, as, err := o.provider.verifyToken(r.Context(), token)
	if err != nil {
		respondAuthError(w, r, err, http.StatusUnauthorized)
		return "", nil, err
	}
	if err := setAuthStorage(o.conf, w, *as); err != nil {
		respondAuthError(w, r, err, http.StatusInternalServerError)
		return "", nil, err
	}
	return username, groups, nil
}

// oauth2LoginState holds the OAuth2 login state information.
// The OAuth2 login state is a very short-lived blob stored in a cookie
// to maintain state between the OAuth2 login request and callback.
// We resourcefully and securely store it a cookie to avoid server-side
// session storage. We leverage the OAuth2 Client Secret to encrypt and
// sign the state cookie. Encryption here is justified because we send
// sensitive information (e.g. k8s resource names) to the IdP in the
// authorization URL.
//
// This cookie is needed for implementing hardening mechanisms like PKCE
// and CSRF, and to preserve the original URL query parameters, allowing
// the server to redirect the application back to the original URL after
// login. To get redirected to the original path, the application must
// send the query parameter "originalPath".
type oauth2LoginState struct {
	PKCEVerifier string     `json:"pkceVerifier"`
	CSRFToken    string     `json:"csrfToken"`
	URLQuery     url.Values `json:"urlQuery"`
	ExpiresAt    time.Time  `json:"expiresAt"`
}

// redirectURL builds the redirect URL for OAuth2 login.
func (o *oauth2LoginState) redirectURL() string {
	return originalURL(o.URLQuery)
}

// originalURL builds the redirect URL from the original path query parameter.
func originalURL(q url.Values) string {
	redirectPath := "/"
	if p := q.Get(authQueryParamOriginalPath); p != "" {
		redirectPath = p
		q.Del(authQueryParamOriginalPath)
	}
	redirectURL := redirectPath
	if len(q) > 0 {
		redirectURL += "?" + q.Encode()
	}
	return redirectURL
}

// encodeState encodes the OAuth2 login state.
func (o *oauth2Authenticator) encodeState(state oauth2LoginState) (string, error) {
	b, err := json.Marshal(state)
	if err != nil {
		return "", fmt.Errorf("failed to marshal oauth2 login state cookie: %w", err)
	}
	nonce := make([]byte, oauth2LoginStateGCMNonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce for oauth2 login state cookie: %w", err)
	}
	ciphertext := o.gcm.Seal(nil, nonce, b, nil)
	return base64.RawURLEncoding.EncodeToString(append(nonce, ciphertext...)), nil
}

// decodeState decodes the OAuth2 login state.
func (o *oauth2Authenticator) decodeState(s string) (*oauth2LoginState, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("failed to decode oauth2 login state: %w", err)
	}
	if len(b) < oauth2LoginStateGCMNonceSize {
		return nil, fmt.Errorf("invalid oauth2 login state size")
	}
	nonce, ciphertext := b[:oauth2LoginStateGCMNonceSize], b[oauth2LoginStateGCMNonceSize:]
	plaintext, err := o.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt oauth2 login state: %w", err)
	}
	var state oauth2LoginState
	if err := json.Unmarshal(plaintext, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal oauth2 login state: %w", err)
	}
	return &state, nil
}

// consumeOAuth2LoginStates retrieves the OAuth2 login state from the query
// parameters and from the cookies and deletes the cookie.
func consumeOAuth2LoginStates(w http.ResponseWriter, r *http.Request) (string, string) {
	defer deleteCookie(w, cookieNameOAuth2LoginState, cookiePathOAuth2LoginState)
	queryState := r.URL.Query().Get("state")
	var cookieState string
	if c, err := r.Cookie(cookieNameOAuth2LoginState); err == nil {
		cookieState = c.Value
	}
	return queryState, cookieState
}
