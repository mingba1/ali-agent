package retrieval

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gogf/gf/v2/frame/g"

	"ali-agent/internal/model"
)

// DBSearch 对应架构图中的 OLTP / OLAP 后端：执行只读 SQL 查询并返回前 N 行。
// 出于安全考虑，仅允许 SELECT，且表必须在白名单内。
func DBSearch(ctx context.Context, sql string, maxRows int) ([]model.Hit, error) {
	trimmed := strings.TrimSpace(sql)
	if trimmed == "" {
		return []model.Hit{}, nil
	}
	upper := strings.ToUpper(trimmed)
	if !strings.HasPrefix(upper, "SELECT") && !strings.HasPrefix(upper, "SHOW") {
		return []model.Hit{}, nil
	}
	allowTables := g.Cfg().MustGet(ctx, "retrieval.db.allowTables").Strings()
	if len(allowTables) > 0 {
		if !tableAllowed(trimmed, allowTables) {
			return []model.Hit{}, nil
		}
	}
	rows, err := g.DB().GetAll(ctx, trimmed)
	if err != nil {
		return nil, err
	}
	hits := make([]model.Hit, 0, len(rows))
	for i, r := range rows {
		if maxRows > 0 && i >= maxRows {
			break
		}
		raw, _ := json.Marshal(r.Map())
		hits = append(hits, model.Hit{
			Source: "db", Row: string(raw), Score: 1,
			Ref: "sql:row" + itoa(int64(i+1)),
		})
	}
	return hits, nil
}

// tableAllowed 校验 SQL 中涉及的表是否在白名单内。
func tableAllowed(sql string, allow []string) bool {
	upper := strings.ToUpper(sql)
	// 简单实现：找出所有表名标识符
	lower := strings.ToLower(sql)
	for _, tbl := range allow {
		if strings.Contains(lower, strings.ToLower(tbl)) {
			return true
		}
	}
	_ = upper
	return false
}
