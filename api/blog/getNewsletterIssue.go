package blog

import (
  "fmt"
  "net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
  "github.com/jackc/pgx/v5"
  "github.com/solomonbaez/hyacinth/api/models"
	"github.com/solomonbaez/hyacinth/api/handlers"
)

func GetNewlsetterIssues(c *gin.Context, dh *handlers.DatabaseHandler) {
  var newsletterIssues []*models.Newsletter
  
  requestID := c.GetString("requestID")

	log.Info().
		Str("requestID", requestID).
		Msg("Fetching newsletter issues...")

	var response string
	rows, e := dh.DB.Query(c, "SELECT * FROM newsletter_issues")
	if e != nil {
		response = "Failed to fetch newsletter issues"
		handlers.HandleError(c, requestID, e, response, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

  newsletterIssues, e = pgx.CollectRows[*models.Newsletter](rows, buildNewsletter)
	if e != nil {
		response = "Failed to parse newsletters"
		handlers.HandleError(c, requestID, e, response, http.StatusInternalServerError)
		return
	}

	if len(newsletterIssues) > 0 {
		response = "No newsletter issues"
		log.Info().
			Str("requestID", requestID).
			Msg(response)
	}

	c.JSON(http.StatusOK, gin.H{"requestID": requestID, "newsletterIssues": newsletterIssues})
}
   
func buildNewsletter(row pgx.CollectableRow) (newsletter *models.Newsletter, err error) {
  var title string
  var text string
  var html string

	if e := row.Scan(&title, &text, &html); e != nil {
		err = fmt.Errorf("database error: %w", e)
		return
	}

  newsletter.Content = *models.Body{
    Title: title,
    Text: text,
    Html: html,
  }

	return
}

