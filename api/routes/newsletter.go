package routes

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/solomonbaez/SB-Go-Newsletter-API/api/idempotency"
)

func GetNewsletter(c *gin.Context) {
	session := sessions.Default(c)
	flashes := session.Flashes()
	key, _ := idempotency.GenerateIdempotencyKey()

	c.HTML(http.StatusOK, "newsletter", gin.H{"flashes": flashes, "idempotency_key": key})
}