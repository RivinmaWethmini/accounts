package connections

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/joho/godotenv"
	"github.com/sliitmozilla/accounts/app/middlewares"
	"github.com/sliitmozilla/accounts/config"
	"github.com/sliitmozilla/accounts/db"
	"github.com/sliitmozilla/accounts/db/models"
	apiErrors "github.com/sliitmozilla/accounts/errors"
	"github.com/sliitmozilla/accounts/helpers"
)

const (
	GoogleAuthURL        = "https://accounts.google.com/o/oauth2/v2/auth"
	OAuthStateKeyPrefix  = "accounts:oidc:state:"
	OAuthStateExpiration = 5 * time.Minute
)

type GoogleOAuthStatePayload struct {
	CodeVerifier string `json:"codeVerifier"`
	UserID       string `json:"userId,omitempty"`
	Redirect     string `json:"redirect,omitempty"`
}

type GoogleCallbackRequestBody struct {
	Code  string `json:"code"`
	State string `json:"state"`
}

// @tags        Connections
// @summary     Initiate Google OpenID Connect / OAuth 2.0 PKCE flow
// @description Generates a secure authorization URL with PKCE S256 challenge and anti-CSRF state token.
// @accept      json
// @produce     json
// @param       redirect query string false "Destination path after successful authentication"
// @success     200 {object} helpers.SuccessResponseModel{data=map[string]string} "Authorization URL and state"
// @failure     500 "Internal Server Error"
// @router      /connections/google/url [GET]
func GetGoogleAuthURL(w http.ResponseWriter, r *http.Request) {
	godotenv.Overload()

	clientID := strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_ID"))
	if clientID == "" {
		helpers.Response(w, http.StatusBadRequest, "Google OAuth is not configured. Please set GOOGLE_CLIENT_ID in your .env file.")
		return
	}

	redirectURI := strings.TrimSpace(os.Getenv("GOOGLE_REDIRECT_URI"))
	if redirectURI == "" {
		redirectURI = "http://localhost:3000/auth/google/callback"
	}

	verifier, err := helpers.GenerateCodeVerifier()
	if err != nil {
		log.Println("Error generating PKCE code verifier:", err)
		helpers.Response(w, http.StatusInternalServerError, "Internal Server Error")
		return
	}

	challenge := helpers.ComputeCodeChallenge(verifier)

	state, err := helpers.GenerateStateToken()
	if err != nil {
		log.Println("Error generating OAuth state token:", err)
		helpers.Response(w, http.StatusInternalServerError, "Internal Server Error")
		return
	}

	var userID string
	if ctxUser, ok := r.Context().Value(middlewares.UserContext{}).(*models.UserModel); ok && ctxUser != nil {
		userID = ctxUser.ID.String()
	}

	redirectParam := strings.TrimSpace(r.URL.Query().Get("redirect"))

	statePayload := GoogleOAuthStatePayload{
		CodeVerifier: verifier,
		UserID:       userID,
		Redirect:     redirectParam,
	}

	payloadJSON, err := json.Marshal(statePayload)
	if err != nil {
		log.Println("Error marshaling state payload:", err)
		helpers.Response(w, http.StatusInternalServerError, "Internal Server Error")
		return
	}

	redisClient := db.ConnectRedis()
	defer redisClient.Close()

	stateKey := OAuthStateKeyPrefix + state
	if err := redisClient.SetEx(r.Context(), stateKey, payloadJSON, OAuthStateExpiration).Err(); err != nil {
		log.Println("Error caching OAuth state in Redis:", err)
		helpers.Response(w, http.StatusInternalServerError, "Internal Server Error")
		return
	}

	params := url.Values{}
	params.Set("client_id", clientID)
	params.Set("response_type", "code")
	params.Set("redirect_uri", redirectURI)
	params.Set("scope", "openid email profile")
	params.Set("state", state)
	params.Set("code_challenge", challenge)
	params.Set("code_challenge_method", "S256")
	params.Set("prompt", "select_account")

	authURL := fmt.Sprintf("%s?%s", GoogleAuthURL, params.Encode())

	helpers.Response(w, http.StatusOK, map[string]string{
		"url":   authURL,
		"state": state,
	})
}

