package shared_redaction_scratch_race_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/application"
	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/httpapi"
	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/redaction"
	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/store"
)

func TestConcurrentCaseQueriesDoNotShareRedactionWorkspace(t *testing.T) {
	repository, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("打开仓储: %v", err)
	}
	service := application.NewService(repository, redaction.New())
	for _, caseID := range []string{"case-concurrent-a", "case-concurrent-b"} {
		payload, marshalErr := json.Marshal(map[string]string{
			"title": caseID, "languageCode": "zh", "collectionContext": "并发复现", "ownerID": "owner-1",
		})
		if marshalErr != nil {
			t.Fatalf("编码建档请求: %v", marshalErr)
		}
		_, executeErr := service.Execute(context.Background(), application.CommandRequest{
			CaseID: caseID, Command: "create_case", ExpectedRevision: 0,
			IdempotencyKey: "create-" + caseID, Actor: application.Actor{ID: "archivist-1", Role: "archivist"},
			Payload: payload,
		})
		if executeErr != nil {
			t.Fatalf("创建案卷 %s: %v", caseID, executeErr)
		}
	}

	handler := httpapi.New(service, slog.New(slog.NewTextHandler(io.Discard, nil)))
	start := make(chan struct{})
	var workers sync.WaitGroup
	for _, caseID := range []string{"case-concurrent-a", "case-concurrent-b"} {
		caseID := caseID
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			for iteration := 0; iteration < 64; iteration++ {
				request := httptest.NewRequest(http.MethodGet, "/api/v1/cases/"+caseID, nil)
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, request)
				if response.Code != http.StatusOK {
					t.Errorf("查询 %s 返回状态码 %d", caseID, response.Code)
					return
				}
			}
		}()
	}
	close(start)
	workers.Wait()
}
