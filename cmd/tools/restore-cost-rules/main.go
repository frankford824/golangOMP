// Restores existing production tariffs and deterministic style bindings.
// Default mode is read-only; applying requires the reviewed plan digest and
// creates an exclusive recovery report before the single database transaction.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"workflow/domain"
	"workflow/repo"
	mysqlrepo "workflow/repo/mysql"
	"workflow/service"
)

type plan struct {
	Database      string                              `json:"database"`
	Rules         []*domain.CostRule                  `json:"original_rules"`
	Successors    []*domain.CostRule                  `json:"successor_rules"`
	Bindings      []service.LegacyCostBindingDecision `json:"binding_decisions"`
	CatalogDigest string                              `json:"catalog_digest,omitempty"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	apply := flag.Bool("apply", false, "apply reviewed configuration only; never edits SKU prices")
	expected := flag.String("expected-plan", "", "SHA256 returned by dry-run")
	confirm := flag.String("confirm-database", "", "required database name for apply")
	recovery := flag.String("recovery", "", "new recovery report path required for apply")
	catalog := flag.String("style-catalog", "", "optional JSON array of existing selectable style identities")
	flag.Parse()
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		c := mysql.NewConfig()
		c.User = os.Getenv("DB_USER")
		c.Passwd = os.Getenv("DB_PASS")
		c.Net = "tcp"
		port := os.Getenv("DB_PORT")
		if port == "" {
			port = "3306"
		}
		c.Addr = os.Getenv("DB_HOST") + ":" + port
		c.DBName = os.Getenv("DB_NAME")
		c.ParseTime = true
		dsn = c.FormatDSN()
	}
	c, err := mysql.ParseDSN(dsn)
	if err != nil {
		return err
	}
	c.ParseTime = true
	db, err := sql.Open("mysql", c.FormatDSN())
	if err != nil {
		return err
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	w := mysqlrepo.New(db)
	rr := mysqlrepo.NewCostRuleRepo(w)
	br := mysqlrepo.NewCostRuleBindingRepo(w)
	p := plan{}
	if err = db.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&p.Database); err != nil {
		return err
	}
	var all []*domain.CostRule
	for page := 1; ; page++ {
		part, total, e := rr.List(ctx, repo.CostRuleListFilter{Page: page, PageSize: 100})
		if e != nil {
			return e
		}
		all = append(all, part...)
		if int64(len(all)) >= total {
			break
		}
		if len(part) == 0 {
			return fmt.Errorf("incomplete rule pagination")
		}
	}
	groups := map[string]bool{}
	for _, r := range all {
		groups[r.CategoryCode] = true
	}
	keys := []string{}
	for g := range groups {
		keys = append(keys, g)
	}
	sort.Strings(keys)
	for _, g := range keys {
		rules, err := rr.ListActiveByCategory(ctx, nil, g, time.Now())
		if err != nil {
			return err
		}
		for _, r := range rules {
			p.Rules = append(p.Rules, r)
			if successor := service.RestoredLegacyCostRule(r); successor != nil {
				p.Successors = append(p.Successors, successor)
			}
		}
	}
	var bindings []*domain.CostRuleBinding
	for page := 1; ; page++ {
		part, total, e := br.List(ctx, repo.CostRuleBindingListFilter{Page: page, PageSize: 100})
		if e != nil {
			return e
		}
		bindings = append(bindings, part...)
		if int64(len(bindings)) >= total {
			break
		}
		if len(part) == 0 {
			return fmt.Errorf("incomplete binding pagination")
		}
	}
	// Only pre-regression snapshots can establish a historical mapping. Include
	// current style identities to expose unmatched newly introduced materials.
	rows, err := db.QueryContext(ctx, `SELECT i_id,rule_group FROM (
 SELECT DISTINCT product_i_id AS i_id,'' AS rule_group FROM task_sku_items WHERE product_i_id<>''
 UNION SELECT JSON_UNQUOTE(JSON_EXTRACT(s.input_snapshot_json,'$.product_i_id')),r.category_code
 FROM omp_sku_cost_snapshots s JOIN cost_rules r ON r.id=s.cost_rule_id
 WHERE s.event_source='task_create' AND s.created_at<'2026-09-18' AND JSON_VALID(s.input_snapshot_json)
 ) evidence ORDER BY i_id,rule_group`)
	if err != nil {
		return err
	}
	evidence := []service.LegacyCostBindingEvidence{}
	for rows.Next() {
		var iid, g sql.NullString
		if err = rows.Scan(&iid, &g); err != nil {
			rows.Close()
			return err
		}
		if iid.Valid && iid.String != "null" {
			evidence = append(evidence, service.LegacyCostBindingEvidence{IID: iid.String, Groups: []string{g.String}})
		}
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if *catalog != "" {
		raw, e := os.ReadFile(*catalog)
		if e != nil {
			return e
		}
		var ids []string
		if e = json.Unmarshal(raw, &ids); e != nil {
			return e
		}
		p.CatalogDigest = service.LegacyCostPlanDigest(ids)
		for _, id := range ids {
			evidence = append(evidence, service.LegacyCostBindingEvidence{IID: id, Catalog: true})
		}
	}
	p.Bindings = service.PlanLegacyCostBindings(p.Rules, bindings, evidence)
	digest := service.LegacyCostPlanDigest(p)
	if !*apply {
		b, _ := json.MarshalIndent(struct {
			SHA  string `json:"sha256"`
			Plan plan   `json:"plan"`
		}{digest, p}, "", "  ")
		fmt.Println(string(b))
		return nil
	}
	if *expected != digest || *confirm != p.Database || strings.TrimSpace(*recovery) == "" {
		return fmt.Errorf("apply requires matching --expected-plan, --confirm-database and a new --recovery file; current digest=%s", digest)
	}
	f, err := os.OpenFile(*recovery, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	if err = json.NewEncoder(f).Encode(p); err == nil {
		err = f.Sync()
	}
	f.Close()
	if err != nil {
		return err
	}
	createdRules, createdBindings := 0, 0
	err = w.RunInTx(ctx, func(tx repo.Tx) error {
		q := mysqlrepo.Unwrap(tx)
		// Lock and re-read all governed terms before applying the reviewed plan.
		for _, original := range p.Rules {
			var id int64
			if err := q.QueryRowContext(ctx, "SELECT id FROM cost_rules WHERE id=? FOR UPDATE", original.RuleID).Scan(&id); err != nil {
				return err
			}
			current, err := rr.GetByID(ctx, id)
			if err != nil {
				return err
			}
			if service.LegacyCostPlanDigest(current) != service.LegacyCostPlanDigest(original) {
				return fmt.Errorf("rule %d changed since preview", id)
			}
		}
		for _, r := range p.Successors {
			if _, err := rr.Create(ctx, tx, r); err != nil {
				return err
			}
			createdRules++
		}
		for _, d := range p.Bindings {
			if d.Group == "" || d.Reason == "existing_binding" {
				continue
			}
			b, err := br.GetActiveByNormalizedIID(ctx, d.NormalizedIID)
			if err != nil {
				return err
			}
			if b != nil {
				return fmt.Errorf("binding %s changed since preview", d.IID)
			}
			_, err = br.Create(ctx, tx, &domain.CostRuleBinding{IIDRaw: d.IID, NormalizedIID: d.NormalizedIID, RuleGroup: d.Group, DisplayName: d.IID, Source: service.LegacyCostRestoreSource, IsActive: true})
			if err != nil {
				return err
			}
			createdBindings++
		}
		return nil
	})
	if err != nil {
		return err
	}
	fmt.Printf("APPLIED database=%s sha256=%s successor_rules=%d bindings=%d recovery=%s\n", p.Database, digest, createdRules, createdBindings, *recovery)
	return nil
}