// @tags        Connections
// @summary     Handles the Google OAuth 2.0 / OIDC callback
// @description Verifies PKCE and state token, exchanges code for ID token via TLS, validates claims and establishes session.
// @accept      json
// @produce     json
// @param       request body GoogleCallbackRequestBody false "Callback request body with code and state"
// @success     200 {object} helpers.SuccessResponseModel{data=map[string]interface{}} "Session tokens"
// @failure     400 "Invalid state or credentials"
// @failure     500 "Internal Server Error"
// @router      /connections/google/callback [POST]
func CallbackGoogle(w http.ResponseWriter, r *http.Request) {
	godotenv.Overload()

	clientID := strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_ID"))
	clientSecret := strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_SECRET"))
	redirectURI := strings.TrimSpace(os.Getenv("GOOGLE_REDIRECT_URI"))
	if redirectURI == "" {
		redirectURI = "http://localhost:3000/auth/google/callback"
	}

	var code, state string

	if r.Method == http.MethodGet {
		code = strings.TrimSpace(r.URL.Query().Get("code"))
		state = strings.TrimSpace(r.URL.Query().Get("state"))
	} else {
		var body GoogleCallbackRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			code = strings.TrimSpace(body.Code)
			state = strings.TrimSpace(body.State)
		}
	}

	if code == "" || state == "" {
		helpers.Response(w, http.StatusBadRequest, "Missing code or state parameter")
		return
	}

	redisClient := db.ConnectRedis()
	defer redisClient.Close()

	stateKey := OAuthStateKeyPrefix + state
	cachedVal, err := redisClient.Get(r.Context(), stateKey).Result()
	if err != nil {
		helpers.Response(w, http.StatusBadRequest, "Invalid, expired, or previously used OAuth state parameter")
		return
	}

	// Strictly delete key immediately to prevent replay attacks
	redisClient.Del(r.Context(), stateKey)

	var statePayload GoogleOAuthStatePayload
	if err := json.Unmarshal([]byte(cachedVal), &statePayload); err != nil {
		log.Println("Error unmarshaling cached state payload:", err)
		helpers.Response(w, http.StatusInternalServerError, "Internal Server Error")
		return
	}

	// Perform server-to-server TLS code exchange
	tokenResp, err := helpers.ExchangeGoogleCode(
		r.Context(),
		code,
		statePayload.CodeVerifier,
		clientID,
		clientSecret,
		redirectURI,
	)
	if err != nil {
		log.Println("Google code exchange error:", err)
		helpers.Response(w, http.StatusBadRequest, "Failed to exchange authorization code with Google")
		return
	}

	// Cryptographically verify ID token signature and claims
	claims, err := helpers.VerifyGoogleIDToken(r.Context(), tokenResp.IDToken, clientID)
	if err != nil {
		log.Println("Google ID token verification error:", err)
		helpers.Response(w, http.StatusUnauthorized, "Invalid Google ID token")
		return
	}

	c := config.GetConfig()

	// Resolve or Provision User
	var targetUser *models.UserModel

	// Check if already linked by Google sub
	existingLinkedUser, err := models.UserModel{}.GetUserByProvider("google", claims.Subject)
	if statePayload.UserID != "" {
		// Explicit linking initiated while authenticated
		parsedUUID := uuid.FromStringOrNil(statePayload.UserID)
		if parsedUUID == uuid.Nil {
			helpers.Response(w, http.StatusBadRequest, "Invalid session user ID")
			return
		}
		userByID, err := models.UserModel{}.GetUserByID(parsedUUID)
		if err != nil {
			helpers.Response(w, http.StatusNotFound, "User not found")
			return
		}

		// Security Check 1: Is this Google account already linked to another user?
		if existingLinkedUser != nil && existingLinkedUser.ID != userByID.ID {
			helpers.Response(w, http.StatusConflict, "This Google account is already linked to another user account")
			return
		}

		// Security Check 2: Does the Google email belong to another existing user?
		existingEmailUser, err := models.UserModel{}.GetUserByEmail(claims.Email)
		if err == nil && existingEmailUser != nil && existingEmailUser.ID != userByID.ID {
			helpers.Response(w, http.StatusConflict, "The email associated with this Google account is already registered to a different account")
			return
		}

		targetUser = &userByID

		conn := models.ConnectionModel{
			UserId:               targetUser.ID.String(),
			Provider:             "google",
			ProviderUserId:       claims.Subject,
			ProviderUserName:     claims.Name,
			ProviderAccountEmail: claims.Email,
		}
		if _, err := conn.Insert(); err != nil {
			if _, ok := err.(apiErrors.DuplicateError); !ok {
				log.Println("Error linking Google connection:", err)
				helpers.Response(w, http.StatusInternalServerError, "Failed to link Google account")
				return
			}
		}
	} else if err == nil && existingLinkedUser != nil {
		targetUser = existingLinkedUser
	} else {
		// Federated Login or Provisioning:
		// Safe Account Linking: Check if an account already exists with this verified email
		existingEmailUser, err := models.UserModel{}.GetUserByEmail(claims.Email)
		if err == nil && existingEmailUser != nil {
			targetUser = existingEmailUser
			conn := models.ConnectionModel{
				UserId:               targetUser.ID.String(),
				Provider:             "google",
				ProviderUserId:       claims.Subject,
				ProviderUserName:     claims.Name,
				ProviderAccountEmail: claims.Email,
			}
			if _, err := conn.Insert(); err != nil {
				if _, ok := err.(apiErrors.DuplicateError); !ok {
					log.Println("Error linking Google connection to existing user:", err)
				}
			}
		} else {
			// Provision a new user
			cleanName := sanitizeUsername(claims.Name, claims.Email)
			newUser, err := models.UserModel{}.CreateFederatedUser(cleanName, claims.Email)
			if err != nil {
				log.Println("Error provisioning federated user:", err)
				helpers.Response(w, http.StatusInternalServerError, "Failed to create federated user")
				return
			}
			targetUser = newUser
			conn := models.ConnectionModel{
				UserId:               targetUser.ID.String(),
				Provider:             "google",
				ProviderUserId:       claims.Subject,
				ProviderUserName:     claims.Name,
				ProviderAccountEmail: claims.Email,
			}
			if _, err := conn.Insert(); err != nil {
				log.Println("Error linking Google connection to new user:", err)
			}
		}
	}

	// Issue application session tokens
	accessToken, refreshToken, err := helpers.GenerateTokens(
		targetUser.ID.String(),
		targetUser.Name,
		targetUser.Email,
		targetUser.Roles,
	)
	if err != nil {
		log.Println("Error generating JWT tokens:", err)
		helpers.Response(w, http.StatusInternalServerError, "Failed to generate session tokens")
		return
	}

	// Hardened session cookie
	secureCookie := r.TLS != nil || os.Getenv("ENV") == "production"
	http.SetCookie(w, &http.Cookie{
		Name:     "refreshToken",
		Value:    refreshToken,
		HttpOnly: true,
		Secure:   secureCookie,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(c.Lifespan.RefreshToken * time.Second),
		Path:     "/",
	})

	if r.Method == http.MethodGet {
		dest := statePayload.Redirect
		if dest == "" {
			dest = "/profile"
		}
		http.Redirect(w, r, dest, http.StatusTemporaryRedirect)
		return
	}

	helpers.Response(w, http.StatusOK, map[string]interface{}{
		"token": accessToken,
		"user": map[string]interface{}{
			"id":    targetUser.ID,
			"name":  targetUser.Name,
			"email": targetUser.Email,
			"roles": targetUser.Roles,
		},
	})
}

