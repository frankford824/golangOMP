package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Report struct {
	Version         string       `json:"version"`
	GeneratedAt     string       `json:"generated_at"`
	OpenAPISHA256   string       `json:"openapi_sha256"`
	TransportSHA256 string       `json:"transport_sha256"`
	Summary         Summary      `json:"summary"`
	Paths           []PathReport `json:"paths"`
	Unmapped        []GapReport  `json:"unmapped"`
	KnownGap        []GapReport  `json:"known_gap"`
}

type Summary struct {
	TotalPaths       int `json:"total_paths"`
	Clean            int `json:"clean"`
	Drift            int `json:"drift"`
	Unmapped         int `json:"unmapped"`
	KnownGap         int `json:"known_gap"`
	MissingInOpenAPI int `json:"missing_in_openapi"`
	MissingInCode    int `json:"missing_in_code"`
}

type PathReport struct {
	Method        string   `json:"method"`
	Path          string   `json:"path"`
	OpenAPIPath   string   `json:"openapi_path"`
	Handler       string   `json:"handler"`
	ResponseType  string   `json:"response_type"`
	CodeFields    []string `json:"code_fields"`
	OpenAPIFields []string `json:"openapi_fields"`
	OnlyInCode    []string `json:"only_in_code"`
	OnlyInOpenAPI []string `json:"only_in_openapi"`
	TypeMismatch  []string `json:"type_mismatch,omitempty"`
	Verdict       string   `json:"verdict"`
}

