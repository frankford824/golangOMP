package orders

import (
	"database/sql"
	"testing"
)

func TestCostMoneyAndVersion(t *testing.T) {
	for _, amount := range []string{"0.00", "12.34", "9999999999.99"} {
		if !costAmount.MatchString(amount) {
			t.Errorf("rejected %s", amount)
		}
	}
	for _, amount := range []string{"-1.00", "1", "1.0", "01.20", "10000000000.00", "NaN", "1.000"} {
		if costAmount.MatchString(amount) {
			t.Errorf("accepted %s", amount)
		}
	}
	zero := sql.NullString{String: "0.00", Valid: true}
	original := costVersion(1, 1, zero, "2026-11-18 13:00:00")
	if original == costVersion(1, 1, sql.NullString{}, "2026-11-18 13:00:00") || original == costVersion(1, 2, zero, "2026-11-18 13:00:00") {
		t.Fatal("version collapsed separate cost states")
	}
}
