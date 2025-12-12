package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestPaginated_ComputesTotalPagesCorrectly(t *testing.T) {
	e := echo.New()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	c := e.NewContext(req, rec)

	// totalCount=45, pageSize=20 -> totalPages = 3
	data := []int{1, 2, 3}
	page := 2
	pageSize := 20
	var totalCount int64 = 45

	if err := Paginated(c, data, page, pageSize, totalCount); err != nil {
		t.Fatalf("Paginated returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body PaginatedResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !body.Success {
		t.Errorf("expected Success=true, got false")
	}
	if body.Page != page {
		t.Errorf("expected Page=%d, got %d", page, body.Page)
	}
	if body.PageSize != pageSize {
		t.Errorf("expected PageSize=%d, got %d", pageSize, body.PageSize)
	}
	if body.TotalCount != totalCount {
		t.Errorf("expected TotalCount=%d, got %d", totalCount, body.TotalCount)
	}
	if body.TotalPages != 3 {
		t.Errorf("expected TotalPages=3, got %d", body.TotalPages)
	}
}