type GapReport struct {
	Method  string `json:"method"`
	Path    string `json:"path"`
	Handler string `json:"handler,omitempty"`
	Class   string `json:"class,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

type Route struct {
	Method      string
	Path        string
	HandlerExpr string
	HandlerType string
	Mount       string
}

type Operation struct {
	Method string
	Path   string
	Fields []string
}

type HandlerIndex struct {
	Funcs        map[string]*ast.FuncDecl
	FileByMethod map[string]string
	ReturnTypes  map[string]string
}

type StructIndex struct {
	FieldsByType map[string][]string
	FileByType   map[string]string
	RawByType    map[string]*ast.StructType
}

type OpenAPIDoc struct {
	Components map[string]any
	Operations []Operation
}

type ResponseShape struct {
	Type        string
	ExtraFields []string
}

func main() {
	var transport, handlers, domain, openapi, output, markdown string
	var failOnDrift bool
	flag.StringVar(&transport, "transport", "transport/http.go", "transport/http.go path")
	flag.StringVar(&handlers, "handlers", "transport/handler", "handler directory")
	flag.StringVar(&domain, "domain", "domain", "domain directory")
	flag.StringVar(&openapi, "openapi", "docs/api/openapi.yaml", "OpenAPI yaml path")
	flag.StringVar(&output, "output", "docs/iterations/V1_2_CONTRACT_AUDIT_v2.json", "JSON output path")
	flag.StringVar(&markdown, "markdown", "docs/iterations/V1_2_CONTRACT_AUDIT_v2.md", "Markdown output path")
	flag.BoolVar(&failOnDrift, "fail-on-drift", false, "exit non-zero when drift exists")
	flag.Parse()

	report, err := BuildReport(transport, handlers, domain, openapi)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if output != "" {
		if err := writeJSON(output, report); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
	if markdown != "" {
		if err := writeMarkdown(markdown, report); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
	if output == "" && markdown == "" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(report)
	}
	if failOnDrift && report.Summary.Drift > 0 {
		os.Exit(1)
	}
}

func BuildReport(transport, handlers, domain, openapiPath string) (Report, error) {
	routes, err := ParseTransportRoutes(transport)
	if err != nil {
		return Report{}, err
	}
	ops, err := OpenAPIOperations(openapiPath)
	if err != nil {
		return Report{}, err
	}
	handlerIndex, err := BuildHandlerIndex(handlers)
	if err != nil {
		return Report{}, err
	}
	returnTypes, err := BuildReturnTypeIndex([]string{handlers, domain, "service"})
	if err != nil {
		return Report{}, err
	}
	for name, typ := range returnTypes {
		handlerIndex.ReturnTypes[name] = typ
	}
	structIndex, err := BuildStructIndex([]string{domain, "service", handlers})
	if err != nil {
		return Report{}, err
	}

	mounted := map[string]Route{}
	for _, r := range routes {
		mounted[methodPath(r.Method, ginToOpenAPIPath(r.Path))] = r
	}
	documented := map[string]Operation{}
	for _, op := range ops {
		documented[methodPath(op.Method, op.Path)] = op
	}

	keys := make(map[string]bool)
	for k := range mounted {
		keys[k] = true
	}
	for k := range documented {
		keys[k] = true
	}
	ordered := make([]string, 0, len(keys))
	for k := range keys {
		ordered = append(ordered, k)
	}
	sort.Strings(ordered)

	report := Report{
		Version:         "v1.2-C",
		GeneratedAt:     time.Now().UTC().Format(time.RFC3339),
		OpenAPISHA256:   fileSHA(openapiPath),
		TransportSHA256: fileSHA(transport),
	}
	for _, key := range ordered {
		method, openapiPathKey := splitMethodPath(key)
		route, hasRoute := mounted[key]
		op, hasOp := documented[key]
		pr := PathReport{Method: method, Path: openapiToGinPath(openapiPathKey), OpenAPIPath: openapiPathKey}
		var codeFields, openapiFields []string
		var hVerdict, reason string
		if hasRoute {
			pr.Path = route.Path
			pr.Handler = route.Mount + " " + displayHandler(route)
			shapes, hv, hr := ResolveHandlerResponseShapes(handlerIndex, route)
			hVerdict, reason = hv, hr
			pr.ResponseType = displayResponseShapes(shapes)
			if hVerdict == "" {
				var missing []string
				var fieldSets [][]string
				for _, shape := range shapes {
					fields := append([]string{}, structIndex.Fields(shape.Type)...)
					fields = append(fields, shape.ExtraFields...)
					fields = normalizeFields(fields)
					if len(fields) == 0 {
						missing = append(missing, shape.Type)
						continue
					}
					fieldSets = append(fieldSets, fields)
					codeFields = append(codeFields, fields...)
				}
				if len(fieldSets) == 0 {
					if len(missing) == 0 {
						missing = append(missing, pr.ResponseType)
					}
					hVerdict, reason = "unmapped_handler", "response struct not found: "+strings.Join(missing, ",")
				} else if inconsistentFieldSets(fieldSets) {
					hVerdict, reason = "multi_exit_inconsistent", "multiple response exits expose different fields: "+pr.ResponseType
				}
			}
		}
		if hasOp {
			openapiFields = op.Fields
		}
		pr.CodeFields = normalizeFields(codeFields)
		pr.OpenAPIFields = normalizeFields(openapiFields)

		switch {
		case !hasRoute:
			pr.Verdict = "mounted_not_found"
			report.Summary.KnownGap++
			report.Summary.MissingInCode++
			report.KnownGap = append(report.KnownGap, GapReport{Method: method, Path: openapiPathKey, Class: "documented_not_mounted"})
		case !hasOp:
			pr.Verdict = "documented_not_found"
			report.Summary.KnownGap++
			report.Summary.MissingInOpenAPI++
			report.KnownGap = append(report.KnownGap, GapReport{Method: method, Path: route.Path, Handler: displayHandler(route), Class: "mounted_not_documented"})
		case hVerdict != "":
			pr.Verdict = hVerdict
			report.Summary.Unmapped++
			report.Unmapped = append(report.Unmapped, GapReport{Method: method, Path: route.Path, Handler: displayHandler(route), Reason: reason})
		default:
			pr.OnlyInCode, pr.OnlyInOpenAPI = DiffFields(pr.CodeFields, pr.OpenAPIFields)
			pr.Verdict = decideVerdict(pr.CodeFields, pr.OpenAPIFields, pr.OnlyInCode, pr.OnlyInOpenAPI)
			if isDrift(pr.Verdict) {
				report.Summary.Drift++
			} else {
				report.Summary.Clean++
			}
		}
		report.Paths = append(report.Paths, pr)
	}
	report.Summary.TotalPaths = len(report.Paths)
	return report, nil
}

func ParseTransportRoutes(transportPath string) ([]Route, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, transportPath, nil, 0)
	if err != nil {
		return nil, err
	}
	groups := map[string]string{"r": "", "v1": "/v1", "ws": "/ws"}
	handlerTypes := transportHandlerTypes(f)
	var routes []Route

	ast.Inspect(f, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i, rhs := range as.Rhs {
			call, ok := rhs.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				continue
			}
			recv, name, ok := selectorName(call.Fun)
			if !ok || name != "Group" {
				continue
			}
			rel, ok := stringLiteral(call.Args[0])
			if !ok {
				continue
			}
			if i < len(as.Lhs) {
				if id, ok := as.Lhs[i].(*ast.Ident); ok {
					groups[id.Name] = joinPath(groups[recv], rel)
				}
			}
		}
		return true
	})

	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		recv, method, ok := selectorName(call.Fun)
		if !ok || !isHTTPMethod(method) {
			return true
		}
		base, ok := groups[recv]
		if !ok {
			return true
		}
		rel, ok := stringLiteral(call.Args[0])
		if !ok {
			return true
		}
		handlerExpr, handlerType := lastHandlerSelector(call.Args, handlerTypes)
		pos := fset.Position(call.Pos())
		routes = append(routes, Route{Method: method, Path: joinPath(base, rel), HandlerExpr: handlerExpr, HandlerType: handlerType, Mount: fmt.Sprintf("%s:%d", transportPath, pos.Line)})
		return true
	})

	// Dynamic reserved routes are real mounts, but their response is intentionally unmapped.
	ast.Inspect(f, func(n ast.Node) bool {
		cl, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		for _, elt := range cl.Elts {
			item, ok := elt.(*ast.CompositeLit)
			if !ok {
				continue
			}
			var groupBase, method, rel string
			var overlaps bool
			for _, e := range item.Elts {
				kv, ok := e.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				key, ok := kv.Key.(*ast.Ident)
				if !ok {
					continue
				}
				switch key.Name {
				case "GroupBase":
					groupBase, _ = stringLiteral(kv.Value)
				case "Method":
					method, _ = httpMethodExpr(kv.Value)
				case "RelativePath":
					rel, _ = stringLiteral(kv.Value)
				case "OverlapsLiveRoute":
					if id, ok := kv.Value.(*ast.Ident); ok && id.Name == "true" {
						overlaps = true
					}
				}
			}
			if groupBase != "" && method != "" && rel != "" && !overlaps {
				routes = append(routes, Route{Method: method, Path: joinPath(groupBase, rel), HandlerExpr: "v1R1ReservedHandler", HandlerType: "", Mount: transportPath})
			}
		}
		return true
	})
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Path == routes[j].Path {
			return routes[i].Method < routes[j].Method
		}
		return routes[i].Path < routes[j].Path
	})
	return routes, nil
}

func BuildHandlerIndex(dir string) (HandlerIndex, error) {
	idx := HandlerIndex{Funcs: map[string]*ast.FuncDecl{}, FileByMethod: map[string]string{}, ReturnTypes: map[string]string{}}
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		indexFunctions(idx, f, path)
		return nil
	})
	return idx, err
}

func BuildStructIndex(roots []string) (StructIndex, error) {
	idx := StructIndex{FieldsByType: map[string][]string{}, FileByType: map[string]string{}, RawByType: map[string]*ast.StructType{}}
	for _, root := range roots {
		if root == "" {
			continue
		}
		if _, err := os.Stat(root); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return idx, err
		}
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			for _, decl := range f.Decls {
				gen, ok := decl.(*ast.GenDecl)
				if !ok {
					continue
				}
				for _, spec := range gen.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					if st, ok := ts.Type.(*ast.StructType); ok {
						idx.RawByType[ts.Name.Name] = st
						idx.FileByType[ts.Name.Name] = path
					}
				}
			}
			return nil
		})
		if err != nil {
			return idx, err
		}
	}
	for name, st := range idx.RawByType {
		idx.FieldsByType[name] = idx.structFields(st, map[string]bool{name: true})
	}
	return idx, nil
}

func BuildReturnTypeIndex(roots []string) (map[string]string, error) {
	out := map[string]string{}
	for _, root := range roots {
		if root == "" {
			continue
		}
		if _, err := os.Stat(root); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return out, err
		}
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			for _, decl := range f.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Type.Results == nil || len(fn.Type.Results.List) == 0 {
					continue
				}
				out[fn.Name.Name] = resultType(fn.Type.Results.List[0].Type)
			}
			return nil
		})
		if err != nil {
			return out, err
		}
	}
	return out, nil
}

func ResolveHandlerResponseShapes(idx HandlerIndex, route Route) (shapes []ResponseShape, verdict, reason string) {
	if route.HandlerExpr == "" {
		return nil, "unmapped_handler_dynamic_payload", "route handler is inline or middleware slice"
	}
	if route.HandlerExpr == "v1R1ReservedHandler" {
		return nil, "unmapped_handler_dynamic_payload", "reserved route handler"
	}
	parts := strings.Split(route.HandlerExpr, ".")
	if len(parts) != 2 || route.HandlerType == "" {
		return nil, "unmapped_handler", "handler receiver type not resolved"
	}
	key := route.HandlerType + "." + parts[1]
	fn := idx.Funcs[key]
	if fn == nil {
		return nil, "unmapped_handler", "handler method not found: " + key
	}
	responses := respondExprs(fn)
	if len(responses) == 0 {
		return nil, "unmapped_handler", "no respondOK/respondCreated/respondOKWithPagination call"
	}
	local := localTypes(fn, idx.ReturnTypes)
	for _, resp := range responses {
		typ := inferExprType(resp.Expr, local, idx.ReturnTypes)
		if typ == "map" || typ == "gin.H" {
			return nil, "unmapped_handler_dynamic_payload", "dynamic payload"
		}
		if typ == "" {
			return nil, "unmapped_handler", "response expression type not inferred"
		}
		shape := ResponseShape{Type: baseTypeName(typ)}
		if resp.Pagination {
			shape.ExtraFields = append(shape.ExtraFields, "pagination")
		}
		shapes = append(shapes, shape)
	}
	return uniqueResponseShapes(shapes), "", ""
}

func OpenAPIOperations(path string) ([]Operation, error) {
	doc, err := loadOpenAPI(path)
	if err != nil {
		return nil, err
	}
	methods := map[string]string{"get": "GET", "post": "POST", "put": "PUT", "patch": "PATCH", "delete": "DELETE", "head": "HEAD"}
	var out []Operation
	paths, _ := doc["paths"].(map[string]any)
	components, _ := doc["components"].(map[string]any)
	for p, raw := range paths {
		item, _ := raw.(map[string]any)
		for m, upper := range methods {
			if op, ok := item[m].(map[string]any); ok {
				out = append(out, Operation{Method: upper, Path: p, Fields: responseFieldsExpanded(op, components)})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path == out[j].Path {
			return out[i].Method < out[j].Method
		}
		return out[i].Path < out[j].Path
	})
	return out, nil
}

func responseFieldsExpanded(op map[string]any, components map[string]any) []string {
	resp, _ := op["responses"].(map[string]any)
	ok200, _ := resp["200"].(map[string]any)
	content, _ := ok200["content"].(map[string]any)
	app, _ := content["application/json"].(map[string]any)
	schema, _ := app["schema"].(map[string]any)
	root := resolveSchema(schema, components, map[string]bool{})
	props, _ := root["properties"].(map[string]any)
	if data, ok := props["data"].(map[string]any); ok {
		var fields []string
		data = resolveSchema(data, components, map[string]bool{})
		if data["type"] == "array" {
			if items, ok := data["items"].(map[string]any); ok {
				data = resolveSchema(items, components, map[string]bool{})
			}
		}
		if dp, ok := data["properties"].(map[string]any); ok {
			fields = append(fields, sortedKeys(dp)...)
		}
		if _, ok := props["pagination"]; ok {
			fields = append(fields, "pagination")
		}
		if len(fields) > 0 {
			return normalizeFields(fields)
		}
		return nil
	}
	return normalizeFields(sortedKeys(props))
}

func resolveSchema(schema map[string]any, components map[string]any, seen map[string]bool) map[string]any {
	if schema == nil {
		return nil
	}
	if ref, ok := schema["$ref"].(string); ok {
		name := strings.TrimPrefix(ref, "#/components/schemas/")
		if seen[name] {
			return nil
		}
		seen[name] = true
		schemas, _ := components["schemas"].(map[string]any)
		target, _ := schemas[name].(map[string]any)
		return resolveSchema(target, components, seen)
	}
	if all, ok := schema["allOf"].([]any); ok {
		merged := map[string]any{"properties": map[string]any{}}
		props := merged["properties"].(map[string]any)
		for _, item := range all {
			child, _ := item.(map[string]any)
			resolved := resolveSchema(child, components, copySeen(seen))
			for k, v := range resolved {
				if k != "properties" {
					merged[k] = v
				}
			}
			if cp, ok := resolved["properties"].(map[string]any); ok {
				for k, v := range cp {
					props[k] = v
				}
			}
		}
		return merged
	}
	return schema
}

func StructJSONFields(goFile, typeName string) ([]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, goFile, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	for _, decl := range f.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gen.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name.Name != typeName {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				return nil, fmt.Errorf("%s is not a struct", typeName)
			}
			return structFields(st), nil
		}
	}
	return nil, fmt.Errorf("type %s not found in %s", typeName, goFile)
}

func DiffFields(code, openapi []string) (onlyCode, onlyOpenAPI []string) {
	c := map[string]bool{}
	o := map[string]bool{}
	for _, x := range normalizeFields(code) {
		c[x] = true
	}
	for _, x := range normalizeFields(openapi) {
		o[x] = true
	}
	for x := range c {
		if !o[x] {
			onlyCode = append(onlyCode, x)
		}
	}
	for x := range o {
		if !c[x] {
			onlyOpenAPI = append(onlyOpenAPI, x)
		}
	}
	sort.Strings(onlyCode)
	sort.Strings(onlyOpenAPI)
	return
}

func (idx StructIndex) Fields(typeName string) []string {
	typeName = baseTypeName(typeName)
	return append([]string(nil), idx.FieldsByType[typeName]...)
}

func (idx StructIndex) structFields(st *ast.StructType, seen map[string]bool) []string {
	var fields []string
	for _, field := range st.Fields.List {
		name := ""
		if field.Tag != nil {
			name = jsonTagName(strings.Trim(field.Tag.Value, "`"))
		}
		if name != "" {
			if name != "-" {
				fields = append(fields, name)
			}
			continue
		}
		if len(field.Names) == 0 {
			embedded := baseTypeName(resultType(field.Type))
			if embedded == "" || seen[embedded] {
				continue
			}
			child := idx.RawByType[embedded]
			if child == nil {
				continue
			}
			seen[embedded] = true
			fields = append(fields, idx.structFields(child, seen)...)
			delete(seen, embedded)
		}
	}
	return normalizeFields(fields)
}