// @tags        Connections
// @summary     Unlink Google account from authenticated user
// @description Unlink Google account from authenticated user
// @security    AccessToken
// @accept      json
// @produce     json
// @success     200 "OK"
// @failure     401 "Not logged in or invalid token"
// @failure     404 "User or provider not found"
// @failure     500 "Internal Server Error"
// @router      /connections/google [DELETE]
func UnlinkGoogle(w http.ResponseWriter, r *http.Request) {
	ctxUser := r.Context().Value(middlewares.UserContext{}).(*models.UserModel)
	c := models.ConnectionModel{
		UserId:   ctxUser.ID.String(),
		Provider: "google",
	}
	rows, err := c.Delete()
	if rows == 0 {
		helpers.Response(w, http.StatusNotFound, "Google connection not found")
		return
	}
	if err != nil {
		log.Println("Error unlinking Google connection:", err)
		helpers.Response(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return
	}
	helpers.Response(w, http.StatusOK, "Google account unlinked successfully")
}

func sanitizeUsername(name, email string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		parts := strings.Split(email, "@")
		name = parts[0]
	}
	// Replace spaces and special characters
	re := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	cleaned := re.ReplaceAllString(name, "")
	if len(cleaned) < 3 {
		cleaned = fmt.Sprintf("user_%d", time.Now().Unix()%10000)
	}
	if len(cleaned) > 28 {
		cleaned = cleaned[:28]
	}
	return cleaned
}
