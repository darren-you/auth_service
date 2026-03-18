package main

import (
	"context"
	stdErrors "errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/darren-you/auth_service/session"
	"github.com/darren-you/auth_service/template_server/internal/api/middleware"
	"github.com/darren-you/auth_service/template_server/internal/api/router"
	"github.com/darren-you/auth_service/template_server/internal/config"
	"github.com/darren-you/auth_service/template_server/internal/constant"
	appErrors "github.com/darren-you/auth_service/template_server/internal/errors"
	"github.com/darren-you/auth_service/template_server/internal/model"
	mysqlRepo "github.com/darren-you/auth_service/template_server/internal/repository/mysql"
	redisRepo "github.com/darren-you/auth_service/template_server/internal/repository/redis"
	"github.com/darren-you/auth_service/template_server/internal/service"
	"github.com/darren-you/auth_service/template_server/pkg/cache"
	"github.com/darren-you/auth_service/template_server/pkg/database"
	"github.com/darren-you/auth_service/template_server/pkg/logger"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		panic(err)
	}

	if err := logger.Init(&cfg.Log); err != nil {
		panic(err)
	}
	defer logger.Close()

	db, err := database.InitMySQL(&cfg.MySQL)
	if err != nil {
		logger.Fatalf("Failed to initialize MySQL: %v", err)
	}

	if err := db.AutoMigrate(
		&model.AuthTenant{},
		&model.AuthProviderConfig{},
		&model.AuthUser{},
		&model.AuthIdentity{},
		&model.AuthSession{},
	); err != nil {
		logger.Fatalf("Failed to migrate database schema: %v", err)
	}

	redisClient, err := cache.InitRedis(&cfg.Redis)
	if err != nil {
		logger.Fatalf("Failed to initialize Redis: %v", err)
	}

	app := fiber.New(fiber.Config{
		ErrorHandler: customErrorHandler,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
	})

	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: strings.Join(cfg.Server.AllowOrigins, ","),
		AllowMethods: strings.Join(cfg.Server.AllowMethods, ","),
		AllowHeaders: strings.Join(cfg.Server.AllowHeaders, ","),
	}))

	traceMiddleware := middleware.NewTraceMiddleware()
	loggerMiddleware := middleware.NewLoggerMiddleware()
	app.Use(traceMiddleware.Trace)
	app.Use(loggerMiddleware.Logger)

	sessionConfig := session.Config{
		SecretKey:     cfg.JWT.Secret,
		Issuer:        cfg.JWT.Issuer,
		AccessExpiry:  time.Duration(cfg.JWT.ExpiresIn) * time.Second,
		RefreshExpiry: time.Duration(cfg.JWT.RefreshExpiresIn) * time.Second,
	}

	mysqlRepository := mysqlRepo.NewAuthRepository(db)
	redisRepository := redisRepo.NewKVRepository(redisClient)
	authService := service.NewAuthService(mysqlRepository, redisRepository, cfg.Auth, sessionConfig)
	if err := authService.SyncCatalog(context.Background()); err != nil {
		logger.Fatalf("Failed to sync tenant/provider configs: %v", err)
	}

	router.SetupRoutes(app, authService)

	go func() {
		addr := fmt.Sprintf(":%d", cfg.Server.Port)
		if err := app.Listen(addr); err != nil && !stdErrors.Is(err, http.ErrServerClosed) {
			logger.Fatalf("Failed to start server: %v", err)
		}
	}()

	logger.Infof("Auth service started on port %d", cfg.Server.Port)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		logger.Errorf("Failed to shutdown app: %v", err)
	}

	sqlDB, err := db.DB()
	if err == nil {
		_ = sqlDB.Close()
	}
	_ = redisClient.Close()
}

func customErrorHandler(c *fiber.Ctx, err error) error {
	traceID, _ := c.Locals(constant.TraceIDKey).(string)
	logger.ErrorfWithTraceID(traceID, "Error handling request: %v", err)

	if ok, customErr := appErrors.IsCustomError(err); ok {
		return c.Status(customErr.HTTPStatus).JSON(fiber.Map{
			"code":      customErr.Code,
			"timestamp": time.Now().UnixMilli(),
			"msg":       customErr.Message,
		})
	}

	var fiberErr *fiber.Error
	if stdErrors.As(err, &fiberErr) {
		return c.Status(fiberErr.Code).JSON(fiber.Map{
			"code":      fiberErr.Code,
			"timestamp": time.Now().UnixMilli(),
			"msg":       fiberErr.Message,
		})
	}

	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
		"code":      fiber.StatusInternalServerError,
		"timestamp": time.Now().UnixMilli(),
		"msg":       "internal server error",
	})
}
