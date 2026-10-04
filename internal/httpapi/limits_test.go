package httpapi_test

import (
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/Lewin671/keduly/internal/core"
)

func TestRowCaps(t *testing.T) {
	s, me := setup(t)
	s.Service.Limits = core.Limits{Items: 2, Events: 1, Projects: 1, Tokens: 1}
	mkItem(t, me, M{"title": "one"})
	mkItem(t, me, M{"title": "two"})
	wantCode(t, me, "POST", "/items", M{"title": "three"}, 400, "invalid_request")
	mkEvent(t, me, M{"title": "one", "start": "2026-10-13T02:00:00Z", "end": "2026-10-13T03:00:00Z"})
	wantCode(t, me, "POST", "/events", M{"title": "two", "start": "2026-10-13T02:00:00Z", "end": "2026-10-13T03:00:00Z"}, 400, "invalid_request")
	mkProject(t, me, "one")
	wantCode(t, me, "POST", "/projects", M{"name": "two"}, 400, "invalid_request")
	me.NewToken("one", "agent", "write", true)
	status, data, _ := me.Do("POST", "/tokens", M{"name": "two", "kind": "agent"})
	if status != http.StatusBadRequest || !contains(string(data), "at most 1 tokens") {
		t.Fatalf("token cap: %d %s", status, data)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestConcurrentWrites(t *testing.T) {
	_, me := setup(t)
	start := revision(t, me)
	var wg sync.WaitGroup
	errs := make(chan string, 40)
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			status, data, _ := me.Do("POST", "/items", M{"title": fmt.Sprintf("item %d", i)})
			if status != http.StatusCreated {
				errs <- fmt.Sprintf("%d %s", status, data)
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Fatalf("a concurrent write failed: %s", e)
	}
	var list itemList
	me.Call("GET", "/items?limit=200", nil, http.StatusOK, &list)
	if list.Total != 40 || revision(t, me) != start+40 || len(activities(t, me)) != 40 {
		t.Fatalf("total %d, revision %d -> %d", list.Total, start, revision(t, me))
	}
}
