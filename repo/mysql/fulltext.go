package mysqlrepo

import (
	"errors"

	"github.com/go-sql-driver/mysql"
)

const (
	assetWorkbenchSubmissionFullTextMatch = `MATCH(s.submission_no, s.notes) AGAINST (? IN BOOLEAN MODE)`
	assetWorkbenchItemFullTextMatch       = `MATCH(i.order_no, i.template_name_snapshot, i.category_snapshot, i.difficulty_class) AGAINST (? IN BOOLEAN MODE)`
	assetWorkbenchFileFullTextMatch       = `MATCH(f.original_filename, f.display_name, f.relative_path, f.file_type, f.mime_type, f.upload_directory_name) AGAINST (? IN BOOLEAN MODE)`
)

func assetWorkbenchSubmissionFullTextMatchFor(alias string) string {
	return `MATCH(` + alias + `.submission_no, ` + alias + `.notes) AGAINST (? IN BOOLEAN MODE)`
}

func assetWorkbenchItemFullTextMatchFor(alias string) string {
	return `MATCH(` + alias + `.order_no, ` + alias + `.template_name_snapshot, ` + alias + `.category_snapshot, ` + alias + `.difficulty_class) AGAINST (? IN BOOLEAN MODE)`
}

func assetWorkbenchFileFullTextMatchFor(alias string) string {
	return `MATCH(` + alias + `.original_filename, ` + alias + `.display_name, ` + alias + `.relative_path, ` + alias + `.file_type, ` + alias + `.mime_type, ` + alias + `.upload_directory_name) AGAINST (? IN BOOLEAN MODE)`
}

func isMySQLFullTextIndexMissing(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1191
}
