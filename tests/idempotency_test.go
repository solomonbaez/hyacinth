package api_test

import (
	"net/http"

	"testing"

	"github.com/pashagolub/pgxmock/v3"
)

func Test_GetSavedResponse_Passes(t *testing.T) {
	app := new_mock_app()
	defer app.database.Close(app.context)

	request, e := http.NewRequest("GET", "/responses", nil)
	if e != nil {
		t.Fatal(e)
	}

	code := http.StatusSeeOther
	body := []byte("httpbody")

	query := "SELECT response_status_code, response_body FROM idempotency WHERE user_id = $1 AND idempotency_key = $2"
	app.database.ExpectQuery(query).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(
			pgxmock.NewRows([]string{"respons_status_code", "response_body"}).
				AddRow(code, body),
		)

	name := "location"
	value := []byte("/admin/dashboard")
	query = "SELECT header_name, header_value FROM idempotency_headers WHERE idempotency_key = $1"
	app.database.ExpectQuery(query).
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(
			pgxmock.NewRows([]string{"header_name", "header_value"}).
				AddRow(name, value),
		)

	app.new_mock_request(request)
	app.database.ExpectationsWereMet()
}

func Test_PostSaveResponse_Passes(t *testing.T) {
	app := new_mock_app()
	defer app.database.Close(app.context)

	request, e := http.NewRequest("POST", "/responses", nil)
	if e != nil {
		t.Fatal(e)
	}

	query := "UPDATE idempotency SET response_status_code = $3, response_body = $4 WHERE user_id = $1 AND idempotency_key = $2"
	app.database.ExpectExec(query).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	query = "UPDATE idempotency_headers SET header_name = $2, header_value = $3 WHERE idempotency_key = $1"
	app.database.ExpectExec(query).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	app.new_mock_request(request)
	app.database.ExpectationsWereMet()
}