func indexFunctions(idx HandlerIndex, f *ast.File, path string) {
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fn.Recv != nil && len(fn.Recv.List) > 0 {
			recv := baseTypeName(exprString(fn.Recv.List[0].Type))
			idx.Funcs[recv+"."+fn.Name.Name] = fn
			idx.FileByMethod[recv+"."+fn.Name.Name] = path
		}
		if fn.Type.Results != nil && len(fn.Type.Results.List) > 0 {
			idx.ReturnTypes[fn.Name.Name] = resultType(fn.Type.Results.List[0].Type)
		}
	}
}

type respondExpr struct {
	Expr       ast.Expr
	Pagination bool
}

func respondExprs(fn *ast.FuncDecl) []respondExpr {
	var found []respondExpr
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) < 2 {
			return true
		}
		id, ok := call.Fun.(*ast.Ident)
		if !ok {
			return true
		}
		switch id.Name {
		case "respondOK", "respondCreated":
			found = append(found, respondExpr{Expr: call.Args[1]})
		case "respondOKWithPagination":
			found = append(found, respondExpr{Expr: call.Args[1], Pagination: true})
		}
		return true
	})
	return found
}

func localTypes(fn *ast.FuncDecl, returns map[string]string) map[string]string {
	local := map[string]string{}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.DeclStmt:
			gen, ok := x.Decl.(*ast.GenDecl)
			if !ok {
				return true
			}
			for _, spec := range gen.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				typ := resultType(vs.Type)
				for _, name := range vs.Names {
					if typ != "" {
						local[name.Name] = typ
					}
				}
			}
		case *ast.AssignStmt:
			for i, lhs := range x.Lhs {
				id, ok := lhs.(*ast.Ident)
				if !ok || id.Name == "_" {
					continue
				}
				if i < len(x.Rhs) {
					if typ := inferExprType(x.Rhs[i], local, returns); typ != "" {
						local[id.Name] = typ
					}
				} else if len(x.Rhs) == 1 {
					if typ := inferExprType(x.Rhs[0], local, returns); typ != "" {
						local[id.Name] = typ
					}
				}
			}
		}
		return true
	})
	return local
}

