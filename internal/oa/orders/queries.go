package orders

const period = ` order_time >= toDateTime(concat({from:String}, ' 00:00:00'), 'Asia/Shanghai') AND order_time < toDateTime(concat({until:String}, ' 00:00:00'), 'Asia/Shanghai') `
const shopFilter = ` AND ({all:UInt8}=1 OR shop_id IN {shops:Array(String)}) AND ({shop:String}='' OR shop_id={shop:String}) AND (({platform:String}='' AND platform NOT IN ('线下店铺','其它平台')) OR platform={platform:String}) `
const hourlySQL = `SELECT toString(toDate(source.order_time,'Asia/Shanghai')) AS day,toHour(source.order_time,'Asia/Shanghai') AS hour,toString(sum(source.payable_amount)) AS payable_amount FROM jst.v_orders AS source WHERE ` + period + shopFilter + ` AND is_valid=1 AND ({brush:UInt8}=1 OR is_brush=0) GROUP BY day,hour ORDER BY day,hour`
const shopDays = ` FROM jst.v_shop_day AS source WHERE day >= toDate({from:String}) AND day < toDate({until:String}) AND order_count>0 ` + shopFilter
const gross = `if(countIf(cost_tier='verified' AND cost_implausible=0 AND isNotNull(cost_amount))=0, NULL, CAST(sumIf(item_pay_amount-cost_amount,cost_tier='verified' AND cost_implausible=0 AND isNotNull(cost_amount)) AS Nullable(Decimal(38,2))))`
const verifiedLine = `cost_tier='verified' AND cost_implausible=0 AND isNotNull(cost_amount)`
const lineMetrics = `toString(sum(item_pay_amount)) AS item_pay_total, toString(sum(quantity)) AS quantity,
 toString(sumIf(item_pay_amount, ` + verifiedLine + `)) AS item_pay_verified_total,
 if(countIf(` + verifiedLine + `)=0,NULL,toString(sumIf(cost_amount,` + verifiedLine + `))) AS cost_verified_total,
 toString(` + gross + `) AS verified_gross,
 toFloat64(sumIf(item_pay_amount, ` + verifiedLine + `))/nullIf(toFloat64(sum(item_pay_amount)),0) AS verified_coverage`
const realLines = ` FROM jst.v_order_lines WHERE ` + period + shopFilter + ` AND order_is_valid=1 AND ({brush:UInt8}=1 OR (order_is_brush=0 AND is_brush=0)) AND is_real=1 `
const attributedLines = `SELECT DISTINCT person_id,sub_order_id FROM jst.v_line_attribution WHERE ` + period + ` AND ({all:UInt8}=1 OR shop_id IN {shops:Array(String)}) AND ({shop:String}='' OR shop_id={shop:String}) AND person_id IS NOT NULL AND person_id!='' `
const personScope = `WITH
 matched AS (
 SELECT DISTINCT person_id FROM jst.v_reg_person WHERE archived=0
 AND match(toString(employee_no),'^[0-9]+$') AND toUInt64OrNull(toString(employee_no))=toUInt64({employee:String})
 ),
 selected AS (SELECT person_id FROM matched WHERE (SELECT count() FROM matched)=1),
 latest AS (
 SELECT ledger_store_id,shop_item_id,max(segment_from) AS current_from FROM jst.v_reg_scheme_alloc
 WHERE segment_from<=toDateTime({at:String},'Asia/Shanghai') GROUP BY ledger_store_id,shop_item_id
 ),
 basis AS (
 SELECT ledger_store_id,person_id FROM jst.v_reg_org_store WHERE person_id IN (SELECT person_id FROM selected)
 UNION DISTINCT
 SELECT a.ledger_store_id,a.person_id FROM jst.v_reg_scheme_alloc AS a
 INNER JOIN latest AS l ON a.ledger_store_id=l.ledger_store_id AND a.shop_item_id=l.shop_item_id AND a.segment_from=l.current_from
 WHERE (isNull(a.segment_to) OR a.segment_to>toDateTime({at:String},'Asia/Shanghai')) AND a.mode='distribute'
 AND a.person_id IN (SELECT person_id FROM selected)
 )
 SELECT DISTINCT shops.shop_id AS shop_id,basis.person_id AS person_id FROM basis
 INNER JOIN jst.v_shops AS shops ON shops.ledger_store_id=basis.ledger_store_id AND shops.ledger_mapping_status='confirmed'
 UNION ALL SELECT '' AS shop_id,person_id FROM selected`
