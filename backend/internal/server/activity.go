package server

import (
	"fmt"
	"maps"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"registre-admin/internal/auth"
)

// maxAuthorLength caps the author name stored in activity logs.
const maxAuthorLength = 200

// getAuthor returns the authenticated user's display name (set by the auth
// middleware) for activity-log authorship.
func getAuthor(c *gin.Context) string {
	author := auth.UserName(c)
	if len(author) > maxAuthorLength {
		author = author[:maxAuthorLength]
	}
	return author
}

// exportDetails describes an audited export: the row count and the query
// that selected the rows, without pagination noise.
func exportDetails(what string, rows int, query url.Values) string {
	filters := maps.Clone(query)
	for _, key := range []string{"export", "page_index", "page_size"} {
		delete(filters, key)
	}
	if len(filters) == 0 {
		return fmt.Sprintf("%s export: %d rows", what, rows)
	}
	return fmt.Sprintf("%s export: %d rows (%s)", what, rows, filters.Encode())
}

// joinInts renders ids as a compact comma-separated list.
func joinInts(ids []int) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.Itoa(id)
	}
	return strings.Join(parts, ",")
}