func inferExprType(expr ast.Expr, local, returns map[string]string) string {
	switch x := expr.(type) {
	case *ast.UnaryExpr:
		return inferExprType(x.X, local, returns)
	case *ast.CompositeLit:
		return resultType(x.Type)
	case *ast.Ident:
		return local[x.Name]
	case *ast.CallExpr:
		if fun, ok := x.Fun.(*ast.Ident); ok && fun.Name == "make" && len(x.Args) > 0 {
			return resultType(x.Args[0])
		}
		if name := methodName(x.Fun); name != "" {
			if typ := returns[name]; typ != "" {
				return typ
			}
		}
		if id, ok := x.Fun.(*ast.Ident); ok {
			return returns[id.Name]
		}
	}
	return ""
}

func transportHandlerTypes(f *ast.File) map[string]string {
	out := map[string]string{}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "NewRouter" || fn.Type.Params == nil {
			continue
		}
		for _, field := range fn.Type.Params.List {
			typ := baseTypeName(exprString(field.Type))
			for _, name := range field.Names {
				out[name.Name] = typ
			}
		}
	}
	return out
}

func lastHandlerSelector(args []ast.Expr, types map[string]string) (expr, handlerType string) {
	for i := len(args) - 1; i >= 1; i-- {
		recv, name, ok := selectorName(args[i])
		if ok {
			return recv + "." + name, types[recv]
		}
	}
	return "", ""
}

