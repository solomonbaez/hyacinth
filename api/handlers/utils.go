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
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"
	"github.com/solomonbaez/SB-Go-Newsletter-API/api/models"
)

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, "OK")
}

func StoreToken(c context.Context, tx pgx.Tx, id string, token string) (err error) {
	query := "INSERT INTO subscription_tokens (subscription_token, subscriber_id) VALUES ($1, $2)"
	_, e := tx.Exec(c, query, token, id)
	if e != nil {
		err = fmt.Errorf("database error: %w", e)
		return
	}

	tx.Commit(c)
	return
}

func GenerateCSPRNG(tokenLen int) (csprng string, err error) {
	b := make([]byte, tokenLen)
	maxIndex := big.NewInt(int64(len(charset)))

	var r *big.Int
	var e error
	for i := range b {
		r, e = rand.Int(rand.Reader, maxIndex)
		if e != nil {
			err = fmt.Errorf("failed to generate csprng: %w", e)
			return
		}

		b[i] = charset[r.Int64()]
	}

	csprng = string(b)
	return
}

func BuildSubscriber(row pgx.CollectableRow) (subscriber *models.Subscriber, err error) {
	var id string
	var email models.SubscriberEmail
	var name models.SubscriberName
	var created time.Time
	var status string

	if e := row.Scan(&id, &email, &name, &created, &status); e != nil {
		err = fmt.Errorf("database error: %w", e)
		return
	}

	subscriber = &models.Subscriber{
		ID:     id,
		Email:  email,
		Name:   name,
		Status: status,
	}

	return
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
