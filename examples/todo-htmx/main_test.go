package main

import (
	"net/http"
	"os"
	"testing"

	"github.com/calvinmclean/babyapi"
	babytest "github.com/calvinmclean/babyapi/test"
)

func TestAPI(t *testing.T) {
	defer os.RemoveAll("storage.json")

	os.Setenv("STORAGE_FILE", "storage.json")
	api := createAPI()

	babytest.RunTableTest(t, api, []babytest.TestCase[*babyapi.AnyResource]{
		{
			Name: "CreateTODO",
			Test: babytest.RequestTest[*babyapi.AnyResource]{
				Method: http.MethodPost,
				Body:   `{"Title": "New TODO"}`,
			},
			ExpectedResponse: babytest.ExpectedResponse{
				Status:     http.StatusCreated,
				BodyRegexp: `{"id":"[0-9a-v]{20}","Title":"New TODO","Description":"","Completed":null}`,
			},
		},
		{
			Name: "GetTODO",
			Test: babytest.RequestTest[*babyapi.AnyResource]{
				Method: http.MethodGet,
				IDFunc: func(getResponse babytest.PreviousResponseGetter) string {
					return getResponse("CreateTODO").Data.GetID()
				},
			},
			ExpectedResponse: babytest.ExpectedResponse{
				Status:     http.StatusOK,
				BodyRegexp: `{"id":"[0-9a-v]{20}","Title":"New TODO","Description":"","Completed":null}`,
			},
		},
		{
			Name: "DeleteTODO",
			Test: babytest.RequestTest[*babyapi.AnyResource]{
				Method: http.MethodDelete,
				IDFunc: func(getResponse babytest.PreviousResponseGetter) string {
					return getResponse("CreateTODO").Data.GetID()
				},
			},
			ExpectedResponse: babytest.ExpectedResponse{
				Status: http.StatusOK,
				NoBody: true,
			},
		},
	})
}