func loadOpenAPI(path string) (map[string]any, error) {
	var doc map[string]any
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func structFields(st *ast.StructType) []string {
	var fields []string
	for _, field := range st.Fields.List {
		if field.Tag == nil {
			continue
		}
		name := jsonTagName(strings.Trim(field.Tag.Value, "`"))
		if name != "" && name != "-" {
			fields = append(fields, name)
		}
	}
	return normalizeFields(fields)
}

func uniqueResponseShapes(in []ResponseShape) []ResponseShape {
	seen := map[string]bool{}
	var out []ResponseShape
	for _, shape := range in {
		key := shape.Type + "|" + strings.Join(normalizeFields(shape.ExtraFields), ",")
		if seen[key] {
			continue
		}
		seen[key] = true
		shape.ExtraFields = normalizeFields(shape.ExtraFields)
		out = append(out, shape)
	}
	return out
}

func displayResponseShapes(shapes []ResponseShape) string {
	if len(shapes) == 0 {
		return ""
	}
	parts := make([]string, 0, len(shapes))
	for _, shape := range shapes {
		part := shape.Type
		if len(shape.ExtraFields) > 0 {
			part += "+" + strings.Join(shape.ExtraFields, "+")
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, " | ")
}

func inconsistentFieldSets(sets [][]string) bool {
	if len(sets) < 2 {
		return false
	}
	first := strings.Join(normalizeFields(sets[0]), "\x00")
	for _, set := range sets[1:] {
		if strings.Join(normalizeFields(set), "\x00") != first {
			return true
		}
	}
	return false
}

func decideVerdict(code, openapi, onlyCode, onlyOpenAPI []string) string {
	switch {
	case len(code) == 0 && len(openapi) == 0:
		return "clean_empty"
	case len(onlyCode) == 0 && len(onlyOpenAPI) == 0:
		return "clean"
	case len(onlyCode) > 0 && len(onlyOpenAPI) == 0:
		return "only_in_code"
	case len(onlyCode) == 0 && len(onlyOpenAPI) > 0:
		return "only_in_openapi"
	default:
		return "both_diff"
	}
}

func isDrift(v string) bool {
	return v == "only_in_code" || v == "only_in_openapi" || v == "both_diff"
}

func methodPath(method, path string) string {
	return strings.ToUpper(method) + " " + path
}

func splitMethodPath(key string) (string, string) {
	parts := strings.SplitN(key, " ", 2)
	if len(parts) != 2 {
		return key, ""
	}
	return parts[0], parts[1]
}

func ginToOpenAPIPath(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, ":") {
			parts[i] = "{" + strings.TrimPrefix(part, ":") + "}"
		}
		if strings.HasPrefix(part, "*") {
			parts[i] = "{" + strings.TrimPrefix(part, "*") + "}"
		}
	}
	return strings.Join(parts, "/")
}