const summarySQL = `SELECT toString(day) AS day,sales.shop_id AS shop_id,sales.platform AS platform,order_count,
 toString(payable_amount) AS payable_amount,toString(sales.item_pay_total) AS item_pay_total,
 if(sales.cost_verified_total=0 AND sales.item_pay_verified_total=0,NULL,toString(sales.cost_verified_total)) AS cost_verified_total,
 toString(sales.item_pay_verified_total) AS item_pay_verified_total,
 if(sales.cost_verified_total=0 AND sales.item_pay_verified_total=0,NULL,toString(sales.item_pay_verified_total-sales.cost_verified_total)) AS verified_gross,
 toFloat64(sales.item_pay_verified_total)/nullIf(toFloat64(sales.item_pay_total),0) AS verified_coverage
 FROM (SELECT day,shop_id,platform,order_count,payable_amount,item_pay_total,cost_verified_total,item_pay_verified_total` + shopDays + `) AS sales ORDER BY day,shop_id`
const verifiedZeroSQL = `SELECT toString(toDate(order_time,'Asia/Shanghai')) AS day,shop_id FROM jst.v_orders WHERE ` + period + shopFilter + ` AND is_valid=1 AND is_brush=0 AND isNotNull(cost_verified_total) AND (shop_id,toString(toDate(order_time,'Asia/Shanghai'))) IN {zero_pairs:Array(Tuple(String,String))} GROUP BY day,shop_id`
const productSelect = `SELECT shop_id,shop_item_id,sku_id AS sku,argMax(product_name,order_time) AS product_name,toString(countIf(isNull(cost_amount))) AS missing_cost_lines,` + lineMetrics + realLines + ` AND ({sku:String}='' OR sku_id={sku:String}) AND ({product:String}='' OR shop_item_id={product:String})`
const productGroup = ` GROUP BY shop_id,shop_item_id,sku_id ORDER BY sum(item_pay_amount) DESC,shop_id,shop_item_id,sku_id LIMIT 101 OFFSET {offset:UInt32}`
const rankedOrderIDs = ` AND order_id IN (SELECT order_id FROM jst.v_orders WHERE ` + period + shopFilter + ` AND is_valid=1 AND ({brush:UInt8}=1 OR is_brush=0)) `
const productsSQL = productSelect + rankedOrderIDs + ` AND is_placeholder_price=0 ` + productGroup
const skuSQL = productSelect + rankedOrderIDs + productGroup
const myProductsSQL = productSelect + rankedOrderIDs + ` AND is_placeholder_price=0 ` + ` AND sub_order_id IN (SELECT sub_order_id FROM (` + attributedLines + ` AND person_id={person:String})) ` + productGroup
const personalSQL = `SELECT toString(toDate(order_time,'Asia/Shanghai')) AS day,toString(sum(item_pay_amount)) AS item_pay_total` + realLines + rankedOrderIDs + ` AND sub_order_id IN (SELECT sub_order_id FROM (` + attributedLines + ` AND person_id={person:String})) GROUP BY day ORDER BY day`
const shopNamesSQL = `SELECT shop_id,shop_name,platform FROM jst.v_shops WHERE ({all:UInt8}=1 OR shop_id IN {shops:Array(String)}) ORDER BY shop_name,shop_id`
const freshnessSQL = `SELECT source_committed_at,toUInt32(greatest(lag_seconds,0)) AS lag_seconds FROM jst.v_freshness LIMIT 1`
const historySQL = `SELECT sub_order_id,sku_id AS sku,toString(cost_amount) AS cost_amount,toString(unit_cost) AS unit_cost,cost_source,cost_tier,cost_status,cost_as_of,cost_frozen,cost_implausible,order_time,shop_id` + realLines + rankedOrderIDs + ` AND sku_id={sku:String} ORDER BY order_time DESC,sub_order_id DESC LIMIT 101 OFFSET {offset:UInt32}`
const refundsSQL = `SELECT toString(toDate(created_at,'Asia/Shanghai')) AS day,shop_id,toString(sum(source.refund_amount)) AS refund_amount,toString(sumIf(source.refund_amount,order_is_valid=1 AND is_outlier=0)) AS valid_refund_amount,toString(sumIf(source.refund_amount,order_found=0)) AS unmatched_refund_amount,toString(sumIf(source.refund_amount,is_outlier=1)) AS outlier_refund_amount FROM jst.v_after_sales AS source WHERE created_at>=toDateTime(concat({from:String},' 00:00:00'),'Asia/Shanghai') AND created_at<toDateTime(concat({until:String},' 00:00:00'),'Asia/Shanghai') AND refund_status='已确认'` + shopFilter + ` GROUP BY day,shop_id ORDER BY day,shop_id`
const orderDetailBase = `SELECT order_id,online_order_no,shop_id,platform,order_time,status,is_valid,is_origin,is_brush,toString(payable_amount) AS payable_amount,toString(collected_amount) AS collected_amount FROM jst.v_orders WHERE ` + period + shopFilter + ` AND ({brush:UInt8}=1 OR is_brush=0) `
const orderDetailPage = ` ORDER BY order_time DESC,order_id DESC LIMIT 101 OFFSET {offset:UInt32}`
const orderDetailSQL = orderDetailBase + orderDetailPage
const exactOrderDetailSQL = orderDetailBase + ` AND online_order_no={order_no:String}` + orderDetailPage
const ledgerSQL = `SELECT shops.shop_id AS shop_id, periods.period AS period, periods.state AS state, periods.computed_at AS computed_at, lines.line_id AS line_id, lines.name AS name, if(lines.available=1,toString(lines.value),NULL) AS value FROM jst.v_ledger_period AS periods INNER JOIN jst.v_ledger_statement AS lines ON periods.ledger_store_id=lines.ledger_store_id AND periods.period=lines.period AND periods.run_id=lines.run_id INNER JOIN (SELECT shop_id,ledger_store_id FROM jst.v_shops WHERE ledger_mapping_status='confirmed'` + shopFilter + `) AS shops ON shops.ledger_store_id=periods.ledger_store_id WHERE periods.period >= substring({from:String},1,7) AND periods.period <= substring(toString(toDate({until:String})-1),1,7) ORDER BY periods.period DESC,shops.shop_id,lines.ord,lines.line_id LIMIT 101 OFFSET {offset:UInt32}`
const compareSQL = `SELECT if(toDate(order_time,'Asia/Shanghai')<toDate({adjusted:String}),'before','after') AS phase, count() AS line_count,` + lineMetrics + realLines + rankedOrderIDs + ` AND sku_id={sku:String} GROUP BY phase ORDER BY phase DESC`

const brushSummarySQL = `SELECT toString(toDate(source.order_time,'Asia/Shanghai')) AS day,source.shop_id AS shop_id,source.platform AS platform,count() AS order_count,
 toString(sum(source.payable_amount)) AS payable_amount,toString(sum(source.item_pay_total)) AS item_pay_total,
 if(count(source.cost_verified_total)=0,NULL,toString(sum(source.cost_verified_total))) AS cost_verified_total,
 toString(sum(source.item_pay_verified_total)) AS item_pay_verified_total,
 if(count(source.cost_verified_total)=0,NULL,toString(sum(source.item_pay_verified_total)-sum(source.cost_verified_total))) AS verified_gross,
 toFloat64(sum(source.item_pay_verified_total))/nullIf(toFloat64(sum(source.item_pay_total)),0) AS verified_coverage
 FROM jst.v_orders AS source WHERE ` + period + shopFilter + ` AND is_valid=1 GROUP BY day,source.shop_id,source.platform ORDER BY day,shop_id`
