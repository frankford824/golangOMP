package mysqlrepo

import (
	"context"
	"fmt"
	"strings"
)

func productQualifiedColumn(qualifier, column string) string {
	qualifier = strings.TrimSpace(qualifier)
	if qualifier == "" {
		return column
	}
	return fmt.Sprintf("%s.%s", qualifier, column)
}

func productIIDSearchExpr(ctx context.Context, q taskSearchDocumentSQL, qualifier string) string {
	if mysqlColumnExists(ctx, q, "products", "i_id_gen") {
		return productQualifiedColumn(qualifier, "i_id_gen")
	}
	specColumn := productQualifiedColumn(qualifier, "spec_json")
	return fmt.Sprintf("NULLIF(TRIM(CASE WHEN JSON_VALID(%[1]s) THEN JSON_UNQUOTE(JSON_EXTRACT(%[1]s, '$.i_id')) ELSE '' END), '')", specColumn)
}

func productCategoryNameSearchExpr(qualifier string) string {
	specColumn := productQualifiedColumn(qualifier, "spec_json")
	return fmt.Sprintf("NULLIF(TRIM(CASE WHEN JSON_VALID(%[1]s) THEN JSON_UNQUOTE(JSON_EXTRACT(%[1]s, '$.category_name')) ELSE '' END), '')", specColumn)
}
