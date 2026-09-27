package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/sliitmozilla/accounts/app/middlewares"
	"github.com/sliitmozilla/accounts/config"
	"github.com/sliitmozilla/accounts/db"
	"github.com/sliitmozilla/accounts/db/models"
	apiErrors "github.com/sliitmozilla/accounts/errors"
	"github.com/sliitmozilla/accounts/helpers"
)

// @tags        Auth
// @summary     Get current session
// @description Get the current session using the access token
// @accept      json
// @produce     json
// @security	AccessToken
// @success 	200 {object} helpers.SuccessResponseModel{data=object{id=string,roles=[]string}} "Session data"
// @failure		401 "Not logged in or invalid token"
// @failure     500 "Internal Server error"
// @router      /session [GET]
func GetSession(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(middlewares.UserContext{}).(*models.UserModel)
	helpers.Response(w, http.StatusOK, map[string]any{
		"id":    u.ID.String(),
		"roles": u.Roles,
	})
}

func createAndStoreCode(id string) ([]byte, error) {
	c := config.GetConfig()

	code := make([]byte, 10)
	rand.Read(code)
	redisClient := db.ConnectRedis()
	defer redisClient.Close()

	s := redisClient.SetEx(
		context.Background(),
		"accounts:code:"+base64.URLEncoding.EncodeToString(code),
		id,
		time.Duration(c.Lifespan.AuthorizationCode*time.Second),
	)

	return code, s.Err()
}

