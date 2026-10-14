package server

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/gorm"
	"registre-admin/internal/auth"
	"registre-admin/internal/config"
	"registre-admin/internal/repository"
	"registre-admin/internal/types"
)

// SetupRouter configures the Gin engine with the hardening middlewares,
// CORS, Swagger (dev only) and all API routes.
func SetupRouter(db *gorm.DB, cfg config.Config, authCfg auth.Config) *gin.Engine {
	r := gin.New()
	// Only proxies listed in TRUSTED_PROXIES may set X-Forwarded-For /
	// X-Forwarded-Proto; with none, c.ClientIP() is the TCP peer.
	if err := r.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		panic(fmt.Sprintf("invalid TRUSTED_PROXIES: %v", err))
	}
	r.MaxMultipartMemory = maxUploadBytes
	r.Use(requestID(), requestLogger(), recovery(), securityHeaders(cfg.Production))

	r.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.CORSAllowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Authorization"},
		AllowCredentials: false,
	}))

	// Swagger UI documents the whole attack surface: dev only.
	if !cfg.Production {
		r.GET("/swagger/*any", swaggerCSP(), ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	// BFF authentication (Keycloak) — see internal/auth.
	authService := auth.NewService(authCfg)

	// Repositories
	participantRepo := repository.NewParticipantRepository(db)
	contactRepo := repository.NewContactRepository(db)
	activityRepo := repository.NewActivityRepository(db)
	searchRepo := repository.NewSearchRepository(db)
	consentRepo := repository.NewConsentRepository(db)
	docRepo := repository.NewDocumentRepository(db)
	extIDRepo := repository.NewExternalIDRepository(db)
	guidRepo := repository.NewGuidRepository(db)
	codeTableRepo := repository.NewCodeTableRepository(db)
	extSysRepo := repository.NewExternalSystemRepository(db)
	cartRepo := repository.NewCartRepository(db)
	reportsRepo := repository.NewReportsRepository(db)
	commRepo := repository.NewCommunicationRepository(db)

	api := r.Group("/api")
	{
		// Body caps and the Origin check apply to every /api route,
		// including the cookie-authenticated POST /auth/logout.
		api.Use(bodyLimit(), csrfGuard(authCfg.BrowserOrigins()))

		// Public routes: health check and the BFF auth endpoints.
		api.GET("/health", HealthHandler(db))
		authService.RegisterRoutes(api)

		// Everything registered below requires an authenticated identity
		// (session cookie or Bearer token) with the registre_admin role.
		// Participant routes (/me/...) will form their own group with
		// auth.RoleParticipant (A5).
		api.Use(authService.Middleware(auth.RoleAdmin))

		api.GET("/enums", EnumsHandler(codeTableRepo))
		api.GET("/reports/summary", ReportsSummaryHandler(reportsRepo))

		api.GET("/participants", ListParticipantsHandler(participantRepo, activityRepo))
		api.GET("/participants/:id", GetParticipantHandler(participantRepo))
		api.POST("/participants/resolve-ids", ResolveIDsHandler(participantRepo, extIDRepo, guidRepo))
		api.POST("/participants", CreateParticipantHandler(participantRepo, contactRepo, activityRepo))
		api.PUT("/participants/:id", UpdateParticipantHandler(participantRepo, contactRepo, activityRepo))
		api.DELETE("/participants/:id", DeleteParticipantHandler(participantRepo, activityRepo))

		api.POST("/participants/:id/contacts", AddContactHandler(participantRepo, contactRepo, activityRepo))
		api.PUT("/contacts/:contactId", UpdateContactHandler(contactRepo, activityRepo))
		api.DELETE("/contacts/:contactId", DeleteContactHandler(contactRepo, activityRepo))

		api.GET("/participants/:id/consents", ListParticipantConsentsHandler(consentRepo))
		api.POST("/participants/:id/consents", CreateParticipantConsentHandler(consentRepo, contactRepo, activityRepo))
		api.PUT("/consents/:consentId", UpdateConsentHandler(consentRepo, contactRepo, activityRepo))
		api.GET("/consent-clauses", ListConsentClausesHandler(consentRepo))
		api.GET("/consent-templates", ListConsentTemplatesHandler(consentRepo))
		api.POST("/consent-templates", CreateConsentTemplateHandler(consentRepo))
		api.PUT("/consent-templates/:id", UpdateConsentTemplateHandler(consentRepo))
		api.DELETE("/consent-templates/:id", DeleteConsentTemplateHandler(consentRepo))

		api.GET("/participants/:id/communications", ListParticipantCommunicationsHandler(commRepo))
		api.POST("/participants/:id/communications", CreateParticipantCommunicationHandler(commRepo))
		api.PUT("/communications/:communicationId", UpdateCommunicationHandler(commRepo))
		api.DELETE("/communications/:communicationId", DeleteCommunicationHandler(commRepo))

		api.POST("/documents", UploadDocumentHandler(docRepo))
		api.GET("/documents/:id/file", DownloadDocumentHandler(docRepo))

		api.GET("/participants/:id/external-ids", ListParticipantExternalIDsHandler(extIDRepo))

		api.GET("/search", SearchHandler(searchRepo))

		api.GET("/activity-logs", ListActivityLogsHandler(activityRepo))
		api.GET("/participants/:id/activity-logs", ListParticipantActivityLogsHandler(participantRepo, activityRepo))

		api.GET("/code-tables", ListCodeTablesHandler(codeTableRepo))
		api.POST("/code-tables/:table/entries", CreateCodeEntryHandler(codeTableRepo))
		api.PUT("/code-tables/:table/entries/:code", UpdateCodeEntryHandler(codeTableRepo))
		api.DELETE("/code-tables/:table/entries/:code", DeleteCodeEntryHandler(codeTableRepo))

		api.GET("/admin-users", ListAdminUsersHandler(authService))

		api.GET("/external-systems", ListExternalSystemsHandler(extSysRepo))
		api.POST("/external-systems", CreateExternalSystemHandler(extSysRepo))
		api.PUT("/external-systems/:id", UpdateExternalSystemHandler(extSysRepo))
		api.DELETE("/external-systems/:id", DeleteExternalSystemHandler(extSysRepo))

		cart := api.Group("/cart")
		{
			cart.GET("/items", ListCartItemsHandler(cartRepo))
			cart.GET("/count", CartCountHandler(cartRepo))
			cart.POST("/items", AddCartItemsHandler(cartRepo))
			cart.DELETE("/items", RemoveCartItemsHandler(cartRepo))
			cart.DELETE("", ClearCartHandler(cartRepo))
			cart.POST("/export-data", CartExportDataHandler(cartRepo, participantRepo, consentRepo, extIDRepo, activityRepo))
			cart.POST("/communications", CreateCartCommunicationsHandler(cartRepo, commRepo))
		}
	}

	return r
}

// parsePaginationParams extracts and validates pagination, sorting, and search parameters from the query string.
func parsePaginationParams(c *gin.Context, defaultSortField string) types.PaginationParams {
	pageIndex, _ := strconv.Atoi(c.DefaultQuery("page_index", "0"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "25"))
	sortField := c.DefaultQuery("sort_field", defaultSortField)
	sortOrder := c.DefaultQuery("sort_order", "asc")
	search := c.Query("search")
	parseCSV := func(key string) []string {
		raw := c.Query(key)
		if raw == "" {
			return nil
		}
		return strings.Split(raw, ",")
	}
	parseIntCSV := func(key string) []int {
		raw, exists := c.GetQuery(key)
		if !exists {
			return nil
		}
		if raw == "" {
			// Param present but empty → match nothing
			return []int{-1}
		}
		parts := strings.Split(raw, ",")
		ids := []int{}
		for _, p := range parts {
			if id, err := strconv.Atoi(strings.TrimSpace(p)); err == nil {
				ids = append(ids, id)
			}
		}
		if len(ids) == 0 {
			return []int{-1}
		}
		return ids
	}

	if pageSize < 1 {
		pageSize = 25
	}
	if pageSize > 200 {
		pageSize = 200
	}
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "asc"
	}

	return types.PaginationParams{
		PageIndex:              pageIndex,
		PageSize:               pageSize,
		SortField:              sortField,
		SortOrder:              sortOrder,
		Search:                 search,
		ConsentRegistry:        parseCSV("consent_registry"),
		ConsentRecontact:       parseCSV("consent_recontact"),
		ConsentExternalLinkage: parseCSV("consent_external_linkage"),
		ExternalSystems:        parseCSV("external_system"),
		ParticipantIDs:         parseIntCSV("participant_ids"),
	}
}
