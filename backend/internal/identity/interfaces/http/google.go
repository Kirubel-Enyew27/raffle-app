package http

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/raffle-app/backend/internal/identity/application"
	apperrors "github.com/raffle-app/backend/pkg/errors"
)

// GoogleAuthHandler exposes the OAuth entry points. It deliberately does not
// know about tokens: everything sensitive stays server-side and is handed to
// the SPA through a one-time code.
type GoogleAuthHandler struct {
	service *application.IdentityService
	// frontendURL is where the browser is sent back to after the callback.
	frontendURL string
}

func NewGoogleAuthHandler(service *application.IdentityService, frontendURL string) *GoogleAuthHandler {
	return &GoogleAuthHandler{
		service:     service,
		frontendURL: strings.TrimRight(frontendURL, "/"),
	}
}

// Providers lets the SPA decide whether to render the Google button at all,
// instead of offering a button that cannot work.
func (h *GoogleAuthHandler) Providers(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": "success", "data": gin.H{
		"google": h.service.GoogleEnabled(),
	}})
}

// Start begins the authorization-code flow by redirecting to Google.
func (h *GoogleAuthHandler) Start(c *gin.Context) {
	redirectURL, err := h.service.StartGoogleLogin(c.Request.Context(), c.Query("redirect"))
	if err != nil {
		if errors.Is(err, application.ErrGoogleUnavailable) {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"code":    "GOOGLE_NOT_CONFIGURED",
				"message": "Google sign-in is not configured on this server",
			})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    "GOOGLE_UNAVAILABLE",
			"message": "Google sign-in is temporarily unavailable, please try again",
		})
		return
	}

	// The URL contains only the public client id plus our opaque state.
	c.Header("Cache-Control", "no-store")
	c.Redirect(http.StatusFound, redirectURL)
}

// LinkStatus reports whether the signed-in account has Google connected.
func (h *GoogleAuthHandler) LinkStatus(c *gin.Context) {
	linked, err := h.service.GoogleLinkStatus(c.Request.Context(), c.GetString("user_id"))
	if err != nil {
		if errors.Is(err, application.ErrGoogleUnavailable) {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"code":    "GOOGLE_NOT_CONFIGURED",
				"message": "Google sign-in is not configured on this server",
			})
			return
		}
		if errors.Is(err, apperrors.ErrUnauthorized) {
			c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "unauthorized"})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    "GOOGLE_UNAVAILABLE",
			"message": "Google sign-in is temporarily unavailable, please try again",
		})
		return
	}

	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"code": "success", "data": gin.H{"linked": linked}})
}

