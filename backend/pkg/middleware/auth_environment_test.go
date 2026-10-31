package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func setAuthTestAppEnv(t *testing.T, value string) {
	t.Helper()
	t.Setenv("APP_ENV", value)
	previous := viper.Get("APP_ENV")
	viper.Set("APP_ENV", value)
	t.Cleanup(func() {
		if previous == nil {
			viper.Set("APP_ENV", "")
			return
		}
		viper.Set("APP_ENV", previous)
	})
}

func TestMagicAdminTokenIsDevelopmentOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousSecret := viper.Get("JWT_SECRET")
	viper.Set("JWT_SECRET", "test-secret")
	t.Cleanup(func() {
		if previousSecret == nil {
			viper.Set("JWT_SECRET", "")
			return
		}
		viper.Set("JWT_SECRET", previousSecret)
	})
	router := gin.New()
	router.GET("/admin", AuthMiddleware(), AdminMiddleware(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	setAuthTestAppEnv(t, "")
	request := httptest.NewRequest(http.MethodGet, "/admin", nil)
	request.Header.Set("Authorization", "Bearer admin")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unset APP_ENV accepted the development admin token: status=%d", response.Code)
	}

	setAuthTestAppEnv(t, "production")
	request = httptest.NewRequest(http.MethodGet, "/admin", nil)
	request.Header.Set("Authorization", "Bearer admin")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("production accepted the development admin token: status=%d", response.Code)
	}

	setAuthTestAppEnv(t, "development")
	request = httptest.NewRequest(http.MethodGet, "/admin", nil)
	request.Header.Set("Authorization", "Bearer admin")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("development rejected its test admin token: status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestOptionalAuthDoesNotGrantAdminInProduction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setAuthTestAppEnv(t, "production")
	router := gin.New()
	router.GET("/optional", OptionalAuthMiddleware(), func(c *gin.Context) {
		c.String(http.StatusOK, c.GetString("role"))
	})

	request := httptest.NewRequest(http.MethodGet, "/optional", nil)
	request.Header.Set("Authorization", "Bearer admin")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.Len() != 0 {
		t.Fatalf("invalid production token unexpectedly authenticated: status=%d role=%q", response.Code, response.Body.String())
	}
}
