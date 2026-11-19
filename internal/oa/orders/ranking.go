package orders

import (
	"errors"
	"strings"
)

func defaultRankingMetric(group string) string {
	if group == "growth" {
		return "growth"
	}
	return "amount"
}

func rankingQuery(group, sort, order string, brush bool) (string, error) {
	if group == "" {
		group = "shop"
	}
	if sort == "" {
		sort = defaultRankingMetric(group)
	}
	direction := "DESC"
	switch order {
	case "", "desc":
	case "asc":
		direction = "ASC"
	default:
		return "", errors.New("排行排序方向无效")
	}
	var aggregate, fields, dimension, keys string
	sorts := map[string]string{"amount": "sales"}
	switch group {
	case "shop", "platform", "growth":
		if group == "growth" && brush {
			return "", errors.New("增长排行使用剔除刷单的固定口径")
		}
		keys = "shop_id"
		if group == "platform" {
			keys = "platform"
		}
		aggregate, fields = rankingShopMetrics(keys, group == "growth", brush)
		sorts = map[string]string{"amount": "payable", "count": "orders", "gross": "gross", "grossRate": "gross_rate"}
		if group == "growth" {
			sorts["growth"] = "growth_rate"
		}
	case "product":
		aggregate = rankingProductAggregate
		fields = `ranked.shop_item_id AS key,ranked.shop_id AS shop_id,ranked.shop_item_id AS shop_item_id,
  coalesce(nullIf(trimBoth(identity.listing_name),''),ranked.shop_item_id) AS title,toString(ranked.sales) AS item_pay_total`
		dimension = ` LEFT ANY JOIN jst.v_listings AS identity ON ranked.shop_id=identity.shop_id AND ranked.shop_item_id=identity.shop_item_id`
		keys = "shop_id,shop_item_id"
	case "person":
		aggregate = rankingPersonAggregate
		fields = `ranked.person_id AS key,ranked.person_id AS person_id,
  coalesce(nullIf(trimBoth(identity.name),''),ranked.person_id) AS name,
  if(empty(identity.person_id),NULL,identity.archived) AS archived,toString(ranked.sales) AS item_pay_total`
		dimension = ` LEFT ANY JOIN jst.v_reg_person AS identity ON ranked.person_id=identity.person_id`
		keys = "person_id"
	case "sku":
		aggregate = rankingSKUAggregate
		fields = `ranked.sku_id AS key,ranked.shop_id AS shop_id,ranked.sku_id AS sku,ranked.sku_id AS sku_id,
  ranked.product_name AS product_name,toString(ranked.sales) AS item_pay_total,toString(ranked.units) AS quantity,
  toString(ranked.verified_sales) AS item_pay_verified_total,toString(ranked.verified_cost) AS cost_verified_total,
  toString(ranked.gross) AS verified_gross,toFloat64(ranked.verified_sales)/nullIf(toFloat64(ranked.sales),0) AS verified_coverage`
		keys = "shop_id,sku_id"
		sorts["quantity"] = "units"
	default:
		return "", errors.New("排行类型无效")
	}
	metric, ok := sorts[sort]
	if !ok {
		return "", errors.New("排行排序字段无效")
	}
	// Dimension names cannot affect which groups make the page.
	sortBy := metric + " " + direction + " NULLS LAST," + keys
	outerSortBy := "ranked." + metric + " " + direction + " NULLS LAST,ranked." + strings.ReplaceAll(keys, ",", ",ranked.")
	return "SELECT " + fields + ",ranked.total_rows AS total_rows FROM (SELECT *,toUInt32(count() OVER ()) AS total_rows FROM (" +
		aggregate + ") AS grouped ORDER BY " + sortBy + " LIMIT 101 OFFSET {offset:UInt32}) AS ranked" + dimension + " ORDER BY " + outerSortBy, nil
}

const rankingProductAggregate = `SELECT shop_id,assumeNotNull(shop_item_id) AS shop_item_id,sum(item_pay_amount) AS sales` +
	realLines + rankedOrderIDs + ` AND is_placeholder_price=0 AND notEmpty(ifNull(shop_item_id,'')) GROUP BY shop_id,shop_item_id`
const rankingPersonAggregate = `SELECT assumeNotNull(person_id) AS person_id,sum(item_pay_amount) AS sales FROM (` +
	attributedLines + `) AS attribution INNER JOIN (SELECT sub_order_id,item_pay_amount` + realLines + rankedOrderIDs +
	`) AS eligible USING sub_order_id GROUP BY person_id`
