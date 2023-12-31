package routes

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"
	"github.com/solomonbaez/hyacinth/api/handlers"
	"github.com/solomonbaez/hyacinth/api/models"
	"github.com/solomonbaez/hyacinth/api/workers"
)

const tokenLength = 25

func Subscribe(c *gin.Context, dh *handlers.DatabaseHandler) {
	var subscriber models.Subscriber
	var loader *handlers.Loader

	requestID := c.GetString("requestID")

	var response string
	tx, e := dh.DB.Begin(c)
	if e != nil {
		response = "Failed to begin transaction"
		handlers.HandleError(c, requestID, e, response, http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(c)

	if e = c.ShouldBindJSON(&loader); e != nil {
		response = "Could not subscribe"
		handlers.HandleError(c, requestID, e, response, http.StatusBadRequest)
		return
	}

	subscriberEmail, e := models.ParseEmail(loader.Email)
	if e != nil {
		response = "Could not subscribe"
		handlers.HandleError(c, requestID, e, response, http.StatusBadRequest)
		return
	}
	subscriberName, e := models.ParseName(loader.Name)
	if e != nil {
		response := "Could not subscribe"
		handlers.HandleError(c, requestID, e, response, http.StatusBadRequest)
		return
	}

	subscriber = models.Subscriber{
		Email:  subscriberEmail,
		Name:   subscriberName,
		Status: "pending",
	}
	if e := insertSubscriber(c, tx, &subscriber); e != nil {
		response = "Failed to insert subscriber"
		handlers.HandleError(c, requestID, e, response, http.StatusInternalServerError)
		return
	}
	if e := workers.EnqueConfirmationTasks(c, tx, subscriber.Email.String()); e != nil {
		response = "Failed to enque confirmation email"
		handlers.HandleError(c, requestID, e, response, http.StatusInternalServerError)
		return
	}

	log.Info().
		Str("requestID", requestID).
		Str("email", subscriber.Email.String()).
		Msg(fmt.Sprintf("Success, enqueued a confirmation email to %v", subscriber.Email.String()))

	tx.Commit(c)
	c.JSON(http.StatusCreated, gin.H{"requestID": requestID, "subscriber": &subscriber})
}

// TODO extract confirmation email logic as a worker TASK
func insertSubscriber(c context.Context, tx pgx.Tx, subscriber *models.Subscriber) (err error) {
	newID := uuid.NewString()

	email := subscriber.Email.String()
	name := subscriber.Name.String()
	query := "INSERT INTO subscriptions (id, email, name, status, created) VALUES ($1, $2, $3, $4, now())"
	_, e := tx.Exec(c, query, newID, email, name, "pending")
	if e != nil {
		err = fmt.Errorf("failed to insert new subscriber: %w", e)
		return
	}

	token, e := handlers.GenerateCSPRNG(tokenLength)
	if e != nil {
		err = fmt.Errorf("failed to generate subscription request token: %w", e)
		return
	}

	if e := handlers.StoreToken(c, tx, newID, token); e != nil {
		err = fmt.Errorf("failed to store subscription request token: %w", e)
		return
	}

	return
}
