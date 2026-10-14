package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"registre-admin/internal/repository"
)

// SearchHandler returns suggestions matching a query across participants and contacts.
//
// @Summary     Global search
// @Description Returns up to 10 search suggestions matching a query across participants and contacts. Requires at least 2 characters.
// @Tags        search
// @Produce     json
// @Param       q query string true "Search query (min 2 characters)"
// @Success     200 {array}  repository.SearchSuggestion
// @Security BearerAuth
// @Router      /search [get]
func SearchHandler(repo repository.SearchDAO) gin.HandlerFunc {
	return func(c *gin.Context) {
		q := strings.TrimSpace(c.Query("q"))
		if len(q) < 2 {
			c.JSON(http.StatusOK, []repository.SearchSuggestion{})
			return
		}
		results, err := repo.Search(q)
		if err != nil {
			handleInternalError(c, err, "Failed to search")
			return
		}
		c.JSON(http.StatusOK, results)
	}
}
