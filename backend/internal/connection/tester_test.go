package connection

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAzureSpeechTestIssuesToken(t *testing.T) {
	var gotKey, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey, gotMethod = r.Header.Get("Ocp-Apim-Subscription-Key"), r.Method
		if gotKey != "good" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte("token"))
	}))
	defer srv.Close()

	tester := NewTester(true)
	var gotRegion string
	tester.speechURL = func(region string) string { gotRegion = region; return srv.URL + "/sts/v1.0/issueToken" }

	res := tester.Test(context.Background(), KindAzureSpeech, map[string]string{"region": "eastus"}, "good")
	if !res.OK || gotMethod != http.MethodPost || gotRegion != "eastus" {
		t.Fatalf("result = %+v, method %s, region %s", res, gotMethod, gotRegion)
	}
	res = tester.Test(context.Background(), KindAzureSpeech, map[string]string{"region": "eastus"}, "bad")
	if res.OK || res.Message != "The provider rejected the credentials (HTTP 401)." {
		t.Fatalf("bad key result = %+v", res)
	}
}