// StartLink begins connecting Google to the signed-in account. The session
// alone is not enough: the caller must re-enter the account password.
func (h *GoogleAuthHandler) StartLink(c *gin.Context) {
	var req struct {
		Password string `json:"password" binding:"required"`
		Redirect string `json:"redirect"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_FAILED", "message": "your password is required"})
		return
	}

	linkURL, err := h.service.StartGoogleLink(
		c.Request.Context(),
		c.GetString("user_id"),
		req.Password,
		req.Redirect,
	)
	if err != nil {
		switch {
		case errors.Is(err, application.ErrInvalidPassword):
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    "INVALID_PASSWORD",
				"message": "the password you entered is incorrect",
			})
		case errors.Is(err, application.ErrGoogleAlreadyLinked):
			c.JSON(http.StatusConflict, gin.H{
				"code":    "ALREADY_LINKED",
				"message": "a google account is already connected to this account",
			})
		case errors.Is(err, application.ErrGoogleLinkEmailRequired):
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    "EMAIL_REQUIRED",
				"message": "add an email address to your account before connecting google",
			})
		case errors.Is(err, apperrors.ErrValidationFailed):
			c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_FAILED", "message": "your password is required"})
		case errors.Is(err, apperrors.ErrUnauthorized):
			c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "unauthorized"})
		case errors.Is(err, apperrors.ErrForbidden):
			c.JSON(http.StatusForbidden, gin.H{"code": "FORBIDDEN", "message": "this account is not available"})
		case errors.Is(err, application.ErrGoogleUnavailable):
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"code":    "GOOGLE_NOT_CONFIGURED",
				"message": "Google sign-in is not configured on this server",
			})
		default:
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"code":    "GOOGLE_UNAVAILABLE",
				"message": "Google sign-in is temporarily unavailable, please try again",
			})
		}
		return
	}

	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"code": "success", "data": gin.H{"url": linkURL}})
}

// Callback validates Google's response and bounces the browser back to the SPA.
func (h *GoogleAuthHandler) Callback(c *gin.Context) {
	result, err := h.service.CompleteGoogleCallback(
		c.Request.Context(),
		c.Query("state"),
		c.Query("code"),
		c.Query("error"),
	)
	linkMode := result != nil && result.LinkMode

	if err != nil {
		reason := url.QueryEscape(googleErrorCode(err))
		if linkMode {
			h.redirectToFrontend(c, "/profile?google="+reason)
		} else {
			h.redirectToFrontend(c, "/login?error="+reason)
		}
		return
	}

	if linkMode {
		// No session was minted: the user is already signed in.
		h.redirectToFrontend(c, "/profile?google=linked")
		return
	}

	// The code travels in the fragment so it is never sent to a server or a
	// proxy log, and never leaks through a Referer header.
	target := "/auth/google/callback#code=" + url.QueryEscape(result.HandoffCode)
	if result.RedirectTo != "" {
		target += "&redirect=" + url.QueryEscape(result.RedirectTo)
	}
	h.redirectToFrontend(c, target)
}

// Exchange redeems the one-time code for the session created by the callback.
func (h *GoogleAuthHandler) Exchange(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_FAILED", "message": "a sign-in code is required"})
		return
	}

	session, redirectTo, err := h.service.ExchangeGoogleHandoff(c.Request.Context(), req.Code)
	if err != nil {
		switch {
		case errors.Is(err, application.ErrOAuthStateInvalid), errors.Is(err, application.ErrGoogleUnavailable):
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    "GOOGLE_SIGN_IN_EXPIRED",
				"message": "this sign-in attempt has expired, please try again",
			})
		default:
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"code":    "GOOGLE_UNAVAILABLE",
				"message": "Google sign-in is temporarily unavailable, please try again",
			})
		}
		return
	}

	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"code": "success", "data": gin.H{
		"token":         session.AccessToken,
		"refresh_token": session.RefreshToken,
		"expires_in":    session.ExpiresIn,
		"user":          session.User,
		"redirect":      redirectTo,
	}})
}

func (h *GoogleAuthHandler) redirectToFrontend(c *gin.Context, target string) {
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	c.Redirect(http.StatusFound, h.frontendURL+target)
}

// googleErrorCode maps internal failures onto the small set of reasons the SPA
// knows how to explain. Nothing implementation-specific is exposed.
func googleErrorCode(err error) string {
	switch {
	case errors.Is(err, application.ErrGoogleDeclined):
		return "access_denied"
	case errors.Is(err, application.ErrGoogleAccountExists):
		return "account_exists"
	case errors.Is(err, application.ErrGoogleEmailUnverified):
		return "email_unverified"
	case errors.Is(err, application.ErrGoogleLinkEmailMismatch):
		return "link_email_mismatch"
	case errors.Is(err, application.ErrGoogleLinkEmailRequired):
		return "email_required"
	case errors.Is(err, application.ErrGoogleAlreadyLinked):
		return "already_linked"
	case errors.Is(err, application.ErrGoogleLinkConflict):
		return "link_conflict"
	case errors.Is(err, application.ErrOAuthStateInvalid):
		return "invalid_state"
	case errors.Is(err, application.ErrGoogleUnavailable):
		return "google_unavailable"
	case errors.Is(err, application.ErrGoogleAuthFailed):
		return "google_failed"
	case errors.Is(err, apperrors.ErrForbidden):
		return "account_disabled"
	case errors.Is(err, apperrors.ErrInternal):
		return "temporarily_unavailable"
	default:
		return "google_failed"
	}
}