// @tags        Auth
// @summary		Initiate the authentication flow
// @description Initiate the authentication flow with the auth service. \
// @description	Any external service should visit this route with a valid redirect
// @description	If the user is already logged in with the auth service, the auth \
// @description	service will redirect the user back to the provided url with a \
// @description	short lived temporary token (1 minute lifespan) - \
// @description that should be used to complete the authentication
// @param		redirect query string true "URL encoded redirect url" example(http://localhost:3001/callback)
// @success     302 "Redirect to the provided URL with temporary code in query param 'code'"
// @failure     400 "Bad Request - invalid redirect URL"
// @failure     500 "Internal Server Error"
// @router      /authorize [GET]
func Authorize(w http.ResponseWriter, r *http.Request) {
	redirect := r.URL.Query().Get("redirect")
	redirect_uri, err := url.ParseRequestURI(redirect)
	if err != nil {
		helpers.Response(w, http.StatusBadRequest, err.Error())
		return
	}
	if redirect_uri.Scheme != "http" && redirect_uri.Scheme != "https" {
		helpers.Response(w, http.StatusBadRequest, "redirect must use http or https")
		return
	}

	refreshToken, err := r.Cookie("refreshToken")
	if err != nil {
		http.Redirect(w, r, "/login?redirect="+r.URL.String(), http.StatusTemporaryRedirect)
		return
	}
	claims, err := helpers.GetClaimsFromToken(refreshToken.Value)
	if err != nil {
		log.Println(err.Error())
		helpers.Response(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return
	}
	userID := claims["id"].(string)

	consentModel := models.ConsentedRedirectModel{UserID: userID, Host: redirect_uri.Host}
	consented, err := consentModel.Exists()
	if err != nil {
		log.Println(err.Error())
		helpers.Response(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return
	}

	if !consented {
		// First time seeing this host for this user — send them to the
		// frontend confirmation page instead of redirecting silently.
		confirmURL := "/redirect/confirm?redirect=" + url.QueryEscape(redirect)
		http.Redirect(w, r, confirmURL, http.StatusTemporaryRedirect)
		return
	}

	issueCodeAndRedirect(w, r, userID, redirect_uri)
}

// @tags        Auth
// @summary     Confirm redirect and complete the authentication flow
// @description Called after the user explicitly confirms a new/unrecognized \
// @description redirect destination on the frontend confirmation page. \
// @description Records consent for future silent redirects to the same host, \
// @description then completes the authorization flow.
// @param       redirect query string true "URL encoded redirect url"
// @success     302 "Redirect to the provided URL with temporary code in query param 'code'"
// @failure     400 "Bad Request - invalid redirect URL"
// @failure     401 "Not logged in"
// @failure     500 "Internal Server Error"
// @router      /authorize/confirm [GET]
func AuthorizeConfirm(w http.ResponseWriter, r *http.Request) {
	redirect := r.URL.Query().Get("redirect")
	redirect_uri, err := url.ParseRequestURI(redirect)
	if err != nil {
		helpers.Response(w, http.StatusBadRequest, err.Error())
		return
	}
	if redirect_uri.Scheme != "http" && redirect_uri.Scheme != "https" {
		helpers.Response(w, http.StatusBadRequest, "redirect must use http or https")
		return
	}

	refreshToken, err := r.Cookie("refreshToken")
	if err != nil {
		helpers.Response(w, http.StatusUnauthorized, "not logged in")
		return
	}
	claims, err := helpers.GetClaimsFromToken(refreshToken.Value)
	if err != nil {
		log.Println(err.Error())
		helpers.Response(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return
	}
	userID := claims["id"].(string)

	consentModel := models.ConsentedRedirectModel{UserID: userID, Host: redirect_uri.Host}
	if _, err := consentModel.Insert(); err != nil {
		log.Println(err.Error())
		helpers.Response(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return
	}

	issueCodeAndRedirect(w, r, userID, redirect_uri)
}

// issueCodeAndRedirect generates a short-lived auth code and redirects the
// browser to the given redirect_uri with the code attached.
func issueCodeAndRedirect(w http.ResponseWriter, r *http.Request, userID string, redirect_uri *url.URL) {
	code, err := createAndStoreCode(userID)
	if err != nil {
		helpers.Response(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return
	}
	query := redirect_uri.Query()
	query.Add("code", base64.URLEncoding.EncodeToString(code))
	redirect_uri.RawQuery = query.Encode()
	http.Redirect(w, r, redirect_uri.String(), http.StatusTemporaryRedirect)
}

// @tags        Auth
// @summary		Get tokens from code
// @description This is the second step of the authorization flow \
// @description Check GET /api/authorize for more information about the first step of this flow.
// @description The client should invoke this endpoint with the code received from the previous step.
// @description The client shall receive a pair of access token (found in response body) \
// @description and refresh token (found in cookie: refreshToken) after completing all the steps.
// @description Client can act on behalf of the user after receiving the token pair
// @accept      json
// @produce     json
// @param		code query string true "URL encoded code received from previous step" example(abcdefgh)
// @success 200 {object} helpers.SuccessResponseModel{data=object{token=string}} "Access token returned in response body; refresh token is set in cookie 'refreshToken'"
// @failure 	401 "If token is invalid or expired"
// @failure 	404 "Token is related to a non-existing user"
// @failure     500 "Internal Server error"
// @router      /token [POST]
func GetToken(w http.ResponseWriter, r *http.Request) {
	c := config.GetConfig()
	code := r.URL.Query().Get("code")
	redisClient := db.ConnectRedis()
	res := redisClient.Get(context.Background(), "accounts:code:"+code)
	if res.Err() != nil {
		if res.Err() == redis.Nil {
			helpers.Response(w, http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
			return
		}
		log.Println(res.Err())
		helpers.Response(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return
	}
	id := uuid.FromStringOrNil(res.Val())
	if id == uuid.Nil {
		// this should never happen
		helpers.Response(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return
	}
	u, err := models.UserModel{}.GetUserByID(id)
	if err != nil {
		if ve, ok := err.(apiErrors.NotFoundError); ok {
			helpers.Response(w, http.StatusNotFound, ve.Error())
			return
		}
		log.Println(err)
		helpers.Response(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	accessToken, refreshToken, err := helpers.GenerateTokens(id.String(), u.Name, u.Email, u.Roles)
	if err != nil {
		log.Println(err.Error())
		helpers.Response(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "refreshToken",
		Value:    refreshToken,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(c.Lifespan.RefreshToken * time.Second),
		Path:     "/",
	})
	helpers.Response(w, http.StatusOK, map[string]string{"token": accessToken})
}

type LoginRequestBody struct {
	Email    string `json:"email" example:"infosliitmcc@gmail.com" validate:"required,email"`
	Password string `json:"password" example:"password" validate:"required"`
}

// @tags        Auth
// @summary     Login user
// @description Endpoint to log in a user with email and password
// @description Upon successful login user receives a pair of access token (found in response body) \
// @description and refresh token (found in cookie: refreshToken)
// @accept      json
// @produce     json
// @param       request body LoginRequestBody true "Request body"
// @success 	200 {object} helpers.SuccessResponseModel{data=object{token=string}} "Access token returned in response body; refresh token is set in cookie 'refreshToken'"
// @failure     400 "Invalid request body"
// @failure     401 "Invalid credentials"
// @failure     429 "Too many login attempts"
// @failure     500 "Internal Server error"
// @router      /login [POST]
func Login(w http.ResponseWriter, r *http.Request) {
	c := config.GetConfig()
	defer r.Body.Close()

	// Extract client IP address for rate limiting
	clientIP := r.RemoteAddr
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		clientIP = strings.TrimSpace(strings.Split(forwarded, ",")[0])
	} else if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		clientIP = strings.TrimSpace(realIP)
	} else if host, _, err := net.SplitHostPort(clientIP); err == nil {
		clientIP = host
	}

	redisClient := db.ConnectRedis()
	defer redisClient.Close()

	rateLimitKey := "accounts:ratelimit:login:" + clientIP
	attempts, err := redisClient.Incr(r.Context(), rateLimitKey).Result()
	if err == nil {
		if attempts == 1 {
			redisClient.Expire(r.Context(), rateLimitKey, 1*time.Minute)
		}
		if attempts > 5 {
			ttl, _ := redisClient.TTL(r.Context(), rateLimitKey).Result()
			retryAfter := int(ttl.Seconds())
			if retryAfter < 1 {
				retryAfter = 60
			}
			w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
			helpers.Response(w, http.StatusTooManyRequests, "Too many login attempts. Please try again later.")
			return
		}
	}

	var requestBody LoginRequestBody

	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		helpers.Response(w, http.StatusBadRequest, "Invalid or empty body")
		return
	}

	errs := helpers.Validate(requestBody)
	if errs != nil {
		fmt.Println(errs)
		helpers.Response(w, http.StatusBadRequest, errs)
		return
	}

	accessToken, refreshToken, err := models.UserModel{}.Login(requestBody.Email, requestBody.Password)
	if err != nil {
		if _, ok := err.(apiErrors.NotFoundError); ok {
			helpers.Response(w, http.StatusUnauthorized, "Invalid credentials")
			return
		}
		if _, ok := err.(apiErrors.UnverifiedEmailError); ok {
			helpers.Response(w, http.StatusForbidden, err.Error())
			return
		}
		log.Println(err.Error())
		helpers.Response(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return
	}
	if accessToken == "" || refreshToken == "" {
		helpers.Response(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	// Reset failed attempts counter on successful login
	redisClient.Del(r.Context(), rateLimitKey)

	http.SetCookie(w, &http.Cookie{
		Name:     "refreshToken",
		Value:    refreshToken,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(c.Lifespan.RefreshToken * time.Second),
		Path:     "/",
	})
	helpers.Response(w, http.StatusOK, map[string]string{"token": accessToken})
}

// @tags        Auth
// @summary     Logout user
// @description Logout user. Clears the refreshToken cookie
// @accept      json
// @produce     json
// @success     200 {object} helpers.SuccessResponseModel "Logout successful"
// @failure     500 "Internal Server Error"
// @router      /logout [POST]
func Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refreshToken",
		Value:    "",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})
	helpers.Response(w, http.StatusOK, http.StatusText(http.StatusOK))
}

func getAccessTokenFromRefresh(refreshToken string) (string, error) {
	claims, err := helpers.GetClaimsFromToken(refreshToken)
	if err != nil {
		return "", errors.New("invalid or expired token")
	}
	typ, ok := claims["typ"].(string)
	if !ok || typ != "refresh" {
		return "", errors.New("invalid token: expected refresh token")
	}
	id := uuid.FromStringOrNil(claims["id"].(string))
	if id == uuid.Nil {
		return "", errors.New("invalid token")
	}

	u, err := models.UserModel{}.GetUserByID(id)
	if err != nil {
		return "", err
	}

	accessToken, _, err := helpers.GenerateTokens(u.ID.String(), u.Name, u.Email, u.Roles)
	return accessToken, err
}

// @tags        Auth
// @summary     Refresh acess token
// @description Refresh the access token with the refresh token
// @accept      json
// @produce     json
// @success 	200 {object} helpers.SuccessResponseModel{data=object{token=string}} "Access token"
// @failure		401 "Invalid or missing refresh token"
// @failure     500 "Internal Server Error"
// @router      /token/refresh [POST]
func RefreshToken(w http.ResponseWriter, r *http.Request) {
	token, err := r.Cookie("refreshToken")
	if err != nil {
		if err == http.ErrNoCookie {
			helpers.Response(w, http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
			return
		}
		log.Println(err.Error())
		helpers.Response(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return
	}

	accessToken, err := getAccessTokenFromRefresh(token.Value)
	if err != nil {
		if _, ok := err.(apiErrors.NotFoundError); ok {
			helpers.Response(w, http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
			return
		}
		helpers.Response(w, http.StatusUnauthorized, err.Error())
		return
	}
	helpers.Response(w, http.StatusOK, map[string]string{"token": accessToken})
}
