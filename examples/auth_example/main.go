package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"corelab/pkg/apperr"
	"corelab/pkg/config"
	"corelab/pkg/i18n"
	"corelab/pkg/logger"
	"corelab/pkg/response"
	"corelab/pkg/server"
	smw "corelab/pkg/server/middleware"
	"corelab/pkg/utils"
	"corelab/pkg/validator"
	"corelab/pkg/version"

	authmw "corelab/pkg/auth/middleware"
	"corelab/pkg/auth/openfga"
)

func main() {
	// Config and logger
	cfg := config.New(
		config.WithDefaults(map[string]any{
			"service.endpoint": "0.0.0.0",
			"service.port":     "8080",
		}),
		config.WithEnv("APP"),
		config.WithAutoPFlags(),
	)
	log := logger.MustNewDefaultLogger()
	log.InfoF("starting example server", "version", version.Info())

	// Translator with JSON bundles under examples/auth_example/locales
	tr := i18n.New(
		i18n.WithDefaultLocale("fr"),
		i18n.WithFallbackLocales("fr"),
		i18n.WithJSONDir("default", "./examples/auth_example/locales"),
	)

	// Server engine with CORS, rate limit, recovery
	rl := smw.NewRateLimitConfig(true, 5, 10, time.Minute)
	engine := server.NewEngine(
		server.WithLogger(log),
		server.WithRecovery(true),
		server.WithCors(smw.DefaultCorsConfig()),
		server.WithRateLimit(rl),
		server.WithPrometheus(false),
	)

	// i18n middleware
	engine.Use(tr.GinMiddleware())

	// JWT auth (configure via env): set JWKS URL/Issuer/Audience for your IdP
	jwtCfg := authmw.JWTConfig{
		Issuer:      os.Getenv("APP_JWT_ISSUER"),
		Audience:    os.Getenv("APP_JWT_AUDIENCE"),
		JWKSURL:     os.Getenv("APP_JWT_JWKSURL"),
		CacheTTL:    10 * time.Minute,
		AllowedAlgs: []string{"RS256"},
	}
	// Memory authorizer example (for tests/demos)
	authz := openfga.NewMemoryAuthorizer()
	authz.Allow("user:demo", "viewer", "doc:1")

	// Create a validator instance for binding helpers
	vi := validator.New()

	// Routes
	engine.GET("/healthz", func(c *gin.Context) {
		response.Success(c, gin.H{"status": "ok"})
	})
	engine.GET("/version", func(c *gin.Context) {
		response.Success(c, version.Info())
	})

	// i18n example
	engine.GET("/i18n/hello", func(c *gin.Context) {
		loc := c.GetString("i18n_locale")
		msg := tr.T(loc, "greeting", map[string]any{"name": "world"})
		c.JSON(http.StatusOK, gin.H{"locale": loc, "message": msg})
	})

	// Validation example
	type CreateUser struct {
		Name  string `json:"name" binding:"required,min=2"`
		Email string `json:"email" binding:"required,email"`
	}
	engine.POST("/users", func(c *gin.Context) {
		body, err := validator.BindJSON[CreateUser](vi, c)
		if err != nil {
			response.HandleError(c, err)
			return
		}
		// Use utils
		slug := utils.Slugify(body.Name)
		response.Success(c, gin.H{"id": fmt.Sprintf("user:%s", slug), "email": body.Email})
	})

	// Demonstrate an application error
	engine.GET("/fail", func(c *gin.Context) {
		err := apperr.New(apperr.ErrorCodeInternal).WithMessage("demo error")
		response.HandleError(c, err)
	})

	// Time utils example
	engine.GET("/time", func(c *gin.Context) {
		now := time.Now().UTC()
		c.JSON(200, gin.H{"rfc3339": now.Format(time.RFC3339), "unix": now.Unix()})
	})

	// i18n pluralization example
	engine.GET("/i18n/items", func(c *gin.Context) {
		loc := c.GetString("i18n_locale")
		count := 3
		msg := tr.T(loc, "cart.items", map[string]any{"count": count})
		c.JSON(200, gin.H{"message": msg, "count": count})
	})

	// Secure routes: attach JWT middleware to a group
	secure := engine.Group("/secure")
	secure.Use(authmw.JWTAuth(jwtCfg))

	secure.GET("/me", authmw.RequireAnyScope("read:profile", "profile"), func(c *gin.Context) {
		claims, _ := authmw.GetClaims(c)
		response.Success(c, gin.H{"sub": claims.Subject(), "scopes": claims.Scopes()})
	})

	// AuthZ example using in-memory authorizer
	secure.GET("/docs/:id",
		authmw.RequireAuthZ(authz, func(c *gin.Context) (string, string, string, error) {
			claims, ok := authmw.GetClaims(c)
			if !ok {
				return "", "", "", fmt.Errorf("no claims")
			}
			user := claims.Subject()
			relation := "viewer"
			object := "doc:" + c.Param("id")
			return user, relation, object, nil
		}),
		func(c *gin.Context) {
			c.JSON(200, gin.H{"doc": c.Param("id"), "status": "ok"})
		},
	)

	// Start server with graceful shutdown
	if err := server.Start(engine,
		server.StartWithConfig(cfg),
		server.StartWithLogger(log),
	); err != nil {
		log.ErrorF("server error: %v", err)
		os.Exit(1)
	}
}
