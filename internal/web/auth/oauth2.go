// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package auth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"sigs.k8s.io/controller-runtime/pkg/log"

	webconfig "github.com/controlplaneio-fluxcd/flux-operator/internal/web/config"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/kubeclient"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

const (
	oauth2LoginStateGCMNonceSize = 12 // 96-bit nonce for AES-GCM
	oauth2LoginStateAESKeySize   = 32 // 32 bytes = AES-256
)

const (
	errInvalidOAuth2Scopes = "The OAuth2 provider does not support the requested scopes. " +
		"If you are using the default scopes, please consider setting custom " +
		"scopes in the OAuth2 configuration that are supported by your provider: " +
		"https://fluxoperator.dev/docs/web-ui/web-config-api/#oidc-provider"

	logCookieTooLarge = "The credentials issued by the OAuth2 provider are too large to fit in HTTP cookies. " +
		"If your provider is Dex with the Microsoft connector, please consider reducing " +
		"the number of groups returned by Dex: " +
		"https://fluxoperator.dev/docs/web-ui/sso-microsoft#restricting-the-groups-added-by-dex-to-the-id-token"
)

// oauth2Authenticator implements OAuth2 authentication.
type oauth2Authenticator struct {
	conf       *webconfig.ConfigSpec
	kubeClient *kubeclient.Client
	provider   oauth2Provider
	gcm        cipher.AEAD
}

// oauth2Provider has methods for implementing the OAuth2 protocol.
type oauth2Provider interface {
	init(ctx context.Context) (initializedOAuth2Provider, error)
}

// initializedOAuth2Provider has methods for implementing the OAuth2 protocol.
type initializedOAuth2Provider interface {
	config() *oauth2.Config
	newVerifier(ctx context.Context) (oauth2Verifier, error)
}

// oauth2Verifier has methods for verifying OAuth2 tokens.
type oauth2Verifier interface {
	verifyAccessToken(ctx context.Context, accessToken string, nonce ...string) (*user.Details, error)
	verifyToken(ctx context.Context, token *oauth2.Token, nonce ...string) (*user.Details, *authStorage, error)
}