func openapiToGinPath(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			parts[i] = ":" + strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}")
		}
	}
	return strings.Join(parts, "/")
}

func joinPath(base, rel string) string {
	if base == "/" {
		base = ""
	}
	if rel == "" {
		return normalizePath(base)
	}
	return normalizePath(strings.TrimRight(base, "/") + "/" + strings.TrimLeft(rel, "/"))
}

func normalizePath(path string) string {
	if path == "" {
		return "/"
	}
	for strings.Contains(path, "//") {
		path = strings.ReplaceAll(path, "//", "/")
	}
	return path
}

func selectorName(expr ast.Expr) (recv, name string, ok bool) {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return "", "", false
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return "", "", false
	}
	return id.Name, sel.Sel.Name, true
}

func methodName(expr ast.Expr) string {
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		return sel.Sel.Name
	}
	if id, ok := expr.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

func stringLiteral(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	return strings.Trim(lit.Value, `"`), true
}

func isHTTPMethod(name string) bool {
	switch name {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD":
		return true
	default:
		return false
	}
}

func httpMethodExpr(expr ast.Expr) (string, bool) {
	if recv, name, ok := selectorName(expr); ok && recv == "http" && strings.HasPrefix(name, "Method") {
		return strings.ToUpper(strings.TrimPrefix(name, "Method")), true
	}
	if s, ok := stringLiteral(expr); ok {
		return strings.ToUpper(s), true
	}
	return "", false
}

