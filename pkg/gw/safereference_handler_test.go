package gw

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	immuclient "github.com/codenotary/immudb/pkg/client"
	immudb "github.com/codenotary/immudb/pkg/server"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
)

func generateRandomTCPPort() int {
	rand.Seed(time.Now().UnixNano())
	min := 1024
	max := 64000
	return rand.Intn(max - min + 1)
}
func insertSampleSet() (string, error) {
	key := base64.StdEncoding.EncodeToString([]byte("Pablo"))
	value := base64.StdEncoding.EncodeToString([]byte("Picasso"))
	ic, err := immuclient.NewImmuClient(immuclient.DefaultOptions())
	if err != nil {
		return "", fmt.Errorf("unable to instantiate client: %s", err)
	}
	mux := runtime.NewServeMux()
	ssh := NewSafesetHandler(mux, ic)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/immurestproxy/item/safe", strings.NewReader(`{"kv": {"key": "`+key+`", "value": "`+value+`"} }`))
	req.Header.Add("Content-Type", "application/json")

	safeset := func(res http.ResponseWriter, req *http.Request) {
		ssh.Safeset(res, req, nil)
	}
	handler := http.HandlerFunc(safeset)
	handler.ServeHTTP(w, req)
	var body map[string]interface{}

	err = json.Unmarshal(w.Body.Bytes(), &body)
	if err != nil {
		return "", errors.New("bad reply JSON")
	}
	fieldValue, ok := body["verified"]
	if !ok {
		return "", errors.New("json reply required field not found")
	}
	if fieldValue != true {
		return "", errors.New("Error inserting key")
	}
	return key, nil
}

func TestSafeReference(t *testing.T) {
	op := immudb.DefaultOptions().WithPort(generateRandomTCPPort()).WithDir("db_" + strconv.FormatInt(int64(generateRandomTCPPort()), 10))
	s := immudb.DefaultServer().WithOptions(op)
	go s.Start()
	time.Sleep(2 * time.Second)
	defer s.Stop()
	defer os.RemoveAll(op.Dir)

	refKey, err := insertSampleSet()
	if err != nil {
		t.Errorf("%s", err)
	}

	ic, err := immuclient.NewImmuClient(immuclient.DefaultOptions())
	if err != nil {
		t.Errorf("unable to instantiate client: %s", err)
		return
	}
	mux := runtime.NewServeMux()
	ssh := NewSafeReferenceHandler(mux, ic)

	tt := []struct {
		test           string
		payload        string
		wantStatus     int
		wantFieldName  string
		wantFieldValue interface{}
	}{
		{
			"Send correct request",
			`{
				"ro": {
					"reference":  "` + refKey + `",
					 "key": "` + refKey + `"
				}
			}`,
			200,
			"verified",
			true,
		},
		{
			"Send correct request",
			`{
				"ro": {
					"reference":  "` + refKey + `",
					 "key": "` + base64.StdEncoding.EncodeToString([]byte("The Stars Look Down")) + `"
				}
			}`,
			400,
			"error",
			"Key not found",
		},
		{
			"Send incorrect json field",
			`{"data": {"key": "UGFibG8=", "reference": "UGljYXNzbw==" } }`,
			400,
			"error",
			"incorrect JSON payload",
		},
		{
			"Missing Key field",
			`{
				"ro": {
					"reference":  "UGFibG8="
				}
			}`,
			400,
			"error",
			"invalid key",
		},
		{
			"Send ASCII instead of base64 encoded",
			`{
				"ro": {
					"reference":  "Archibald Cronin",
					 "key": "` + refKey + `"
				}
			}`,
			400,
			"error",
			"illegal base64 data at input byte 9",
		},
	}

	for _, tc := range tt {
		t.Run(tc.test, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/v1/immurestproxy/safe/reference", strings.NewReader(tc.payload))
			req.Header.Add("Content-Type", "application/json")

			safeset := func(res http.ResponseWriter, req *http.Request) {
				ssh.SafeReference(res, req, nil)
			}
			handler := http.HandlerFunc(safeset)
			handler.ServeHTTP(w, req)
			var body map[string]interface{}

			err = json.Unmarshal(w.Body.Bytes(), &body)
			if err != nil {
				t.Error("bad reply JSON")
			}
			fieldValue, ok := body[tc.wantFieldName]
			if !ok {
				t.Errorf("json reply required field not found")
			}
			if fieldValue != tc.wantFieldValue {
				t.Errorf("handler returned wrong json reply: got %v want %v",
					fieldValue, tc.wantFieldValue)
				t.Error(body)
				t.Error(string(w.Body.Bytes()))
			}

			// TODO gjergji this should be used once #263 is fixed
			// if w.Code != tc.want {
			// 	t.Errorf("handler returned wrong status code: got %v want %v",
			// 		w.Code, tc.want)
			// }
		})
	}
}
