package extensions

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/calvinmclean/babyapi"
	babytest "github.com/calvinmclean/babyapi/test"

	"github.com/rs/xid"
	"github.com/stretchr/testify/require"
)

type TestType struct {
	babyapi.DefaultResource
	FieldOne string
}

func TestHATEOASResponseRenderAndMarshal(t *testing.T) {
	id, err := xid.FromString("cn0rbolo4027cdoo5jd0")
	require.NoError(t, err)

	tests := []struct {
		name     string
		input    *HATEOASResponse[*TestType]
		method   string
		path     string
		expected string
	}{
		{
			"SuccessfulGetNoChildren",
			&HATEOASResponse[*TestType]{
				Resource: &TestType{
					DefaultResource: babyapi.DefaultResource{ID: babyapi.ID{ID: id}},
					FieldOne:        "ValueOne",
				},
				Links:      map[string]string{},
				childPaths: map[string]string{},
				linkKey:    "links",
			},
			http.MethodGet,
			"/item/cn0rbolo4027cdoo5jd0",
			`{"id":"cn0rbolo4027cdoo5jd0","FieldOne":"ValueOne","links":{"self":"/item/cn0rbolo4027cdoo5jd0"}}`,
		},
		{
			"SuccessfulGetWithChildren",
			&HATEOASResponse[*TestType]{
				Resource: &TestType{
					DefaultResource: babyapi.DefaultResource{ID: babyapi.ID{ID: id}},
					FieldOne:        "ValueOne",
				},
				Links: map[string]string{},
				childPaths: map[string]string{
					"childItem": "/children",
				},
			},
			http.MethodGet,
			"/item/cn0rbolo4027cdoo5jd0",
			`{"id":"cn0rbolo4027cdoo5jd0","FieldOne":"ValueOne","links":{"childItem":"/item/cn0rbolo4027cdoo5jd0/children","self":"/item/cn0rbolo4027cdoo5jd0"}}`,
		},
		{
			"SuccessfulPostNoChildren",
			&HATEOASResponse[*TestType]{
				Resource: &TestType{
					DefaultResource: babyapi.DefaultResource{ID: babyapi.ID{ID: id}},
					FieldOne:        "ValueOne",
				},
				Links:      map[string]string{},
				childPaths: map[string]string{},
			},
			http.MethodPost,
			"/item",
			`{"id":"cn0rbolo4027cdoo5jd0","FieldOne":"ValueOne","links":{"self":"/item/cn0rbolo4027cdoo5jd0"}}`,
		},
		{
			"SuccessfulPostWithChildren",
			&HATEOASResponse[*TestType]{
				Resource: &TestType{
					DefaultResource: babyapi.DefaultResource{ID: babyapi.ID{ID: id}},
					FieldOne:        "ValueOne",
				},
				Links: map[string]string{},
				childPaths: map[string]string{
					"childItem": "/children",
				},
			},
			http.MethodPost,
			"/item",
			`{"id":"cn0rbolo4027cdoo5jd0","FieldOne":"ValueOne","links":{"childItem":"/item/cn0rbolo4027cdoo5jd0/children","self":"/item/cn0rbolo4027cdoo5jd0"}}`,
		},
		{
			"SuccessfulGetWithChildrenAndCustomLinkFunction",
			&HATEOASResponse[*TestType]{
				Resource: &TestType{
					DefaultResource: babyapi.DefaultResource{ID: babyapi.ID{ID: id}},
					FieldOne:        "ValueOne",
				},
				Links: map[string]string{},
				childPaths: map[string]string{
					"childItem": "/children",
				},
				customLinks: func(r *http.Request) map[string]string {
					return map[string]string{
						"new": "/link",
					}
				},
			},
			http.MethodGet,
			"/item/cn0rbolo4027cdoo5jd0",
			`{"id":"cn0rbolo4027cdoo5jd0","FieldOne":"ValueOne","links":{"childItem":"/item/cn0rbolo4027cdoo5jd0/children","new":"/link","self":"/item/cn0rbolo4027cdoo5jd0"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err = tt.input.Render(nil, &http.Request{Method: tt.method, URL: &url.URL{Path: tt.path}})
			require.NoError(t, err)

			data, err := json.Marshal(tt.input)
			require.NoError(t, err)

			require.Equal(t, tt.expected, string(data))
		})
	}
}

func TestAPIWithExtension(t *testing.T) {
	api := babyapi.NewAPI("Test", "/item", func() *TestType { return &TestType{} })
	childAPI := babyapi.NewAPI("Child", "/child", func() *TestType { return &TestType{} })

	ext := HATEOAS[*TestType]{}
	api.ApplyExtension(ext)
	childAPI.ApplyExtension(ext)

	// Add nested API after applying extension to make sure it still works
	api.AddNestedAPI(childAPI)

	babytest.RunTableTest(t, api, []babytest.TestCase[*babyapi.AnyResource]{
		{
			Name: "CreateParent",
			Test: babytest.RequestTest[*babyapi.AnyResource]{
				Method: http.MethodPost,
				Body:   `{"FieldOne": "ValueOne"}`,
			},
			ExpectedResponse: babytest.ExpectedResponse{
				Status:     http.StatusCreated,
				BodyRegexp: `{"id":"[0-9a-v]{20}","FieldOne":"ValueOne","links":{"Child":"/item/[0-9a-v]{20}/child","self":"/item/[0-9a-v]{20}"}}`,
			},
		},
		{
			Name: "GetParent",
			Test: babytest.RequestTest[*babyapi.AnyResource]{
				Method: http.MethodGet,
				IDFunc: func(getResponse babytest.PreviousResponseGetter) string {
					return getResponse("CreateParent").Data.GetID()
				},
			},
			ExpectedResponse: babytest.ExpectedResponse{
				Status:     http.StatusOK,
				BodyRegexp: `{"id":"[0-9a-v]{20}","FieldOne":"ValueOne","links":{"Child":"/item/[0-9a-v]{20}/child","self":"/item/[0-9a-v]{20}"}}`,
			},
		},
		{
			Name: "SearchParents",
			Test: babytest.RequestTest[*babyapi.AnyResource]{
				Method: babyapi.MethodSearch,
			},
			ExpectedResponse: babytest.ExpectedResponse{
				Status:     http.StatusOK,
				BodyRegexp: `{"items":\[{"id":"[0-9a-v]{20}","FieldOne":"ValueOne","links":{"Child":"/item/[0-9a-v]{20}/child","self":"/item/[0-9a-v]{20}"}}\]}`,
			},
		},
		{
			Name: "CreateChild",
			Test: babytest.RequestTest[*babyapi.AnyResource]{
				Method: http.MethodPost,
				ParentIDsFunc: func(getResponse babytest.PreviousResponseGetter) []string {
					return []string{getResponse("CreateParent").Data.GetID()}
				},
				Body: `{"FieldOne": "ValueOne"}`,
			},
			ClientName: "Child",
			ExpectedResponse: babytest.ExpectedResponse{
				Status:     http.StatusCreated,
				BodyRegexp: `{"id":"[0-9a-v]{20}","FieldOne":"ValueOne","links":{"self":"/item/[0-9a-v]{20}/child/[0-9a-v]{20}"}}`,
			},
		},
		{
			Name: "GetChild",
			Test: babytest.RequestTest[*babyapi.AnyResource]{
				Method: http.MethodGet,
				ParentIDsFunc: func(getResponse babytest.PreviousResponseGetter) []string {
					return []string{getResponse("CreateParent").Data.GetID()}
				},
				IDFunc: func(getResponse babytest.PreviousResponseGetter) string {
					return getResponse("CreateChild").Data.GetID()
				},
				Body: `{"FieldOne": "ValueOne"}`,
			},
			ClientName: "Child",
			ExpectedResponse: babytest.ExpectedResponse{
				Status:     http.StatusOK,
				BodyRegexp: `{"id":"[0-9a-v]{20}","FieldOne":"ValueOne","links":{"self":"/item/[0-9a-v]{20}/child/[0-9a-v]{20}"}}`,
			},
		},
		{
			Name: "SearchChildren",
			Test: babytest.RequestTest[*babyapi.AnyResource]{
				Method: babyapi.MethodSearch,
				ParentIDsFunc: func(getResponse babytest.PreviousResponseGetter) []string {
					return []string{getResponse("CreateParent").Data.GetID()}
				},
			},
			ClientName: "Child",
			ExpectedResponse: babytest.ExpectedResponse{
				Status:     http.StatusOK,
				BodyRegexp: `{"items":\[{"id":"[0-9a-v]{20}","FieldOne":"ValueOne","links":{"self":"/item/[0-9a-v]{20}/child/[0-9a-v]{20}"}}\]}`,
			},
		},
	})
}
