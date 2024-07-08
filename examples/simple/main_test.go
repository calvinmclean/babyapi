package main

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/calvinmclean/babyapi"
	babytest "github.com/calvinmclean/babyapi/test"
)

func TestAPI(t *testing.T) {
	api := babyapi.NewAPI(
		"TODOs", "/todos",
		func() *TODO { return &TODO{} },
	)

	babytest.RunTableTest(t, api, []babytest.TestCase[*babyapi.AnyResource]{
		{
			Name: "Create",
			Test: babytest.RequestTest[*babyapi.AnyResource]{
				Method: http.MethodPost,
				Body:   `{"Title": "New TODO", "Description": "This is the first TODO item"}`,
			},
			ExpectedResponse: babytest.ExpectedResponse{
				Status:     http.StatusCreated,
				BodyRegexp: `{"id":"[0-9a-v]{20}","Title":"New TODO","Description":"This is the first TODO item","Completed":false}`,
			},
		},
		{
			Name: "Get",
			Test: babytest.RequestTest[*babyapi.AnyResource]{
				Method: http.MethodGet,
				IDFunc: func(getResponse babytest.PreviousResponseGetter) string {
					return getResponse("Create").Data.GetID()
				},
			},
			ExpectedResponse: babytest.ExpectedResponse{
				Status:     http.StatusOK,
				BodyRegexp: `{"id":"[0-9a-v]{20}","Title":"New TODO","Description":"This is the first TODO item","Completed":false}`,
			},
		},
		{
			Name: "List",
			Test: babytest.RequestTest[*babyapi.AnyResource]{
				Method: babyapi.MethodGetAll,
			},
			ExpectedResponse: babytest.ExpectedResponse{
				Status:     http.StatusOK,
				BodyRegexp: `{"items":\[{"id":"[0-9a-v]{20}","Title":"New TODO","Description":"This is the first TODO item","Completed":false}\]}`,
			},
		},
		{
			Name: "UpdateWithPut",
			Test: babytest.RequestTest[*babyapi.AnyResource]{
				Method: http.MethodPut,
				IDFunc: func(getResponse babytest.PreviousResponseGetter) string {
					return getResponse("Create").Data.GetID()
				},
				BodyFunc: func(getResponse babytest.PreviousResponseGetter) string {
					id := getResponse("Create").Data.GetID()
					return fmt.Sprintf(`{"id":"%s","Title":"New Title","Description":"New Description"}`, id)
				},
			},
			ExpectedResponse: babytest.ExpectedResponse{
				Status:     http.StatusOK,
				BodyRegexp: `{"id":"[0-9a-v]{20}","Title":"New Title","Description":"New Description","Completed":false}`,
			},
		},
		{
			Name: "Delete",
			Test: babytest.RequestTest[*babyapi.AnyResource]{
				Method: http.MethodDelete,
				IDFunc: func(getResponse babytest.PreviousResponseGetter) string {
					return getResponse("Create").Data.GetID()
				},
			},
			ExpectedResponse: babytest.ExpectedResponse{
				Status: http.StatusNoContent,
			},
		},
		{
			Name: "NotFoundAfterDelete",
			Test: babytest.RequestTest[*babyapi.AnyResource]{
				Method: http.MethodGet,
				IDFunc: func(getResponse babytest.PreviousResponseGetter) string {
					return getResponse("Create").Data.GetID()
				},
			},
			ExpectedResponse: babytest.ExpectedResponse{
				Status: http.StatusNotFound,
				Error:  "error getting resource: unexpected response with text: Resource not found.",
				Body:   `{"status":"Resource not found."}`,
			},
		},
	})
}
