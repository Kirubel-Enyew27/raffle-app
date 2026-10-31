package realtime

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func TestStreamRejectsDevelopmentAdminTokenInProduction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("APP_ENV", "production")
	previous := viper.Get("APP_ENV")
	viper.Set("APP_ENV", "production")
	t.Cleanup(func() {
		if previous == nil {
			viper.Set("APP_ENV", "")
			return
		}
		viper.Set("APP_ENV", previous)
	})

	service := NewService(nil, zap.NewNop())
	router := gin.New()
	router.GET("/stream", NewHandler(service, "test-secret", zap.NewNop()).Stream)
	request := httptest.NewRequest(http.MethodGet, "/stream?token=admin&channel=global", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("production SSE accepted the development admin token: status=%d", response.Code)
	}
	if service.ConnectedCount() != 0 {
		t.Fatal("rejected token still opened an SSE connection")
	}
}