const rankingSKUAggregate = `SELECT shop_id,assumeNotNull(sku_id) AS sku_id,
 argMax(product_name,tuple(order_time,sub_order_id)) AS product_name,sum(item_pay_amount) AS sales,sum(quantity) AS units,
 sumIf(item_pay_amount,` + verifiedLine + `) AS verified_sales,
 if(countIf(` + verifiedLine + `)=0,NULL,sumIf(cost_amount,` + verifiedLine + `)) AS verified_cost,
 ` + gross + ` AS gross` + realLines + rankedOrderIDs +
	` AND is_placeholder_price=0 AND notEmpty(ifNull(sku_id,'')) GROUP BY shop_id,sku_id`

// v_shop_day represents missing verified cost with zeros. Confirm true zero/zero
// days against the same valid non-brush orders used by the summary repair.
const rankingVerifiedZeroPairs = `SELECT shop_id,toDate(order_time,'Asia/Shanghai') AS verified_day FROM jst.v_orders WHERE ` +
	period + shopFilter + ` AND is_valid=1 AND is_brush=0 AND isNotNull(cost_verified_total) GROUP BY shop_id,verified_day`

func rankingShopMetrics(key string, growth, brush bool) (string, string) {
	from := shopDays
	orders := "sum(source.order_count)"
	verifiedDay := "(source.cost_verified_total!=0 OR source.item_pay_verified_total!=0 OR (source.shop_id,source.day) IN (" + rankingVerifiedZeroPairs + "))"
	costPresent := "countIf(" + verifiedDay + ")"
	payable := "sum(source.payable_amount)"
	sales := "sum(source.item_pay_total)"
	verified := "sum(source.item_pay_verified_total)"
	cost := "sum(source.cost_verified_total)"
	baseline := ""
	fields := `ranked.` + key + ` AS key,ranked.` + key + ` AS ` + key + `,
 toString(ranked.sales) AS item_pay_total,toString(ranked.payable) AS payable_amount,ranked.orders AS order_count,
 toString(ranked.verified_cost) AS cost_verified_total,toString(ranked.verified_sales) AS item_pay_verified_total,
 toString(ranked.gross) AS verified_gross,toFloat64(ranked.gross_rate) AS verified_gross_rate,
 toFloat64(ranked.verified_sales)/nullIf(toFloat64(ranked.sales),0) AS verified_coverage`
	switch {
	case growth:
		from = ` FROM jst.v_shop_day AS source WHERE day>=toDate({baseline:String}) AND day<toDate({until:String}) AND order_count>0 ` + shopFilter
		current := "source.day>=toDate({from:String})"
		payable = "sumIf(source.payable_amount," + current + ")"
		sales = "sumIf(source.item_pay_total," + current + ")"
		verified = "sumIf(source.item_pay_verified_total," + current + ")"
		orders = "sumIf(source.order_count," + current + ")"
		cost = "sumIf(source.cost_verified_total," + current + ")"
		costPresent = "countIf(" + verifiedDay + " AND " + current + ")"
		baseline = ",sumIf(source.payable_amount,source.day<toDate({from:String})) AS baseline"
		fields += `,toString(ranked.baseline) AS baseline_amount,
 if(ranked.baseline=0,'zero_baseline','comparable') AS growth_status,toFloat64(ranked.growth_rate) AS growth_rate`
	case brush:
		from = ` FROM jst.v_orders AS source WHERE ` + period + shopFilter + ` AND is_valid=1`
		orders = "count()"
		costPresent = "count(source.cost_verified_total)"
	}
	grouped := "SELECT source." + key + " AS " + key + "," + payable + " AS payable," + sales + " AS sales," +
		orders + " AS orders," + verified + " AS verified_sales,if(" + costPresent + "=0,NULL," + cost + ") AS verified_cost" +
		baseline + from + " GROUP BY source." + key
	// ClickHouse can evaluate both if branches. Give divideDecimal a safe
	// denominator even for rows whose displayed and sorted ratio is NULL.
	metrics := `SELECT *,verified_sales-verified_cost AS gross,
 if(verified_sales>0,divideDecimal(verified_sales-verified_cost,if(verified_sales>0,verified_sales,1),24),NULL) AS gross_rate`
	if growth {
		metrics += ",toFloat64(payable-baseline)/nullIf(toFloat64(baseline),0) AS growth_rate"
	}
	return metrics + " FROM (" + grouped + ") AS metrics", fields
}