// newOAuth2Authenticator creates a new OAuth2 authenticator.
func newOAuth2Authenticator(conf *webconfig.ConfigSpec,
	kubeClient *kubeclient.Client, provider oauth2Provider) (*oauth2Authenticator, error) {

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

// serveAuthorize serves the OAuth2 authorize endpoint.
func (o *oauth2Authenticator) serveAuthorize(w http.ResponseWriter, r *http.Request) {
	// Build OAuth2 config.
	p, err := o.provider.init(r.Context())
	if err != nil {
		log.FromContext(r.Context()).Error(err, "failed to initialize OAuth2 provider")
		setAuthErrorCookie(w, errInternalError)
		http.Redirect(w, r, originalURL(r.URL.Query()), http.StatusSeeOther)
		return
	}
	oauth2Conf := o.config(p)
	oauth2Conf.ClientSecret = "" // No need for client secret in this part of the flow.

	// Build and set state.
	pkceVerifier := oauth2.GenerateVerifier()
	pkceChallenge := oauth2.S256ChallengeFromVerifier(pkceVerifier)
	csrfToken := oauth2.GenerateVerifier()
	nonce := oauth2.GenerateVerifier()
	state, err := o.encodeState(oauth2LoginState{
		PKCEVerifier: pkceVerifier,
		CSRFToken:    csrfToken,
		Nonce:        nonce,
		URLQuery:     r.URL.Query(),
		ExpiresAt:    time.Now().Add(cookieDurationShortLived),
	})
	if err != nil {
		log.FromContext(r.Context()).Error(err, "failed to encode OAuth2 login state")
		setAuthErrorCookie(w, errInternalError)
		http.Redirect(w, r, originalURL(r.URL.Query()), http.StatusSeeOther)
		return
	}
	setSecureCookie(w, cookieNameOAuth2LoginState, cookiePathOAuth2LoginState,
		state, cookieDurationShortLived, !o.conf.Insecure)

	// Redirect to authorization URL.
	authCodeURL := oauth2Conf.AuthCodeURL(state,
		oauth2.SetAuthURLParam("code_challenge", pkceChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		oauth2.SetAuthURLParam("nonce", nonce))
	http.Redirect(w, r, authCodeURL, http.StatusSeeOther)
}

// serveCallback serves the OAuth2 callback endpoint.
func (o *oauth2Authenticator) serveCallback(w http.ResponseWriter, r *http.Request) {
	// Check callback error and log but do not respond yet, see if we
	// can redirect back to the original URL after parsing the state.
	const errorCodeKey = "error"
	const errorDescKey = "error_description"
	const errorURIKey = "error_uri"
	var callbackErr error
	errCode := r.URL.Query().Get(errorCodeKey)
	errDesc := r.URL.Query().Get(errorDescKey)
	errURI := r.URL.Query().Get(errorURIKey)
	if errCode != "" || errDesc != "" || errURI != "" {
		const logMsg = "OAuth2 callback error"
		const invalidScope = "invalid_scope"
		switch {
		// Special case: it's common needing to configure the correct scopes.
		case strings.Contains(errCode, invalidScope), strings.Contains(errDesc, invalidScope):
			callbackErr = fmt.Errorf("%s", errInvalidOAuth2Scopes)
			log.FromContext(r.Context()).Error(callbackErr, logMsg)
		default:
			callbackErr = errInternalError
			errFields := map[string]any{
				errorCodeKey: errCode,
				errorDescKey: errDesc,
				errorURIKey:  errURI,
			}
			// For user errors, log at V(1) to reduce log noise.
			noise := errCode == "access_denied" || strings.HasSuffix(errCode, "_required")
			if noise {
				callbackErr = errUserError
				log.FromContext(r.Context()).V(1).Info(logMsg, "oauth2Error", errFields)
			} else {
				log.FromContext(r.Context()).Error(callbackErr, logMsg, "oauth2Error", errFields)
			}
		}
		setAuthErrorCookie(w, callbackErr)
	}

	// Parse state.
	queryState, cookieState := consumeOAuth2LoginStates(w, r)
	if queryState == "" {
		if callbackErr == nil {
			const msg = "the OAuth2 callback state is missing in the query parameters"
			log.FromContext(r.Context()).Error(errors.New(msg), msg)
			setAuthErrorCookie(w, errInternalError)
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if cookieState != "" && cookieState != queryState {
		if callbackErr == nil {
			const msg = "the OAuth2 callback state cookie does not match the query parameter"
			log.FromContext(r.Context()).Error(errors.New(msg), msg)
			setAuthErrorCookie(w, errInternalError)
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	state, err := o.decodeState(queryState)
	if err != nil {
		if callbackErr == nil {
			log.FromContext(r.Context()).Error(err, "failed to decode OAuth2 login state")
			setAuthErrorCookie(w, errInternalError)
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Now that we have a redirect URL, we can defer the response to it.
	defer http.Redirect(w, r, state.redirectURL(), http.StatusSeeOther)

	// If there was a callback error, nothing more to do, it was
	// already logged and the auth error cookie set.
	if callbackErr != nil {
		return
	}

	// Check expiry errors.
	if cookieState == "" {
		log.FromContext(r.Context()).V(1).Info("OAuth2 login state cookie expired")
		setAuthErrorCookie(w, errUserError)
		return
	}
	if state.ExpiresAt.Before(time.Now()) {
		log.FromContext(r.Context()).V(1).Info("OAuth2 login state expired")
		setAuthErrorCookie(w, errUserError)
		return
	}

	// Initialize provider and verifier.
	p, v, err := o.providerAndVerifierOrLogError(r.Context())
	if err != nil {
		setAuthErrorCookie(w, err)
		return
	}

	// Exchange code for token.
	code := r.URL.Query().Get("code")
	token, err := o.config(p).Exchange(r.Context(), code,
		oauth2.SetAuthURLParam("code_verifier", state.PKCEVerifier))
	if err != nil {
		log.FromContext(r.Context()).Error(err, "failed to exchange code for token")
		setAuthErrorCookie(w, errInternalError)
		return
	}

	// Verify the token and set the auth storage.
	if _, err := o.verifyTokenAndSetStorageOrLogError(r.Context(), w, v, token, state.Nonce); err != nil {
		setAuthErrorCookie(w, err)
		return
	}

	// Authentication successful. Set the auth provider cookie.
	o.setAuthenticated(w)
}

// serveAPI serves API requests enforcing OAuth2 authentication.
// It retrieves the authentication storage from the request,
// verifies the access token, and refreshes it if necessary.
func (o *oauth2Authenticator) serveAPI(w http.ResponseWriter, r *http.Request, api http.Handler) {
	// Set the auth provider cookie to indicate OAuth2 is in use and not yet authenticated.
	o.setUnauthenticated(w)

	// Try to authenticate the request refreshing the access token if needed.
	as, err := getAuthStorage(r)
	if err != nil {
		log.FromContext(r.Context()).V(1).Info("failed to get auth storage from request", "error", err.Error())
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	p, v, err := o.providerAndVerifierOrLogError(r.Context())
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	details := o.verifyAccessTokenOrDeleteStorageAndLogError(r.Context(), w, v, as.AccessToken)
	if details == nil {
		if as.RefreshToken == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		token := o.refreshTokenOrLogError(r.Context(), p, as.RefreshToken)
		if token == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if details, err = o.verifyTokenAndSetStorageOrLogError(r.Context(), w, v, token); err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	// Authentication successful. Set the auth provider cookie.
	o.setAuthenticated(w)

	// Build and store user session.
	client, err := o.kubeClient.GetUserClientFromCache(details.Impersonation)
	if err != nil {
		log.FromContext(r.Context()).Error(err, "failed to create Kubernetes client for user")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	ctx := user.StoreSession(r.Context(), *details, client)
	l := log.FromContext(ctx).WithValues("permissions", details.Impersonation)
	ctx = log.IntoContext(ctx, l)
	r = r.WithContext(ctx)

	// Serve the API request.
	api.ServeHTTP(w, r)
}

// serveIndex serves the index.html page enhancing it with the auth provider cookie.
func (o *oauth2Authenticator) serveIndex(w http.ResponseWriter, r *http.Request, assets http.Handler) {
	defer assets.ServeHTTP(w, r)

	// Avoid blocking the index page load.
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Set the auth provider cookie to indicate OAuth2 is in use and not yet authenticated.
	o.setUnauthenticated(w)

	// Try to authenticate the request refreshing the access token if needed.
	as, err := getAuthStorage(r)
	if err != nil {
		log.FromContext(ctx).V(1).Info("failed to get auth storage from request", "error", err.Error())
		return
	}
	p, v, err := o.providerAndVerifierOrLogError(ctx)
	if err != nil {
		return
	}
	if o.verifyAccessTokenOrDeleteStorageAndLogError(ctx, w, v, as.AccessToken) == nil {
		if as.RefreshToken == "" {
			return
		}
		token := o.refreshTokenOrLogError(ctx, p, as.RefreshToken)
		if token == nil {
			return
		}
		if _, err := o.verifyTokenAndSetStorageOrLogError(ctx, w, v, token); err != nil {
			return
		}
	}

	// Authentication successful. Set the auth provider cookie.
	o.setAuthenticated(w)
}

// providerAndVerifierOrLogError initializes the OAuth2 provider
// and verifier, or logs any error and returns internalErr on
// failure.
func (o *oauth2Authenticator) providerAndVerifierOrLogError(
	ctx context.Context) (initializedOAuth2Provider, oauth2Verifier, error) {

	p, err := o.provider.init(ctx)
	if err != nil {
		log.FromContext(ctx).Error(err, "failed to initialize OAuth2 provider")
		return nil, nil, errInternalError
	}

	v, err := p.newVerifier(ctx)
	if err != nil {
		log.FromContext(ctx).Error(err, "failed to initialize OAuth2 provider verifier")
		return nil, nil, errInternalError
	}

	return p, v, nil
}

// config builds the OAuth2 configuration from the
// provider base and from the web server configuration.
func (o *oauth2Authenticator) config(p initializedOAuth2Provider) *oauth2.Config {
	base := p.config()
	base.ClientID = o.conf.Authentication.OAuth2.ClientID
	base.ClientSecret = o.conf.Authentication.OAuth2.ClientSecret
	base.RedirectURL = o.conf.BaseURL + oauth2PathCallback
	if s := o.conf.Authentication.OAuth2.Scopes; len(s) > 0 {
		base.Scopes = s
	}
	return base
}

// verifyTokenAndSetStorageOrLogError verifies the token, sets
// the auth storage and returns the user details, or logs any
// error and returns an error for the auth error cookie.
func (o *oauth2Authenticator) verifyTokenAndSetStorageOrLogError(
	ctx context.Context, w http.ResponseWriter, v oauth2Verifier,
	token *oauth2.Token, nonce ...string) (*user.Details, error) {

	details, as, err := v.verifyToken(ctx, token, nonce...)
	if err != nil {
		log.FromContext(ctx).Error(err, "failed to verify token")
		return nil, errUserError
	}

	if err := setAuthStorage(o.conf, w, *as); err != nil {
		log.FromContext(ctx).Error(err, logCookieTooLarge)
		return nil, errInternalError
	}

	return details, nil
}

// verifyAccessTokenOrDeleteStorageAndLogError verifies the access token and
// returns the user details, or logs any error encountered and returns nil.
func (o *oauth2Authenticator) verifyAccessTokenOrDeleteStorageAndLogError(ctx context.Context,
	w http.ResponseWriter, v oauth2Verifier, accessToken string) *user.Details {

	details, err := v.verifyAccessToken(ctx, accessToken)
	if err != nil {
		log.FromContext(ctx).V(1).Info("failed to verify access token", "error", err.Error())
		deleteAuthStorage(w)
		return nil
	}

	return details
}

// refreshTokenOrLogError refreshes the access token using the
// refresh token, or logs any error encountered and returns nil.
func (o *oauth2Authenticator) refreshTokenOrLogError(
	ctx context.Context, p initializedOAuth2Provider, refreshToken string) *oauth2.Token {
	token, err := o.config(p).
		TokenSource(ctx, &oauth2.Token{RefreshToken: refreshToken}).
		Token()
	if err != nil {
		log.FromContext(ctx).V(1).Info("failed to refresh access token", "error", err.Error())
		return nil
	}
	return token
}

// setAuthenticated sets the authentication provider cookie
// to indicate that the user is authenticated.
func (o *oauth2Authenticator) setAuthenticated(w http.ResponseWriter) {
	o.setAuthProvider(w, true)
}

// setUnauthenticated sets the authentication provider cookie
// to indicate that the user is not authenticated.
func (o *oauth2Authenticator) setUnauthenticated(w http.ResponseWriter) {
	o.setAuthProvider(w, false)
}

// setAuthProvider sets the authentication provider cookie.
func (o *oauth2Authenticator) setAuthProvider(w http.ResponseWriter, authenticated bool) {
	setAuthProviderCookie(w,
		o.conf.Authentication.OAuth2.Provider,
		o.conf.BaseURL+oauth2PathAuthorize,
		authenticated)
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
	Nonce        string     `json:"nonce"`
	URLQuery     url.Values `json:"urlQuery"`
	ExpiresAt    time.Time  `json:"expiresAt"`
}

// redirectURL builds the redirect URL for OAuth2 login.
func (o *oauth2LoginState) redirectURL() string {
	return originalURL(o.URLQuery)
}

// originalURL builds the redirect URL from the original path query parameter.
// It validates that the path is a safe relative path to prevent open redirects.
func originalURL(q url.Values) string {
	redirectPath := "/"
	if p := q.Get(authQueryParamOriginalPath); p != "" && isSafeRedirectPath(p) {
		redirectPath = p
	}
	// Always delete originalPath from query params to avoid it appearing in the final URL
	q.Del(authQueryParamOriginalPath)
	redirectURL := redirectPath
	if len(q) > 0 {
		redirectURL += "?" + q.Encode()
	}
	return redirectURL
}

// isSafeRedirectPath validates that the path is a safe relative path.
// It prevents open redirect attacks by ensuring the path:
// - Starts with a single forward slash
// - Second character is safe (not /, \, or whitespace/control chars)
// - Does not have a scheme before the first slash (e.g., http://...)
func isSafeRedirectPath(path string) bool {
	// Must start with /
	if !strings.HasPrefix(path, "/") {
		return false
	}
	// Check second character if present - must be a safe path character.
	// Browsers interpret //host, /\host, /\thost, etc. as absolute URLs.
	if len(path) > 1 {
		c := path[1]
		// Block: / \ and any control/whitespace characters (ASCII < 33)
		// Safe characters start at '!' (33) - allows letters, numbers, punctuation
		if c == '/' || c == '\\' || c < '!' {
			return false
		}
	}
	// Check for scheme at the beginning (before any path component)
	// We only check up to the first path segment to allow query params containing URLs
	firstSlash := strings.Index(path[1:], "/")
	pathToCheck := path
	if firstSlash > 0 {
		pathToCheck = path[:firstSlash+1]
	}
	if strings.Contains(pathToCheck, "://") {
		return false
	}
	return true
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
