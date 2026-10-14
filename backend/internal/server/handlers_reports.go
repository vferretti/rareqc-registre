package server

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"registre-admin/internal/repository"
	_ "registre-admin/internal/types" // for swagger annotations
)

// ReportsSummaryHandler returns aggregated summary statistics.
//
// @Summary     Get report summary
// @Description Returns summary statistics as of a given date (defaults to today)
// @Tags        reports
// @Produce     json
// @Param       report_date query string false "Report date (YYYY-MM-DD), defaults to today"
// @Success     200 {object} types.ReportsSummary
// @Security BearerAuth
// @Router      /reports/summary [get]
func ReportsSummaryHandler(repo repository.ReportsDAO) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Default to start of tomorrow (includes all of today)
		reportDate := time.Now().AddDate(0, 0, 1).Truncate(24 * time.Hour)

		if rd := c.Query("report_date"); rd != "" {
			parsed, err := time.Parse("2006-01-02", rd)
			if err != nil {
				handleBadRequest(c, "Invalid report_date format (expected YYYY-MM-DD)")
				return
			}
			// Start of next day so queries use created_at < reportDate
			reportDate = parsed.AddDate(0, 0, 1)
		}

		summary, err := repo.GetSummary(reportDate)
		if err != nil {
			handleInternalError(c, err, "Failed to compute summary")
			return
		}
		c.JSON(http.StatusOK, summary)
	}
}
