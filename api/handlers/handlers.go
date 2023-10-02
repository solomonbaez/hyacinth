package handlers

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rs/zerolog/log"
	"github.com/solomonbaez/SB-Go-Newsletter-API/api/clients"
	"github.com/solomonbaez/SB-Go-Newsletter-API/api/models"
)

// TODO switch to cfg baseURL
const baseURL = "http://localhost:8000"
const tokenLength = 25
const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

var confirmationLink string
var confirmation = &clients.Message{
	Subject: "Confirm Your Subscription!",
}

type Database interface {
	Exec(c context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
	Query(c context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	QueryRow(c context.Context, sql string, args ...interface{}) pgx.Row
	Begin(c context.Context) (pgx.Tx, error)
}

type RouteHandler struct {
	DB Database
}

func NewRouteHandler(db Database) *RouteHandler {
	return &RouteHandler{
		DB: db,
	}
}

type Loader struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

var loader *Loader

func (rh *RouteHandler) PostNewsletter(c *gin.Context, client *clients.SMTPClient) {
	// var body models.NewsletterBody

	subscribers := rh.GetConfirmedSubscribers(c)
	for _, s := range subscribers {
		fmt.Printf(s.Email.String())
	}
}

func (rh *RouteHandler) Subscribe(c *gin.Context, client *clients.SMTPClient) {
	var subscriber *models.Subscriber

	requestID := c.GetString("requestID")

	newID := uuid.NewString()
	created := time.Now()
	status := "pending"

	var response string
	var e error

	tx, e := rh.DB.Begin(c)
	if e != nil {
		response = "Failed to begin transaction"
		HandleError(c, requestID, e, response, http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(c)

	if e = c.ShouldBindJSON(&loader); e != nil {
		response = "Could not subscribe"
		HandleError(c, requestID, e, response, http.StatusBadRequest)
		return
	}

	log.Info().
		Str("requestID", requestID).
		Msg("Validating inputs...")

	subscriberEmail, e := models.ParseEmail(loader.Email)
	if e != nil {
		response = "Could not subscribe"
		HandleError(c, requestID, e, response, http.StatusBadRequest)
		return
	}
	subscriberName, e := models.ParseName(loader.Name)
	if e != nil {
		response := "Could not subscribe"
		HandleError(c, requestID, e, response, http.StatusBadRequest)
		return
	}

	subscriber = &models.Subscriber{
		Email:  subscriberEmail,
		Name:   subscriberName,
		Status: status,
	}

	// correlate request with inputs
	log.Info().
		Str("requestID", requestID).
		Str("email", subscriber.Email.String()).
		Str("name", subscriber.Name.String()).
		Msg("")

	log.Info().
		Str("requestID", requestID).
		Msg("Subscribing...")

	query := "INSERT INTO subscriptions (id, email, name, created, status) VALUES ($1, $2, $3, $4, $5)"
	_, e = tx.Exec(c, query, newID, subscriber.Email.String(), subscriber.Name.String(), created, status)
	if e != nil {
		response = "Failed to subscribe"
		HandleError(c, requestID, e, response, http.StatusInternalServerError)
		return
	}

	token, e := generateCSPRNG()
	if e != nil {
		response = "Failed to generate token"
		HandleError(c, requestID, e, response, http.StatusInternalServerError)
		return
	}

	if client.SmtpServer != "test" {
		confirmationLink = fmt.Sprintf("%v/%v", baseURL, token)
		confirmation.Text = fmt.Sprintf("Welcome to our newsletter! Please confirm your subscription at: %v", confirmationLink)
		confirmation.Html = fmt.Sprintf("<p>Welcome to our newsletter! Please confirm your subscription at: <a>%v</a></p>", confirmationLink)

		confirmation.Recipient = subscriber.Email
		if e := client.SendEmail(c, confirmation, token); e != nil {
			response = "Failed to send confirmation email"
			HandleError(c, requestID, e, response, http.StatusInternalServerError)
			return
		}
	}

	if e := rh.storeToken(c, tx, newID, token); e != nil {
		response = "Failed to store user token"
		HandleError(c, requestID, e, response, http.StatusInternalServerError)
		return
	}

	log.Info().
		Str("requestID", requestID).
		Str("email", subscriber.Email.String()).
		Msg(fmt.Sprintf("Success, sent a confirmation email to %v", subscriber.Email.String()))

	c.JSON(http.StatusCreated, gin.H{"requestID": requestID, "subscriber": subscriber})
}

func (rh *RouteHandler) ConfirmSubscriber(c *gin.Context) {
	var id string
	var query string
	var response string
	var e error

	requestID := c.GetString("requestID")
	token := c.Param("token")

	query = "SELECT (subscriber_id) FROM subscription_tokens WHERE subscription_token = $1"
	e = rh.DB.QueryRow(c, query, token).Scan(&id)
	if e != nil {
		response = "Failed to fetch subscriber ID"
		HandleError(c, requestID, e, response, http.StatusInternalServerError)
		return
	}

	query = "UPDATE subscriptions SET status = 'confirmed' WHERE id = $1"
	_, e = rh.DB.Exec(c, query, id)
	if e != nil {
		response = "Failed to confirm subscription"
		HandleError(c, requestID, e, response, http.StatusInternalServerError)
		return
	}

	log.Info().
		Msg("Subscription confirmed")

	c.JSON(http.StatusAccepted, gin.H{"requestID": requestID, "subscriber": "Subscription confirmed"})
}

func (rh *RouteHandler) GetSubscribers(c *gin.Context) {
	var subscribers []*models.Subscriber
	requestID := c.GetString("requestID")

	var response string
	var e error

	log.Info().
		Str("requestID", requestID).
		Msg("Fetching subscribers...")

	rows, e := rh.DB.Query(c, "SELECT * FROM subscriptions")
	if e != nil {
		response = "Failed to fetch subscribers"
		HandleError(c, requestID, e, response, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	subscribers, e = pgx.CollectRows[*models.Subscriber](rows, BuildSubscriber)
	if e != nil {
		response = "Failed to parse subscribers"
		HandleError(c, requestID, e, response, http.StatusInternalServerError)
		return
	}

	if len(subscribers) > 0 {
		c.JSON(http.StatusOK, gin.H{"requestID": requestID, "subscribers": subscribers})
	} else {
		response = "No subscribers"
		log.Info().
			Str("requestID", requestID).
			Msg(response)

		c.JSON(http.StatusOK, gin.H{"requestID": requestID, "subscribers": response})
	}
}

func (rh *RouteHandler) GetSubscriberByID(c *gin.Context) {
	requestID := c.GetString("requestID")

	var response string
	var e error

	log.Info().
		Str("requestID", requestID).
		Msg("Validating ID...")

	// Validate UUID
	u := c.Param("id")
	id, e := uuid.Parse(u)
	if e != nil {
		response = "Invalid ID format"
		HandleError(c, requestID, e, response, http.StatusBadRequest)
		return
	}

	log.Info().
		Str("requestID", requestID).
		Msg("Fetching subscriber...")

	var subscriber models.Subscriber
	e = rh.DB.QueryRow(c, "SELECT id, email, name, status FROM subscriptions WHERE id=$1", id).Scan(&subscriber.ID, &subscriber.Email, &subscriber.Name, &subscriber.Status)
	if e != nil {
		if e == pgx.ErrNoRows {
			response = "Subscriber not found"
		} else {
			response = "Database query error"
		}
		HandleError(c, requestID, e, response, http.StatusNotFound)
		return
	}

	c.JSON(http.StatusFound, gin.H{"requestID": requestID, "subscriber": subscriber})
}

func (rh RouteHandler) GetConfirmedSubscribers(c *gin.Context) []*models.Subscriber {
	var subscribers []*models.Subscriber
	requestID := c.GetString("requestID")

	var response string
	var e error

	log.Info().
		Str("requestID", requestID).
		Msg("Fetching confirmed subscribers...")

	rows, e := rh.DB.Query(c, "SELECT * FROM subscriptions WHERE status=$1", "confirmed")
	if e != nil {
		response = "Failed to fetch confirmed subscribers"
		HandleError(c, requestID, e, response, http.StatusInternalServerError)
		return nil
	}
	defer rows.Close()

	subscribers, e = pgx.CollectRows[*models.Subscriber](rows, BuildSubscriber)
	if e != nil {
		response = "Failed to parse confirmed subscribers"
		HandleError(c, requestID, e, response, http.StatusInternalServerError)
		return nil
	}

	if len(subscribers) > 0 {
		c.JSON(http.StatusOK, gin.H{"requestID": requestID, "subscribers": subscribers})
		return subscribers
	} else {
		response = "No confirmed subscribers"
		log.Info().
			Str("requestID", requestID).
			Msg(response)

		c.JSON(http.StatusOK, gin.H{"requestID": requestID, "subscribers": response})
		return nil
	}
}

func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, "OK")
}

func (rh *RouteHandler) storeToken(c *gin.Context, tx pgx.Tx, id string, token string) error {
	query := "INSERT INTO subscription_tokens (subscription_token, subscriber_id) VALUES ($1, $2)"
	_, e := tx.Exec(c, query, token, id)
	if e != nil {
		return e
	}

	// commit changes
	tx.Commit(c)
	return nil
}

func generateCSPRNG() (string, error) {
	b := make([]byte, tokenLength)

	maxIndex := big.NewInt(int64(len(charset)))

	for i := range b {
		r, e := rand.Int(rand.Reader, maxIndex)
		if e != nil {
			return "", e
		}

		b[i] = charset[r.Int64()]
	}

	return string(b), nil
}

func BuildSubscriber(row pgx.CollectableRow) (*models.Subscriber, error) {
	var id string
	var email models.SubscriberEmail
	var name models.SubscriberName
	var created time.Time
	var status string

	e := row.Scan(&id, &email, &name, &created, &status)
	s := &models.Subscriber{
		ID:     id,
		Email:  email,
		Name:   name,
		Status: status,
	}

	return s, e
}

func HandleError(c *gin.Context, id string, e error, response string, status int) {
	log.Error().
		Str("requestID", id).
		Err(e).
		Msg(response)

	var message strings.Builder
	message.WriteString(response)
	message.WriteString(": ")
	message.WriteString(e.Error())

	c.JSON(status, gin.H{"requestID": id, "error": message.String()})
}
