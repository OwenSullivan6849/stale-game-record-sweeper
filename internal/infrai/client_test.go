package infrai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestCreateCronRetriesWithStableRequest(t *testing.T) {
	var calls int
	var keys []string
	testCredential := t.Name()
	wantBody := map[string]string{
		"cron_expr": "17 * * * *",
		"task":      "https://games.example.net/maintenance/stale-records",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer "+testCredential {
			t.Fatal("missing bearer authorization")
		}
		keys = append(keys, r.Header.Get("Idempotency-Key"))

		var gotBody map[string]string
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(gotBody, wantBody) {
			t.Fatalf("body = %#v", gotBody)
		}

		w.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"data":{"job_id":"job_sweep_01"},"error":null,"metadata":{}}`))
	}))
	defer server.Close()

	client, err := New(testCredential)
	if err != nil {
		t.Fatal(err)
	}
	client.baseURL = server.URL
	client.sleep = func(context.Context, time.Duration) error { return nil }

	got, err := client.CreateCron(context.Background(), CronCreateRequest{
		CronExpr: wantBody["cron_expr"],
		Task:     wantBody["task"],
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.JobID != "job_sweep_01" {
		t.Fatalf("job ID = %q", got.JobID)
	}
	if calls != 2 || keys[0] == "" || keys[0] != keys[1] {
		t.Fatalf("calls = %d, idempotency keys = %#v", calls, keys)
	}
}
