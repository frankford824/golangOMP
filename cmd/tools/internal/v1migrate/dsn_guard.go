package v1migrate

import (
	"fmt"
	"strings"

	"github.com/go-sql-driver/mysql"
)

const ExitCodeR35SafetyViolation = 4

// GuardR35DSN rejects any R3.5 verification DSN that does not point at an
// isolated *_r3_test database.
func GuardR35DSN(dsn string) error {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return fmt.Errorf("parse DSN: %w", err)
	}
	if !strings.HasSuffix(cfg.DBName, "_r3_test") {
		return fmt.Errorf("R3.5 safety violation: DSN database %q must end with '_r3_test'", cfg.DBName)
	}
	return nil
}