func resultType(expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	switch x := expr.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.StarExpr:
		return resultType(x.X)
	case *ast.SelectorExpr:
		return exprString(x)
	case *ast.ArrayType:
		return resultType(x.Elt)
	case *ast.MapType:
		return "map"
	case *ast.InterfaceType:
		return "any"
	default:
		return exprString(expr)
	}
}

func exprString(expr ast.Expr) string {
	switch x := expr.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.StarExpr:
		return resultType(x.X)
	case *ast.SelectorExpr:
		return exprString(x.X) + "." + x.Sel.Name
	case *ast.ArrayType:
		return resultType(x.Elt)
	case *ast.MapType:
		return "map"
	case *ast.InterfaceType:
		return "any"
	default:
		return ""
	}
}

func baseTypeName(typ string) string {
	typ = strings.TrimPrefix(typ, "*")
	if i := strings.LastIndex(typ, "."); i >= 0 {
		return typ[i+1:]
	}
	return typ
}

func displayHandler(route Route) string {
	if route.HandlerType != "" && strings.Contains(route.HandlerExpr, ".") {
		return route.HandlerType + "." + strings.Split(route.HandlerExpr, ".")[1]
	}
	return route.HandlerExpr
}

func normalizeFields(fields []string) []string {
	set := map[string]bool{}
	for _, f := range fields {
		f = strings.ToLower(strings.TrimSpace(f))
		if f != "" {
			set[f] = true
		}
	}
	out := make([]string, 0, len(set))
	for f := range set {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

func jsonTagName(tag string) string {
	for _, part := range strings.Split(tag, " ") {
		if strings.HasPrefix(part, `json:"`) {
			v := strings.TrimPrefix(part, `json:"`)
			v = strings.TrimSuffix(v, `"`)
			return strings.Split(v, ",")[0]
		}
	}
	return ""
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func copySeen(in map[string]bool) map[string]bool {
	out := map[string]bool{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func fileSHA(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func writeJSON(path string, report Report) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func writeMarkdown(path string, report Report) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# V1.2-C Contract Audit\n\n")
	fmt.Fprintf(&b, "- generated_at: %s\n- total_paths: %d\n- clean: %d\n- drift: %d\n- unmapped: %d\n- known_gap: %d\n\n", report.GeneratedAt, report.Summary.TotalPaths, report.Summary.Clean, report.Summary.Drift, report.Summary.Unmapped, report.Summary.KnownGap)
	fmt.Fprintf(&b, "| method | path | openapi_path | verdict | only_in_code | only_in_openapi | handler |\n|---|---|---|---|---:|---:|---|\n")
	for _, p := range report.Paths {
		fmt.Fprintf(&b, "| %s | `%s` | `%s` | %s | %d | %d | `%s` |\n", p.Method, p.Path, p.OpenAPIPath, p.Verdict, len(p.OnlyInCode), len(p.OnlyInOpenAPI), p.Handler)
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}
