package routes

import (
	"fmt"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

func Logout(c *gin.Context) {
	session := sessions.Default(c)
	user := session.Get("user")
	log.Info().
		Str("user", fmt.Sprintf("%s", user)).
		Msg("logged out")

	session.Clear()
	session.AddFlash("logged out")
	session.Save()

	c.Header("X-Redirect", "Logged out")
	c.Redirect(http.StatusSeeOther, "../login")
}
