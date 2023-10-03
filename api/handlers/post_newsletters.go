package handlers

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"github.com/solomonbaez/SB-Go-Newsletter-API/api/clients"
	"github.com/solomonbaez/SB-Go-Newsletter-API/api/models"
)

type Credentials struct {
	username string
	password string
}

func (rh *RouteHandler) PostNewsletter(c *gin.Context, client *clients.SMTPClient) {
	var newsletter models.Newsletter
	var body models.Body
	var response string
	var e error

	requestID := c.GetString("requestID")
	userCredentials, e := BasicAuth(c)
	if e != nil {
		response = "Unauthorized user"
		HandleError(c, requestID, e, response, http.StatusBadRequest)
		return
	}

	_, e = rh.ValidateCredentials(c, userCredentials)
	if e != nil {
		response := "Failed to validate credentials"
		HandleError(c, requestID, e, response, http.StatusBadRequest)
	}

	if e = c.ShouldBindJSON(&body); e != nil {
		response = "Could not send newsletter"
		HandleError(c, requestID, e, response, http.StatusInternalServerError)
		return
	}
	newsletter.Content = &body

	subscribers := rh.GetConfirmedSubscribers(c)
	for _, s := range subscribers {
		// re-parse email to ensure data integrity
		newsletter.Recipient, e = models.ParseEmail(s.Email.String())
		if e != nil {
			response = fmt.Sprintf("Invalid email: %v", s.Email.String())
			HandleError(c, requestID, e, response, http.StatusConflict)
			continue
		}
		if e = client.SendEmail(c, &newsletter); e != nil {
			response = "Could not send newsletter"
			HandleError(c, requestID, e, response, http.StatusInternalServerError)
			continue
		}
	}

	c.JSON(http.StatusOK, gin.H{"requestID": requestID, "message": "Emails successfully delivered"})
}

func (rh *RouteHandler) ValidateCredentials(c *gin.Context, credentials *Credentials) (*string, error) {
	var id string

	requestID := c.GetString("requestID")

	query := "SELECT id FROM users WHERE username=$1 AND password=$2"
	e := rh.DB.QueryRow(c, query, credentials.username, credentials.password).Scan(&id)
	if e != nil {
		return nil, e
	}

	log.Info().
		Str("requestID", requestID).
		Str("userID", id).
		Msg("Successfully validated user credentials")

	return &id, nil
}

func BasicAuth(c *gin.Context) (*Credentials, error) {
	var e error

	h := c.GetHeader("Authorization")

	encodedSegment, valid := strings.CutPrefix(h, "Basic ")
	if !valid {
		e := errors.New("authorization method is not Basic")
		return nil, e
	}

	decodedSegment, e := base64.RawStdEncoding.DecodeString(encodedSegment)
	if e != nil {
		return nil, e
	}

	valid = utf8.Valid(decodedSegment)
	if !valid {
		e = errors.New("invalid header encoding")
		return nil, e
	}

	// valid header should only contain two segments
	utf8Segment := string(decodedSegment)
	s := strings.Split(utf8Segment, ":")
	username := s[0]
	password := s[1]

	credentials := &Credentials{
		username,
		password,
	}

	return credentials, nil
}