/*
   zorm is a better orm library for Go.

  Copyright (c) 2019 <http://ez8.co> <orca.zhang@yahoo.com>

  This library is released under the MIT License.
  Please see LICENSE file or visit https://github.com/IceWhaleTech/zorm for details.
*/

package zorm

import (
	"container/list"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math"
	"path"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
	"unsafe"

	"github.com/modern-go/reflect2"
)

const (
	_fields = 1 << iota
	_join
	_indexedBy
	_where
	_groupBy
	_having
	_orderBy
	_limit
	_onConflictDoUpdateSet

	_cond = iota
	_andCondEx
	_orCondEx
)

const (
	_timeLayout           = "2006-01-02 15:04:05"
	_timeLayoutWithZ      = "2006-01-02 15:04:05Z"
	_timeLayoutWithTZ     = "2006-01-02 15:04:05 -07:00"
	_timeLayoutWithNanoTZ = "2006-01-02 15:04:05.999999999 -07:00"
)

var config struct {
	Mock bool
}

// V - an alias object value type
type V map[string]interface{}

// U - an alias string type for update to support `x=x+1`
type U string

// Config .
type Config struct {
	Debug               bool
	Reuse               bool // 默认开启 SQL 和元数据复用
	UseNameWhenTagEmpty bool
	ToTimestamp         bool
}

const defaultStmtCacheSize = 128

// Table .
func Table(db ZormDBIFace, name string, ctx ...context.Context) *ZormTable {
	if len(ctx) > 0 {
		return &ZormTable{
			DB:   wrapDefaultStmtCacheDB(db),
			Name: name,
			ctx:  ctx[0],
			Cfg:  Config{Reuse: true}, // 默认开启Reuse（内建形状感知）
		}
	}
	return &ZormTable{
		DB:   wrapDefaultStmtCacheDB(db),
		Name: name,
		ctx:  context.Background(),
		Cfg:  Config{Reuse: true}, // 默认开启Reuse（内建形状感知）
	}
}

// TableContext 创建带Context的Table，参数顺序：context, db, name
func TableContext(ctx context.Context, db ZormDBIFace, name string) *ZormTable {
	return &ZormTable{
		DB:   wrapDefaultStmtCacheDB(db),
		Name: name,
		ctx:  ctx,
		Cfg:  Config{Reuse: true}, // 默认开启Reuse（内建形状感知）
	}
}

// Reuse .
func (t *ZormTable) Reuse() *ZormTable {
	t.Cfg.Reuse = true
	return t
}

// NoReuse 关闭Reuse功能（如果不需要缓存优化）
func (t *ZormTable) NoReuse() *ZormTable {
	t.Cfg.Reuse = false
	return t
}

// Debug .
func (t *ZormTable) Debug() *ZormTable {
	t.Cfg.Debug = true
	return t
}

// Audit enables SQL auditing for this table
func (t *ZormTable) Audit(auditLogger interface{}, telemetryCollector interface{}) *ZormTable {
	var logger AuditLogger
	if typedLogger, ok := auditLogger.(AuditLogger); ok {
		logger = typedLogger
	}

	var collector TelemetryCollector
	if typedCollector, ok := telemetryCollector.(TelemetryCollector); ok {
		collector = typedCollector
	}

	if auditable, ok := t.DB.(*AuditableDB); ok {
		if logger != nil {
			auditable.auditLogger = logger
		}
		if collector != nil {
			auditable.telemetryCollector = collector
		}
		auditable.Enable()
		return t
	}

	t.DB = NewAuditableDB(t.DB, logger, collector)
	return t
}

// UseNameWhenTagEmpty .
func (t *ZormTable) UseNameWhenTagEmpty() *ZormTable {
	t.Cfg.UseNameWhenTagEmpty = true
	return t
}

// ToTimestamp .
func (t *ZormTable) ToTimestamp() *ZormTable {
	t.Cfg.ToTimestamp = true
	return t
}

// SafeReuse 已合并进 Reuse，保持兼容
func (t *ZormTable) SafeReuse() *ZormTable { return t.Reuse() }

// NoSafeReuse 已合并进 Reuse，保持兼容
func (t *ZormTable) NoSafeReuse() *ZormTable { return t }

const shapeKeyCacheMaxParts = 8

type shapeKeyCacheKey struct {
	Base  string
	Op    string
	N     uint8
	Parts [shapeKeyCacheMaxParts]string
}

// buildShapeKey 基于调用点key和参数形状构建复用key
func buildShapeKey(baseKey string, op string, args []ZormItem) string {
	if len(args) <= shapeKeyCacheMaxParts {
		total := len(baseKey) + len(op) + 1
		key := shapeKeyCacheKey{Base: baseKey, Op: op, N: uint8(len(args))}
		for i, a := range args {
			shaped, ok := a.(interface{ ShapeKeyPart() string })
			if !ok {
				goto fallback
			}
			part := shaped.ShapeKeyPart()
			key.Parts[i] = part
			total += 1 + len(part)
		}
		if cached, ok := _shapeKeyStringCache.Load(key); ok {
			return cached.(string)
		}
		buf := getShapeKeyBuffer(total)
		buf = append(buf, baseKey...)
		buf = append(buf, '|')
		buf = append(buf, op...)
		for i := 0; i < len(args); i++ {
			buf = append(buf, '|')
			buf = append(buf, key.Parts[i]...)
		}
		shapeKey := string(buf)
		putShapeKeyBuffer(buf)
		if actual, loaded := _shapeKeyStringCache.LoadOrStore(key, shapeKey); loaded {
			return actual.(string)
		}
		return shapeKey
	}

fallback:
	total := len(baseKey) + len(op) + 1
	var fastPartsBuf [shapeKeyCacheMaxParts]string
	fastParts := fastPartsBuf[:0]
	if len(args) > len(fastPartsBuf) {
		fastParts = make([]string, 0, len(args))
	}
	for _, a := range args {
		shaped, ok := a.(interface{ ShapeKeyPart() string })
		if !ok {
			fastParts = nil
			break
		}
		part := shaped.ShapeKeyPart()
		fastParts = append(fastParts, part)
		total += 1 + len(part)
	}
	if fastParts != nil {
		buf := getShapeKeyBuffer(total)
		buf = append(buf, baseKey...)
		buf = append(buf, '|')
		buf = append(buf, op...)
		for _, part := range fastParts {
			buf = append(buf, '|')
			buf = append(buf, part...)
		}
		shapeKey := string(buf)
		putShapeKeyBuffer(buf)
		return shapeKey
	}

	b := getSQLBuilder()
	defer putSQLBuilder(b)
	b.WriteString(baseKey)
	b.WriteByte('|')
	b.WriteString(op)
	for _, a := range args {
		b.WriteByte('|')
		if shaped, ok := a.(interface{ ShapeKeyPart() string }); ok {
			b.WriteString(shaped.ShapeKeyPart())
			continue
		}
		b.WriteString(strconv.Itoa(a.Type()))
		a.BuildSQL(b)
	}
	return b.String()
}

func buildItemShapePart(itemType int, sql string) string {
	if sql == "" {
		return strconv.Itoa(itemType)
	}
	return strconv.Itoa(itemType) + sql
}

type mapUpdateFieldSpec struct {
	Name   string
	Inline bool
}

func collectMapUpdateFieldSpecs(m V, args []ZormItem) []mapUpdateFieldSpec {
	if len(args) > 0 && args[0].Type() == _fields {
		fields := args[0].(*fieldsItem).Fields
		specs := make([]mapUpdateFieldSpec, 0, len(fields))
		for _, field := range fields {
			v, ok := m[field]
			if !ok || v == nil {
				continue
			}
			_, inline := v.(U)
			specs = append(specs, mapUpdateFieldSpec{Name: field, Inline: inline})
		}
		return specs
	}

	if len(m) == 0 {
		return nil
	}
	fields := make([]string, 0, len(m))
	for field := range m {
		fields = append(fields, field)
	}
	sort.Strings(fields)

	specs := make([]mapUpdateFieldSpec, 0, len(fields))
	for _, field := range fields {
		_, inline := m[field].(U)
		specs = append(specs, mapUpdateFieldSpec{Name: field, Inline: inline})
	}
	return specs
}

func appendMapUpdateShapeKey(base string, specs []mapUpdateFieldSpec) string {
	if len(specs) == 0 {
		return base
	}
	total := len(base)
	for _, spec := range specs {
		total += 3 + len(spec.Name)
	}
	buf := getShapeKeyBuffer(total)
	buf = append(buf, base...)
	for _, spec := range specs {
		buf = append(buf, '|')
		buf = append(buf, spec.Name...)
		buf = append(buf, '=')
		if spec.Inline {
			buf = append(buf, 'u')
		} else {
			buf = append(buf, 'p')
		}
	}
	shapeKey := string(buf)
	putShapeKeyBuffer(buf)
	return shapeKey
}

func getShapeKeyBuffer(size int) []byte {
	buf := _shapeKeyBufferPool.Get().([]byte)
	if cap(buf) < size {
		return make([]byte, 0, size)
	}
	return buf[:0]
}

func putShapeKeyBuffer(buf []byte) {
	if cap(buf) > 4096 {
		return
	}
	_shapeKeyBufferPool.Put(buf[:0])
}

func buildFieldsSQL(fields []string) string {
	var sb strings.Builder
	for i, field := range fields {
		if i > 0 {
			sb.WriteString(",")
		}
		if field == "*" {
			sb.WriteString("*")
		} else {
			fieldEscape(&sb, field)
		}
	}
	return sb.String()
}

func writeCondListSQL(sb *strings.Builder, conds []interface{}, prefix string) {
	if len(conds) <= 0 {
		return
	}
	sb.WriteString(prefix)
	for i, c := range conds {
		if i > 0 {
			sb.WriteString(" and ")
		}
		if cond, ok := c.(*ormCond); ok {
			cond.BuildSQL(sb)
		} else if expr, ok := c.(*ormExpr); ok {
			expr.BuildSQL(sb)
		} else if condEx, ok := c.(*ormCondEx); ok {
			if condEx.Ty > _andCondEx && len(condEx.Conds) > 1 && len(conds) > 1 {
				sb.WriteString("(")
			}
			condEx.BuildSQL(sb)
			if condEx.Ty > _andCondEx && len(condEx.Conds) > 1 && len(conds) > 1 {
				sb.WriteString(")")
			}
		}
	}
}

func buildWhereSQL(conds []interface{}) string {
	var sb strings.Builder
	writeCondListSQL(&sb, conds, " where ")
	return sb.String()
}

func buildHavingSQL(conds []interface{}) string {
	var sb strings.Builder
	writeCondListSQL(&sb, conds, " having ")
	return sb.String()
}

func buildOrderBySQL(orders []string) string {
	var sb strings.Builder
	sb.WriteString(" order by ")
	for i, order := range orders {
		if i > 0 {
			sb.WriteString(",")
		}
		fieldEscape(&sb, order)
	}
	return sb.String()
}

func buildGroupBySQL(fields []string) string {
	var sb strings.Builder
	sb.WriteString(" group by ")
	for i, field := range fields {
		if i > 0 {
			sb.WriteString(",")
		}
		fieldEscape(&sb, field)
	}
	return sb.String()
}

func buildLimitSQL(args []interface{}) string {
	var sb strings.Builder
	sb.WriteString(" limit ?")
	if len(args) > 1 {
		sb.WriteString(" offset ?")
	}
	return sb.String()
}

func buildCondSQL(field string, op string) string {
	var sb strings.Builder
	if field != "" {
		fieldEscape(&sb, field)
	}
	sb.WriteString(op)
	return sb.String()
}

func buildCondExSQL(ty int, conds []interface{}) string {
	cx := &ormCondEx{Ty: ty, Conds: conds}
	var sb strings.Builder
	cx.BuildSQL(&sb)
	return sb.String()
}

// Fields .
func Fields(fields ...string) *fieldsItem {
	sql := buildFieldsSQL(fields)
	return &fieldsItem{Fields: fields, SQL: sql, Shape: buildItemShapePart(_fields, sql)}
}

// Join .
func Join(stmt string) *joinItem {
	return &joinItem{Stmt: stmt}
}

// LeftJoin 左连接
func LeftJoin(table string, on ...interface{}) *joinItem {
	return &joinItem{
		JoinType: "LEFT JOIN",
		Table:    table,
		On:       on,
	}
}

// RightJoin 右连接
func RightJoin(table string, on ...interface{}) *joinItem {
	return &joinItem{
		JoinType: "RIGHT JOIN",
		Table:    table,
		On:       on,
	}
}

// InnerJoin 内连接
func InnerJoin(table string, on ...interface{}) *joinItem {
	return &joinItem{
		JoinType: "INNER JOIN",
		Table:    table,
		On:       on,
	}
}

// FullJoin 全连接
func FullJoin(table string, on ...interface{}) *joinItem {
	return &joinItem{
		JoinType: "FULL OUTER JOIN",
		Table:    table,
		On:       on,
	}
}

// Where .
func Where(conds ...interface{}) *whereItem {
	w, err := TryWhere(conds...)
	if err != nil {
		panic(err.Error())
	}
	return w
}

// TryWhere returns an error instead of panicking on invalid input.
func TryWhere(conds ...interface{}) (*whereItem, error) {
	if l := len(conds); l > 0 {
		if s, ok := conds[0].(string); ok {
			sql := " where " + s
			return &whereItem{Conds: []interface{}{
				&ormCond{
					Op:   s,
					Args: conds[1:l],
					SQL:  s,
				},
			}, SQL: sql, Shape: buildItemShapePart(_where, sql)}, nil
		}
		w := &whereItem{}
		for _, c := range conds {
			if condEx, ok := c.(*ormCondEx); ok {
				if len(condEx.Conds) <= 0 {
					continue
				}
			}
			w.Conds = append(w.Conds, c)
		}
		w.SQL = buildWhereSQL(w.Conds)
		w.Shape = buildItemShapePart(_where, w.SQL)
		return w, nil
	}
	return nil, errors.New("too few conditions")
}

// GroupBy .
func GroupBy(fields ...string) *groupByItem {
	sql := buildGroupBySQL(fields)
	return &groupByItem{Fields: fields, SQL: sql, Shape: buildItemShapePart(_groupBy, sql)}
}

// Having .
func Having(conds ...interface{}) *havingItem {
	h, err := TryHaving(conds...)
	if err != nil {
		panic(err.Error())
	}
	return h
}

// TryHaving returns an error instead of panicking on invalid input.
func TryHaving(conds ...interface{}) (*havingItem, error) {
	if l := len(conds); l > 0 {
		if s, ok := conds[0].(string); ok {
			sql := " having " + s
			return &havingItem{Conds: []interface{}{
				&ormCond{
					Op:   s,
					Args: conds[1:l],
					SQL:  s,
				},
			}, SQL: sql, Shape: buildItemShapePart(_having, sql)}, nil
		}
		h := &havingItem{}
		for _, c := range conds {
			if condEx, ok := c.(*ormCondEx); ok {
				if len(condEx.Conds) <= 0 {
					continue
				}
			}
			h.Conds = append(h.Conds, c)
		}
		h.SQL = buildHavingSQL(h.Conds)
		h.Shape = buildItemShapePart(_having, h.SQL)
		return h, nil
	}
	return nil, errors.New("too few conditions")
}

// OrderBy .
func OrderBy(orders ...string) *orderByItem {
	sql := buildOrderBySQL(orders)
	return &orderByItem{Orders: orders, SQL: sql, Shape: buildItemShapePart(_orderBy, sql)}
}

// Limit .
func Limit(i ...interface{}) *limitItem {
	l, err := TryLimit(i...)
	if err != nil {
		panic(err.Error())
	}
	return l
}

// TryLimit returns an error instead of panicking on invalid input.
func TryLimit(i ...interface{}) (*limitItem, error) {
	switch len(i) {
	case 1, 2:
		sql := buildLimitSQL(i)
		return &limitItem{I: i, SQL: sql, Shape: buildItemShapePart(_limit, sql)}, nil
	}
	return nil, errors.New("too few or too many limit params")
}

// OnConflictDoUpdateSet 支持 SQLite 的 INSERT ... ON CONFLICT DO UPDATE 语法
// 使用 excluded. 前缀来引用冲突行的值
// 这是 SQLite 的官方 UPSERT 语法，功能上等价于 MySQL 的 ON DUPLICATE KEY UPDATE
// 示例：
//
//	INSERT INTO users (id, name, age)
//	VALUES (1, 'Alice', 18)
//	ON CONFLICT(id) DO UPDATE SET
//	    name = excluded.name,
//	    age  = excluded.age;
func OnConflictDoUpdateSet(conflictFields []string, updateFields []string) *onConflictDoUpdateSetItem {
	res := &onConflictDoUpdateSetItem{}
	if len(conflictFields) <= 0 || len(updateFields) <= 0 {
		return res
	}

	var sb strings.Builder
	sb.WriteString(" on conflict(")
	for i, field := range conflictFields {
		if i > 0 {
			sb.WriteString(",")
		}
		fieldEscape(&sb, field)
	}
	sb.WriteString(") do update set")
	for i, field := range updateFields {
		if i > 0 {
			sb.WriteString(",")
		}
		fieldEscape(&sb, field)
		sb.WriteString("=excluded.")
		fieldEscape(&sb, field)
	}
	res.Conds = sb.String()
	return res
}

// IndexedBy .
func IndexedBy(idx string) *indexedByItem {
	return &indexedByItem{idx: idx}
}

// Select .
func (t *ZormTable) Select(res interface{}, args ...ZormItem) (int, error) {
	// Support unconditional queries (no args required)

	var (
		rt         = reflect2.TypeOf(res)
		isArray    bool
		isPtrArray bool
		rtElem     = rt
		isPtrPtr   bool // 标记是否是指针的指针（如 **User）

		item     *DataBindingItem
		stmtArgs []interface{}
		shapeKey string
		cache    *dataBindingCache
	)
	origArgs := args

	// 使用池化的参数切片
	stmtArgs = getArgsSlice()
	defer func() { putArgsSlice(stmtArgs) }()

	switch rt.Kind() {
	case reflect.Ptr:
		// 检查是否是指针的指针
		rtTemp := rt.(reflect2.PtrType).Elem()
		if rtTemp.Kind() == reflect.Ptr {
			isPtrPtr = true
		}
		// 解引用所有指针层，直到找到非指针类型
		for rt.Kind() == reflect.Ptr {
			rt = rt.(reflect2.PtrType).Elem()
		}
		rtElem = rt
		if rt.Kind() == reflect.Slice {
			rtElem = rt.(reflect2.ListType).Elem()
			isArray = true

			if rtElem.Kind() == reflect.Ptr {
				rtElem = rtElem.(reflect2.PtrType).Elem()
				isPtrArray = true
			}
		} else if rt.Kind() == reflect.Map {
			// 支持 *map[string]interface{} 或 **map[string]interface{} 类型
			rtElem = rt
		}
	case reflect.Map:
		// 直接支持map类型
		rtElem = rt
	default:
		return 0, errors.New("argument 2 should be map or ptr")
	}

	if config.Mock {
		pc, fileName, _, _ := runtime.Caller(1)
		if ok, data, n, e := checkMock(t.Name, "Select", runtime.FuncForPC(pc).Name(), fileName, path.Dir(fileName)); ok {
			rt.UnsafeSet(reflect2.PtrOf(res), reflect2.PtrOf(data))
			return n, e
		}
	}

	if t.Cfg.Reuse {
		callSite := getCallSite()
		shapeKey = buildShapeKey(callSite.Key, "Select", origArgs)
		cache = t.getDataBindingCache(callSite, reflect.TypeOf(res), "", 0)
		if i, ok := cache.Load(shapeKey); ok {
			item = i.(*DataBindingItem)
		}
	}

	queryArgs := origArgs
	if item != nil {
		switch rtElem.Kind() {
		case reflect.Struct, reflect.Map:
			if len(queryArgs) > 0 && queryArgs[0].Type() == _fields {
				queryArgs = queryArgs[1:]
			}
		default:
			if len(queryArgs) > 0 {
				queryArgs = queryArgs[1:]
			}
		}

		for _, arg := range queryArgs {
			arg.BuildArgs(&stmtArgs)
		}
	} else {
		item = &DataBindingItem{Type: rtElem}

		sb := getSQLBuilder()
		sb.WriteString("select ")

		switch rtElem.Kind() {
		case reflect.Struct:
			s := rtElem.(reflect2.StructType)
			fieldSet := t.getStructFieldSet(s)
			selected := false

			if len(queryArgs) > 0 && queryArgs[0].Type() == _fields {
				fieldsList := queryArgs[0].(*fieldsItem).Fields
				queryArgs = queryArgs[1:]
				if len(fieldsList) > 0 {
					selected = true
					wildcardUsed := false
					for _, field := range fieldsList {
						if field == "*" {
							wildcardUsed = true
							break
						}
					}

					if wildcardUsed {
						sb.WriteString(fieldSet.DefaultSQL)
						item.SCols = make([]reflect2.StructField, 0, len(fieldSet.Ordered))
						for _, meta := range fieldSet.Ordered {
							item.SCols = append(item.SCols, meta.Field)
						}
					} else {
						queryArgs = append([]ZormItem{Fields(fieldsList...)}, queryArgs...)
						queryArgs[0].BuildSQL(sb)
						queryArgs = queryArgs[1:]
						item.SCols = make([]reflect2.StructField, 0, len(fieldsList))
						for _, field := range fieldsList {
							if f := fieldSet.FieldMap[field]; f != nil {
								item.SCols = append(item.SCols, f)
							}
						}
					}
				}
			}

			if !selected {
				if len(fieldSet.Ordered) == 0 {
					putSQLBuilder(sb)
					return 0, errors.New("no fields to select: struct has no fields with zorm tags or UseNameWhenTagEmpty is false")
				}
				sb.WriteString(fieldSet.DefaultSQL)
				item.SCols = make([]reflect2.StructField, 0, len(fieldSet.Ordered))
				for _, meta := range fieldSet.Ordered {
					item.SCols = append(item.SCols, meta.Field)
				}
			}
		case reflect.Map:
			// map类型：如果指定了Fields，使用指定的字段；否则使用 SELECT *
			if len(queryArgs) > 0 && queryArgs[0].Type() == _fields {
				fi := queryArgs[0].(*fieldsItem)
				item.Fields = append([]string(nil), fi.Fields...)
				for i, field := range item.Fields {
					if i > 0 {
						sb.WriteString(",")
					}
					fieldEscape(sb, field)
				}
				item.Cols = make([]interface{}, len(item.Fields))
				queryArgs = queryArgs[1:]
			} else {
				sb.WriteString("*")
			}
		default:
			// 必须有fields且为1
			if len(queryArgs) == 0 || queryArgs[0].Type() != _fields {
				putSQLBuilder(sb)
				return 0, errors.New("argument 3 need ONE Fields(\"name\") with ONE field")
			}

			fi := queryArgs[0].(*fieldsItem)
			if len(fi.Fields) < 1 {
				putSQLBuilder(sb)
				return 0, errors.New("too few fields")
			}

			fieldEscape(sb, fi.Fields[0])
			queryArgs = queryArgs[1:]
		}

		sb.WriteString(" from ")
		fieldEscape(sb, t.Name)

		// 处理 args，自动将 ormCond 和 ormCondEx 包装为 whereItem
		for _, arg := range queryArgs {
			if condEx, ok := arg.(*ormCondEx); ok {
				where := &whereItem{Conds: []interface{}{condEx}}
				where.BuildSQL(sb)
				where.BuildArgs(&stmtArgs)
			} else if cond, ok := arg.(*ormCond); ok {
				where := &whereItem{Conds: []interface{}{cond}}
				where.BuildSQL(sb)
				where.BuildArgs(&stmtArgs)
			} else {
				arg.BuildSQL(sb)
				arg.BuildArgs(&stmtArgs)
			}
		}

		item.SQL = sb.String()
		putSQLBuilder(sb)

		shouldCache := t.Cfg.Reuse && shapeKey != "" && !(rtElem.Kind() == reflect.Map && len(item.Fields) == 0)
		if shouldCache {
			cache.Store(shapeKey, item)
		}
	}

	if t.Cfg.Debug {
		log.Println(item.SQL, stmtArgs)
	}

	var (
		selectElem interface{}
		scanCols   []interface{}
	)
	if rtElem.Kind() != reflect.Map {
		if isArray || isPtrPtr {
			selectElem = rtElem.New()
		} else {
			selectElem = res
		}
		binding := bindSelectScanCols(item, selectElem)
		scanCols = binding.Cols
		defer putSelectScanBinding(binding)
	}

	if !isArray {
		// fire
		if rtElem.Kind() == reflect.Map {
			stringAnyMap := isStringAnyMapType(rtElem)
			// Map类型需要特殊处理
			// 如果没有指定Fields（使用SELECT *），第一次查询后缓存列名
			if len(item.Fields) == 0 {
				rows, err := t.DB.QueryContext(t.ctx, item.SQL, stmtArgs...)
				if err != nil {
					return 0, err
				}
				defer rows.Close()

				columns, err := rows.Columns()
				if err != nil {
					return 0, err
				}
				item.Fields = append([]string(nil), columns...)
				if t.Cfg.Reuse && shapeKey != "" {
					cache.Store(shapeKey, item)
				}

				binding := getMapScanBinding(len(item.Fields))
				defer putMapScanBinding(binding)

				if rows.Next() {
					err = rows.Scan(binding.Ptrs...)
					if err != nil {
						return 0, err
					}

					if stringAnyMap {
						mapVal := buildStringAnyMap(item.Fields, binding.Values)
						if dest, ok := res.(*map[string]interface{}); ok {
							*dest = mapVal
						} else {
							reflect.ValueOf(res).Elem().Set(reflect.ValueOf(mapVal))
						}
						return 1, nil
					}
					mapVal := reflect.MakeMap(rtElem.(reflect2.MapType).Type1())
					fillReflectMap(mapVal, item.Fields, binding.Values)
					reflect.ValueOf(res).Elem().Set(mapVal)
					return 1, nil
				}
				if err := rows.Err(); err != nil {
					return 0, err
				}
				return 0, nil
			}

			binding := getMapScanBinding(len(item.Fields))
			defer putMapScanBinding(binding)

			err := t.DB.QueryRowContext(t.ctx, item.SQL, stmtArgs...).Scan(binding.Ptrs...)
			if err != nil {
				if err == sql.ErrNoRows {
					return 0, nil
				}
				return 0, err
			}

			if stringAnyMap {
				mapVal := buildStringAnyMap(item.Fields, binding.Values)
				if dest, ok := res.(*map[string]interface{}); ok {
					*dest = mapVal
				} else {
					reflect.ValueOf(res).Elem().Set(reflect.ValueOf(mapVal))
				}
				return 1, nil
			}
			mapVal := reflect.MakeMap(rtElem.(reflect2.MapType).Type1())
			fillReflectMap(mapVal, item.Fields, binding.Values)
			reflect.ValueOf(res).Elem().Set(mapVal)
			return 1, nil
		}

		err := t.DB.QueryRowContext(t.ctx, item.SQL, stmtArgs...).Scan(scanCols...)
		if err != nil {
			if err == sql.ErrNoRows {
				return 0, nil
			}
			return 0, err
		}

		if isPtrPtr && rtElem.Kind() == reflect.Struct {
			rvRes := reflect.ValueOf(res)
			if rvRes.Kind() == reflect.Ptr && !rvRes.IsNil() {
				newPtr := reflect.New(rtElem.Type1())
				elemVal := reflect.ValueOf(selectElem)
				if elemVal.Kind() == reflect.Ptr {
					newPtr.Elem().Set(elemVal.Elem())
				} else {
					newPtr.Elem().Set(elemVal)
				}
				rvRes.Elem().Set(newPtr)
			}
		}

		return 1, nil
	}

	rows, err := t.DB.QueryContext(t.ctx, item.SQL, stmtArgs...)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var mapBinding *mapScanBinding
	stringAnyMap := isStringAnyMapType(rtElem)
	if rtElem.Kind() == reflect.Map {
		if len(item.Fields) == 0 {
			columns, err := rows.Columns()
			if err != nil {
				return 0, err
			}
			item.Fields = append([]string(nil), columns...)
			if t.Cfg.Reuse && shapeKey != "" {
				cache.Store(shapeKey, item)
			}
		}
		mapBinding = getMapScanBinding(len(item.Fields))
		defer putMapScanBinding(mapBinding)
	}

	count := 0
	for rows.Next() {
		if rtElem.Kind() == reflect.Map {
			err = rows.Scan(mapBinding.Ptrs...)
			if err != nil {
				break
			}

			if stringAnyMap {
				mapVal := buildStringAnyMap(item.Fields, mapBinding.Values)
				if isPtrArray {
					rt.(reflect2.SliceType).UnsafeAppend(reflect2.PtrOf(res), unsafe.Pointer(&mapVal))
				} else if dest, ok := res.(*[]map[string]interface{}); ok {
					*dest = append(*dest, mapVal)
				} else {
					reflect.ValueOf(res).Elem().Set(reflect.Append(reflect.ValueOf(res).Elem(), reflect.ValueOf(mapVal)))
				}
			} else {
				mapVal := reflect.MakeMap(rtElem.(reflect2.MapType).Type1())
				fillReflectMap(mapVal, item.Fields, mapBinding.Values)
				if isPtrArray {
					rt.(reflect2.SliceType).UnsafeAppend(reflect2.PtrOf(res), unsafe.Pointer(&mapVal))
				} else {
					reflect.ValueOf(res).Elem().Set(reflect.Append(reflect.ValueOf(res).Elem(), mapVal))
				}
			}
		} else {
			err = rows.Scan(scanCols...)
			if err != nil {
				break
			}

			if isPtrArray {
				copyElem := rtElem.UnsafeNew()
				rtElem.UnsafeSet(copyElem, reflect2.PtrOf(selectElem))
				rt.(reflect2.SliceType).UnsafeAppend(reflect2.PtrOf(res), unsafe.Pointer(&copyElem))
			} else {
				rt.(reflect2.SliceType).UnsafeAppend(reflect2.PtrOf(res), reflect2.PtrOf(selectElem))
			}
		}
		count++
	}
	if err == nil {
		err = rows.Err()
	}
	return count, err
}

// InsertIgnore .
func (t *ZormTable) InsertIgnore(objs interface{}, args ...ZormItem) (int, error) {
	if config.Mock {
		pc, fileName, _, _ := runtime.Caller(1)
		if ok, _, n, e := checkMock(t.Name, "InsertIgnore", runtime.FuncForPC(pc).Name(), fileName, path.Dir(fileName)); ok {
			return n, e
		}
	}

	return t.insert("insert or ignore into ", objs, args)
}

// ReplaceInto .
func (t *ZormTable) ReplaceInto(objs interface{}, args ...ZormItem) (int, error) {
	if config.Mock {
		pc, fileName, _, _ := runtime.Caller(1)
		if ok, _, n, e := checkMock(t.Name, "ReplaceInto", runtime.FuncForPC(pc).Name(), fileName, path.Dir(fileName)); ok {
			return n, e
		}
	}

	return t.insert("replace into ", objs, args)
}

// Insert .
func (t *ZormTable) Insert(objs interface{}, args ...ZormItem) (int, error) {
	if config.Mock {
		pc, fileName, _, _ := runtime.Caller(1)
		if ok, _, n, e := checkMock(t.Name, "Insert", runtime.FuncForPC(pc).Name(), fileName, path.Dir(fileName)); ok {
			return n, e
		}
	}

	return t.insert("insert into ", objs, args)
}

func (t *ZormTable) insert(prefix string, objs interface{}, args []ZormItem) (int, error) {
	// 检查 nil 指针
	if objs == nil {
		return 0, errors.New("cannot insert nil pointer")
	}

	// 使用反射检查是否为 nil 指针
	rv := reflect.ValueOf(objs)
	if rv.Kind() == reflect.Ptr && rv.IsNil() {
		return 0, errors.New("cannot insert nil pointer")
	}

	var (
		rt         = reflect2.TypeOf(objs)
		rtOrig     = rt          // 保存原始类型，用于判断 objs 是否已经是指针
		rtSlice    reflect2.Type // 保存切片类型，用于后续操作
		isArray    bool
		isPtrArray bool
		rtPtr      reflect2.Type
		rtElem     = rt

		stmtArgs []interface{}
		cols     []reflect2.StructField

		item     *DataBindingItem
		cache    *dataBindingCache
		shapeKey string
	)
	origArgs := args
	stmtArgs = getArgsSlice()
	defer func() { putArgsSlice(stmtArgs) }()

	cacheRt := rt
	cacheRtElem := cacheRt
	cacheRtSlice := rtSlice
	cacheRtPtr := rtPtr
	cacheIsArray := isArray
	cacheIsPtrArray := isPtrArray
	switch cacheRt.Kind() {
	case reflect.Ptr:
		cacheRt = cacheRt.(reflect2.PtrType).Elem()
		cacheRtElem = cacheRt
		if cacheRt.Kind() == reflect.Slice {
			cacheRtSlice = cacheRt
			cacheRtElem = cacheRt.(reflect2.ListType).Elem()
			cacheIsArray = true
			if cacheRtElem.Kind() == reflect.Ptr {
				cacheRtPtr = cacheRtElem
				cacheRtElem = cacheRtElem.(reflect2.PtrType).Elem()
				cacheIsPtrArray = true
			}
		}
	case reflect.Struct:
		cacheRtElem = cacheRt
	case reflect.Slice:
		cacheRtSlice = cacheRt
		cacheRtElem = cacheRt.(reflect2.ListType).Elem()
		cacheIsArray = true
		if cacheRtElem.Kind() == reflect.Ptr {
			cacheRtPtr = cacheRtElem
			cacheRtElem = cacheRtElem.(reflect2.PtrType).Elem()
			cacheIsPtrArray = true
		}
	}

	if t.Cfg.Reuse {
		callSite := getCallSite()
		shapeKey = buildShapeKey(callSite.Key, "Insert", origArgs)
		rows := 0
		if cacheIsArray {
			rows = cacheRtSlice.(reflect2.SliceType).UnsafeLengthOf(reflect2.PtrOf(objs))
		}
		cache = t.getDataBindingCache(callSite, reflect.TypeOf(objs), prefix, rows)
		if i, ok := cache.Load(shapeKey); ok {
			item = i.(*DataBindingItem)
		}
	}

	if item != nil && cacheRtElem.Kind() == reflect.Struct {
		cols = item.SCols
		if len(cols) == 0 {
			item = nil
		} else {
			rtElem = cacheRtElem
			rtSlice = cacheRtSlice
			rtPtr = cacheRtPtr
			isArray = cacheIsArray
			isPtrArray = cacheIsPtrArray

			// 使用缓存的SQL和字段信息
			// 构建参数
			if isArray {
				slice := rtSlice.(reflect2.SliceType)
				length := slice.UnsafeLengthOf(reflect2.PtrOf(objs))
				for i := 0; i < length; i++ {
					ptr := slice.UnsafeGetIndex(reflect2.PtrOf(objs), i)
					t.inputArgs(&stmtArgs, cols, item.ArgIndex, rtPtr, rtElem, isPtrArray, ptr)
				}
			} else {
				t.inputArgs(&stmtArgs, cols, item.ArgIndex, nil, rtElem, false, reflect2.PtrOf(objs))
			}

			// 处理额外的args
			for _, arg := range origArgs {
				arg.BuildArgs(&stmtArgs)
			}
		}
	} else {
		// 构建SQL和字段信息
		item = &DataBindingItem{Type: rtElem}

		sb := getSQLBuilder()
		sb.WriteString(prefix)
		fieldEscape(sb, t.Name)
		sb.WriteString(" (")

		switch rt.Kind() {
		case reflect.Ptr:
			rt = rt.(reflect2.PtrType).Elem()
			rtElem = rt
			if rt.Kind() == reflect.Slice {
				rtSlice = rt // 保存切片类型
				rtElem = rtElem.(reflect2.ListType).Elem()
				isArray = true

				if rtElem.Kind() == reflect.Ptr {
					rtPtr = rtElem
					rtElem = rtElem.(reflect2.PtrType).Elem()
					isPtrArray = true
				}
			}
		case reflect.Struct:
			// 支持非指针结构体
			rtElem = rt
			// 创建临时指针用于操作
			objs = reflect2.PtrOf(objs)
			rt = reflect2.TypeOf(objs)
		case reflect.Slice:
			// 支持非指针切片
			rtSlice = rt // 保存切片类型
			rtElem = rt.(reflect2.ListType).Elem()
			isArray = true

			// 检查是否是slice of maps ([]V)
			if rtElem.Kind() == reflect.Map {
				// 处理slice of maps
				mapType := rtElem.(reflect2.MapType)
				keyType := mapType.Key()

				// 只支持string key的map
				if keyType.Kind() != reflect.String {
					return 0, errors.New("map key must be string type")
				}

				sliceType := rt.(reflect2.SliceType)
				sliceLen := sliceType.UnsafeLengthOf(reflect2.PtrOf(objs))
				if sliceLen == 0 {
					return 0, errors.New("empty slice")
				}

				// 获取第一个map来确定字段
				firstMapPtr := sliceType.UnsafeGetIndex(reflect2.PtrOf(objs), 0)
				// UnsafeGetIndex 返回的是 map 的指针，需要转换为实际的 map 值
				firstMapVal := mapType.UnsafeIndirect(firstMapPtr)
				firstMapReflectVal := reflect.ValueOf(firstMapVal)

				// 检查是否有Fields参数
				var fields []string
				if len(args) > 0 && args[0].Type() == _fields {
					fields = args[0].(*fieldsItem).Fields
				} else {
					// 从第一个map中提取所有字段
					mapIter := firstMapReflectVal.MapRange()
					for mapIter.Next() {
						fields = append(fields, mapIter.Key().String())
					}
					// 排序字段以确保顺序一致
					sort.Strings(fields)
				}

				if len(fields) == 0 {
					return 0, errors.New("no fields found in map")
				}

				// 构建字段列表
				for i, field := range fields {
					if i > 0 {
						sb.WriteString(",")
					}
					fieldEscape(sb, field)
				}
				sb.WriteString(") values ")

				// 构建VALUES部分
				valuePlaceholder := "("
				for i := range fields {
					if i > 0 {
						valuePlaceholder += ","
					}
					valuePlaceholder += "?"
				}
				valuePlaceholder += ")"

				// 为每个map构建values
				for i := 0; i < sliceLen; i++ {
					if i > 0 {
						sb.WriteString(",")
					}
					sb.WriteString(valuePlaceholder)

					// 获取当前map
					mapPtr := sliceType.UnsafeGetIndex(reflect2.PtrOf(objs), i)
					// UnsafeGetIndex 返回的是 map 的指针，需要转换为实际的 map 值
					mapVal := mapType.UnsafeIndirect(mapPtr)
					mapReflectVal := reflect.ValueOf(mapVal)

					// 构建参数
					for _, field := range fields {
						fieldVal := mapReflectVal.MapIndex(reflect.ValueOf(field))
						if fieldVal.IsValid() {
							stmtArgs = append(stmtArgs, fieldVal.Interface())
						} else {
							stmtArgs = append(stmtArgs, nil)
						}
					}
				}

				// 处理额外的args
				argStart := 0
				if len(args) > 0 && args[0].Type() == _fields {
					argStart = 1
				}
				for i := argStart; i < len(args); i++ {
					args[i].BuildSQL(sb)
					args[i].BuildArgs(&stmtArgs)
				}

				// 执行SQL
				result, err := t.DB.ExecContext(t.ctx, sb.String(), stmtArgs...)
				if err != nil {
					return 0, err
				}

				affected, err := result.RowsAffected()
				if err != nil {
					return 0, err
				}

				putSQLBuilder(sb) // 释放字符串构建器
				return int(affected), nil
			}

			// 处理slice of structs
			if rtElem.Kind() == reflect.Ptr {
				rtPtr = rtElem
				rtElem = rtElem.(reflect2.PtrType).Elem()
				isPtrArray = true
			}
			// 创建临时指针用于操作
			objs = reflect2.PtrOf(objs)
			rt = reflect2.TypeOf(objs)
		case reflect.Map:
			// 处理map类型
			mapType := rt.(reflect2.MapType)
			keyType := mapType.Key()

			// 只支持string key的map
			if keyType.Kind() != reflect.String {
				return 0, errors.New("map key must be string type")
			}

			// 检查是否有Fields参数
			if len(args) > 0 && args[0].Type() == _fields {
				// 使用Fields参数指定的字段
				fields := args[0].(*fieldsItem).Fields
				fieldMap := make(map[string]interface{})

				// 从map中提取指定字段
				mapVal := reflect.ValueOf(objs)
				for _, field := range fields {
					if mapVal.MapIndex(reflect.ValueOf(field)).IsValid() {
						fieldMap[field] = mapVal.MapIndex(reflect.ValueOf(field)).Interface()
					}
				}

				// 构建字段列表
				for i, field := range fields {
					if i > 0 {
						sb.WriteString(",")
					}
					fieldEscape(sb, field)
				}

				sb.WriteString(") values (")
				for i := range fields {
					if i > 0 {
						sb.WriteString(",")
					}
					sb.WriteString("?")
				}
				sb.WriteString(")")

				// 构建参数
				for _, field := range fields {
					if val, exists := fieldMap[field]; exists {
						stmtArgs = append(stmtArgs, val)
					} else {
						stmtArgs = append(stmtArgs, nil)
					}
				}

				// 处理额外的args
				for i := 1; i < len(args); i++ {
					args[i].BuildSQL(sb)
					args[i].BuildArgs(&stmtArgs)
				}
			} else {
				// 使用所有字段
				// 使用通用字段收集函数处理map
				var fieldInfos []FieldInfo
				if err := t.collectFieldsGeneric(objs, rt, sb, &fieldInfos); err != nil {
					return 0, err
				}

				// 构建VALUES部分
				sb.WriteString(") values (")
				for i := range fieldInfos {
					if i > 0 {
						sb.WriteString(",")
					}
					sb.WriteString("?")
				}
				sb.WriteString(")")

				// 构建参数
				for _, fieldInfo := range fieldInfos {
					stmtArgs = append(stmtArgs, fieldInfo.GetValue(nil))
				}

				// 处理额外的args
				for _, arg := range args {
					arg.BuildSQL(sb)
					arg.BuildArgs(&stmtArgs)
				}
			}

			// 执行SQL
			result, err := t.DB.ExecContext(t.ctx, sb.String(), stmtArgs...)
			if err != nil {
				return 0, err
			}

			affected, err := result.RowsAffected()
			if err != nil {
				return 0, err
			}

			return int(affected), nil
		default:
			return 0, errors.New("argument 2 should be map or ptr")
		}

		// Fields or None
		// struct类型
		if rtElem.Kind() != reflect.Struct {
			return 0, errors.New("non-structure type not supported yet")
		}

		s := rtElem.(reflect2.StructType)
		if len(args) > 0 && args[0].Type() == _fields {
			m := t.getStructFieldMap(s)

			for _, field := range args[0].(*fieldsItem).Fields {
				f := m[field]
				if f != nil {
					cols = append(cols, f)
				}
			}

			(args[0]).BuildSQL(sb)
			args = args[1:]

		} else {
			t.collectFieldsForInsert(s, sb, &cols)
		}

		sb.WriteString(") values ")

		sbTmp := sb
		if isArray {
			sbTmp = getSQLBuilder()
		}

		sbTmp.WriteString("(")
		for j := range cols {
			if j > 0 {
				sbTmp.WriteString(",")
			}
			sbTmp.WriteString("?")
		}
		sbTmp.WriteString(")")

		item.ArgIndex = buildStructArgIndexes(s, cols)
		// inputArgs objs
		if isArray {
			// 数组
			for i := 0; i < rtSlice.(reflect2.SliceType).UnsafeLengthOf(reflect2.PtrOf(objs)); i++ {
				if i > 0 {
					sb.WriteString(",")
				}
				sb.WriteString(sbTmp.String())
				t.inputArgs(&stmtArgs, cols, item.ArgIndex, rtPtr, s, isPtrArray, rtSlice.(reflect2.ListType).UnsafeGetIndex(reflect2.PtrOf(objs), i))
			}
			putSQLBuilder(sbTmp) // 释放临时构建器
		} else {
			// 普通元素
			t.inputArgs(&stmtArgs, cols, item.ArgIndex, rtPtr, s, false, reflect2.PtrOf(objs))
		}

		// on duplicate key update
		for _, arg := range args {
			arg.BuildSQL(sb)
			arg.BuildArgs(&stmtArgs)
		}

		item.SQL = sb.String()
		putSQLBuilder(sb) // 释放字符串构建器
		item.Cols = make([]interface{}, len(cols))
		item.SCols = make([]reflect2.StructField, len(cols))
		for i, f := range cols {
			item.Cols[i] = f
			item.SCols[i] = f
		}

		// 存储到缓存
		if t.Cfg.Reuse {
			cache.Store(shapeKey, item)
		}
	}

	if t.Cfg.Debug {
		log.Println(item.SQL, stmtArgs)
	}

	res, err := t.DB.ExecContext(t.ctx, item.SQL, stmtArgs...)
	if err != nil {
		return 0, err
	}

	if !isArray && rtElem.Kind() == reflect.Struct {
		s := rtElem.(reflect2.StructType)
		// 首先尝试使用新的自增主键字段
		if autoField := t.getAutoIncrementField(s); autoField != nil {
			id, _ := res.LastInsertId()
			// 尝试使用 reflect2 设置，如果失败则使用 reflect 包
			func() {
				defer func() {
					if r := recover(); r != nil {
						// 使用 reflect 包设置字段值（支持嵌入结构体）
						rv := reflect.ValueOf(objs).Elem()
						fieldVal := rv.FieldByName(autoField.Name())
						if !fieldVal.IsValid() {
							// 如果直接字段名找不到，尝试在嵌入结构体中查找
							for i := 0; i < rv.NumField(); i++ {
								field := rv.Type().Field(i)
								if field.Anonymous {
									embeddedVal := rv.Field(i)
									if embeddedVal.Kind() == reflect.Struct {
										fieldVal = embeddedVal.FieldByName(autoField.Name())
										if fieldVal.IsValid() {
											break
										}
									}
								}
							}
						}
						if fieldVal.IsValid() && fieldVal.CanSet() {
							fieldVal.SetInt(id)
						}
					}
				}()
				autoField.UnsafeSet(reflect2.PtrOf(objs), reflect2.PtrOf(id))
			}()
		} else if f := s.FieldByName("ZormLastId"); f != nil {
			// 向后兼容：支持旧的 ZormLastId 字段
			id, _ := res.LastInsertId()
			f.UnsafeSet(reflect2.PtrOf(objs), reflect2.PtrOf(id))
		}
	} else if isArray && rtElem.Kind() == reflect.Struct {
		// 处理批量插入的自增ID设置
		s := rtElem.(reflect2.StructType)
		if autoField := t.getAutoIncrementField(s); autoField != nil {
			slice := rtSlice.(reflect2.SliceType)
			// 获取 slice 的指针
			// 使用与构建参数时相同的方式：reflect2.PtrOf(objs)
			// 注意：在 line 1052 和 1057，构建参数时使用的是 reflect2.PtrOf(objs)
			// 为了保持一致，这里也使用相同的方式
			// reflect2.PtrOf 对于指针类型会返回指针本身的值（即指针指向的地址）
			// 对于 *[]User，reflect2.PtrOf(objs) 返回的是 []User 的地址，这正是我们需要的
			slicePtr := reflect2.PtrOf(objs)
			length := slice.UnsafeLengthOf(slicePtr)
			lastInsertId, _ := res.LastInsertId()
			// 为每个对象设置自增ID（假设是连续的自增ID）
			for i := 0; i < length; i++ {
				ptr := slice.UnsafeGetIndex(slicePtr, i)
				if isPtrArray {
					// 指针数组：对于 []*User，ptr 是指向元素的地址（即 &slice[i]）
					// slice[i] 的类型是 *User，所以 ptr 是 **User
					// 我们需要先解引用获取 *User 的值
					elemPtr := *(*unsafe.Pointer)(ptr)
					// 如果 slice 中的元素是 nil，elemPtr 就是 nil
					if elemPtr != nil {
						autoField.UnsafeSet(elemPtr, reflect2.PtrOf(lastInsertId-int64(length-i-1)))
					}
				} else {
					// 值数组：需要获取指针
					autoField.UnsafeSet(reflect2.PtrOf(ptr), reflect2.PtrOf(lastInsertId-int64(length-i-1)))
				}
			}
		} else if f := s.FieldByName("ZormLastId"); f != nil {
			// 向后兼容：支持旧的 ZormLastId 字段
			slice := rtSlice.(reflect2.SliceType)
			// 获取 slice 的指针
			var slicePtr unsafe.Pointer
			if rtOrig.Kind() == reflect.Ptr {
				// objs 是 *[]User，objs 本身的值就是指向 []User 的指针
				rv := reflect.ValueOf(objs)
				if rv.Kind() == reflect.Ptr {
					slicePtr = unsafe.Pointer(rv.Elem().UnsafeAddr())
				} else {
					slicePtr = reflect2.PtrOf(objs)
				}
			} else {
				slicePtr = reflect2.PtrOf(objs)
			}
			length := slice.UnsafeLengthOf(slicePtr)
			lastInsertId, _ := res.LastInsertId()
			for i := 0; i < length; i++ {
				ptr := slice.UnsafeGetIndex(slicePtr, i)
				if isPtrArray {
					// 指针数组：对于 []*User，ptr 是指向元素的地址（即 &slice[i]）
					// slice[i] 的类型是 *User，所以 ptr 是 **User
					// 我们需要先解引用获取 *User 的值
					elemPtr := *(*unsafe.Pointer)(ptr)
					// 如果 slice 中的元素是 nil，elemPtr 就是 nil
					if elemPtr != nil {
						f.UnsafeSet(elemPtr, reflect2.PtrOf(lastInsertId-int64(length-i-1)))
					}
				} else {
					f.UnsafeSet(reflect2.PtrOf(ptr), reflect2.PtrOf(lastInsertId-int64(length-i-1)))
				}
			}
		}
	}

	row, _ := res.RowsAffected()
	return int(row), nil
}

// Update .
func (t *ZormTable) Update(obj interface{}, args ...ZormItem) (int, error) {
	if config.Mock {
		pc, fileName, _, _ := runtime.Caller(1)
		if ok, _, n, e := checkMock(t.Name, "Update", runtime.FuncForPC(pc).Name(), fileName, path.Dir(fileName)); ok {
			return n, e
		}
	}

	if len(args) <= 0 {
		return 0, errors.New("argument 2 cannot be omitted")
	}

	var (
		stmtArgs       []interface{}
		item           *DataBindingItem
		shapeKey       string
		cache          *dataBindingCache
		mapUpdateSpecs []mapUpdateFieldSpec
	)
	origArgs := args
	if m, ok := obj.(V); ok {
		mapUpdateSpecs = collectMapUpdateFieldSpecs(m, origArgs)
	}

	// 使用池化的参数切片
	stmtArgs = getArgsSlice()
	defer func() { putArgsSlice(stmtArgs) }()

	// Reuse缓存检查
	if t.Cfg.Reuse {
		callSite := getCallSite()
		shapeKey = buildShapeKey(callSite.Key, "Update", origArgs)
		if len(mapUpdateSpecs) > 0 {
			shapeKey = appendMapUpdateShapeKey(shapeKey, mapUpdateSpecs)
		}
		cache = t.getDataBindingCache(callSite, reflect.TypeOf(obj), "", 0)
		if i, ok := cache.Load(shapeKey); ok {
			item = i.(*DataBindingItem)
		}
	}

	if item != nil {
		// 使用缓存的SQL
		// 构建参数
		if m, ok := obj.(V); ok {
			// Map类型
			for _, field := range item.Fields {
				v, ok := m[field]
				if !ok {
					stmtArgs = append(stmtArgs, nil)
					continue
				}
				if _, ok := v.(U); ok {
					continue
				}
				stmtArgs = append(stmtArgs, v)
			}
		} else {
			// Struct类型
			rt := reflect2.TypeOf(obj)
			var objPtr interface{}

			if rt.Kind() == reflect.Ptr {
				rt = rt.(reflect2.PtrType).Elem()
				objPtr = obj
			} else if rt.Kind() == reflect.Struct {
				// 支持非指针结构体
				objPtr = reflect2.PtrOf(obj)
				rt = reflect2.TypeOf(objPtr).(reflect2.PtrType).Elem()
			} else {
				return 0, errors.New("update requires struct or pointer to struct")
			}
			if rt.Kind() == reflect.Struct {
				s := rt.(reflect2.StructType)
				t.inputArgs(&stmtArgs, item.SCols, item.ArgIndex, nil, s, false, reflect2.PtrOf(objPtr))
			} else {
				return 0, errors.New("non-structure type not supported yet")
			}
		}

		// 处理Where条件
		for _, arg := range args {
			arg.BuildArgs(&stmtArgs)
		}
	} else {
		// 构建SQL和字段信息
		item = &DataBindingItem{Type: reflect2.TypeOf(obj)}

		sb := getSQLBuilder()
		sb.WriteString("update ")
		fieldEscape(sb, t.Name)
		sb.WriteString(" set ")

		// 处理SET部分
		if m, ok := obj.(V); ok {
			// Map类型处理
			for i, spec := range mapUpdateSpecs {
				if i > 0 {
					sb.WriteString(",")
				}
				fieldEscape(sb, spec.Name)
				v := m[spec.Name]
				if inline, ok := v.(U); ok {
					sb.WriteString("=")
					sb.WriteString(string(inline))
				} else {
					sb.WriteString("=?")
					stmtArgs = append(stmtArgs, v)
				}
				item.Fields = append(item.Fields, spec.Name)
			}
			if len(args) > 0 && args[0].Type() == _fields {
				args = args[1:]
			}
		} else {
			// Struct类型处理
			rt := reflect2.TypeOf(obj)
			var objPtr interface{}

			if rt.Kind() == reflect.Ptr {
				rt = rt.(reflect2.PtrType).Elem()
				objPtr = obj
			} else if rt.Kind() == reflect.Struct {
				// 支持非指针结构体
				objPtr = reflect2.PtrOf(obj)
				rt = reflect2.TypeOf(objPtr).(reflect2.PtrType).Elem()
			} else {
				return 0, errors.New("update requires struct or pointer to struct")
			}
			if rt.Kind() == reflect.Struct {
				s := rt.(reflect2.StructType)
				// 如果传入了 Fields(...)，仅更新这些字段，并消费该参数
				if len(args) > 0 && args[0].Type() == _fields {
					m := t.getStructFieldMap(s)
					fields := args[0].(*fieldsItem).Fields
					for i, name := range fields {
						f := m[name]
						if f == nil {
							putSQLBuilder(sb)
							return 0, errors.New("field not found: " + name)
						}
						if i > 0 {
							sb.WriteString(",")
						}
						fieldEscape(sb, name)
						sb.WriteString("=?")
						item.Cols = append(item.Cols, f)
						item.SCols = append(item.SCols, f)
					}
					item.Fields = fields
					item.ArgIndex = buildStructArgIndexes(s, item.SCols)
					t.inputArgs(&stmtArgs, item.SCols, item.ArgIndex, nil, s, false, reflect2.PtrOf(objPtr))
					args = args[1:]
				} else {
					// 如果没有 Fields，更新所有字段（除了被忽略的）
					argCnt := 0
					for i := 0; i < s.NumField(); i++ {
						f := s.Field(i)
						ft := f.Tag().Get("zorm")
						if !t.Cfg.UseNameWhenTagEmpty && ft == "" {
							continue
						}
						if ft == "-" {
							continue
						}
						if isAutoIncrementField(f) {
							continue
						}
						// 忽略 ZormLastId 字段（向后兼容字段，不是数据库列）
						if f.Name() == "ZormLastId" {
							continue
						}
						if argCnt > 0 {
							sb.WriteString(",")
						}
						// 使用getFieldName获取数据库字段名（自动转换驼峰为蛇形）
						dbFieldName := getFieldName(f)
						if dbFieldName != "" {
							fieldEscape(sb, dbFieldName)
						}
						sb.WriteString("=?")
						val := f.Get(s.PackEFace(reflect2.PtrOf(objPtr)))
						if f.Type().String() == "time.Time" {
							if t.Cfg.ToTimestamp {
								val = val.(*time.Time).UTC().Unix()
							} else {
								val = val.(*time.Time).UTC().Format(_timeLayout)
							}
						}
						stmtArgs = append(stmtArgs, val)
						item.Cols = append(item.Cols, f)
						item.SCols = append(item.SCols, f)
						argCnt++
					}
					item.Fields = make([]string, argCnt)
					idx := 0
					for i := 0; i < s.NumField(); i++ {
						f := s.Field(i)
						ft := f.Tag().Get("zorm")
						if !t.Cfg.UseNameWhenTagEmpty && ft == "" {
							continue
						}
						if ft == "-" {
							continue
						}
						if isAutoIncrementField(f) {
							continue
						}
						// 忽略 ZormLastId 字段
						if f.Name() == "ZormLastId" {
							continue
						}
						if ft == "" {
							item.Fields[idx] = f.Name()
						} else {
							item.Fields[idx] = ft
						}
						idx++
					}
					if t.Cfg.Reuse {
						item.ArgIndex = buildStructArgIndexes(s, item.SCols)
					}
				}
			} else {
				return 0, errors.New("non-structure type not supported yet")
			}
		}

		// 合并多个 Where 为一个 WHERE 子句
		var whereItems []*whereItem
		var otherArgs []ZormItem
		for _, arg := range args {
			if w, ok := arg.(*whereItem); ok {
				whereItems = append(whereItems, w)
			} else {
				otherArgs = append(otherArgs, arg)
			}
		}

		// 如果有多个 Where，合并它们
		if len(whereItems) > 0 {
			mergedWhere := &whereItem{}
			for _, w := range whereItems {
				mergedWhere.Conds = append(mergedWhere.Conds, w.Conds...)
			}
			mergedWhere.BuildSQL(sb)
			mergedWhere.BuildArgs(&stmtArgs)
		}

		// 处理其他参数
		for _, arg := range otherArgs {
			arg.BuildSQL(sb)
			arg.BuildArgs(&stmtArgs)
		}

		item.SQL = sb.String()
		putSQLBuilder(sb) // 释放字符串构建器

		// 存储到缓存
		if t.Cfg.Reuse && shapeKey != "" {
			cache.Store(shapeKey, item)
		}
	}

	if t.Cfg.Debug {
		log.Println(item.SQL, stmtArgs)
	}

	res, err := t.DB.ExecContext(t.ctx, item.SQL, stmtArgs...)
	if err != nil {
		return 0, err
	}

	row, _ := res.RowsAffected()
	return int(row), nil
}

// Delete .
func (t *ZormTable) Delete(args ...ZormItem) (int, error) {
	if len(args) <= 0 {
		return 0, errors.New("argument 1 cannot be omitted")
	}

	if config.Mock {
		pc, fileName, _, _ := runtime.Caller(1)
		if ok, _, n, e := checkMock(t.Name, "Delete", runtime.FuncForPC(pc).Name(), fileName, path.Dir(fileName)); ok {
			return n, e
		}
	}

	var (
		stmtArgs []interface{}
		item     *DataBindingItem
		cache    *dataBindingCache
		shapeKey string
	)

	// 使用池化的参数切片
	stmtArgs = getArgsSlice()
	defer func() { putArgsSlice(stmtArgs) }()

	// Reuse缓存检查
	if t.Cfg.Reuse {
		callSite := getCallSite()
		shapeKey = buildShapeKey(callSite.Key, "Delete", args)
		cache = t.getDataBindingCache(callSite, nil, "", 0)
		if i, ok := cache.Load(shapeKey); ok {
			item = i.(*DataBindingItem)
		}
	}

	if item != nil {
		// 使用缓存的SQL
		// 构建参数
		for _, arg := range args {
			arg.BuildArgs(&stmtArgs)
		}
	} else {
		// 构建SQL
		item = &DataBindingItem{Type: nil}

		sb := getSQLBuilder()
		sb.WriteString("delete from ")
		fieldEscape(sb, t.Name)

		// 合并多个 Where 为一个 WHERE 子句
		var whereItems []*whereItem
		var otherArgs []ZormItem
		for _, arg := range args {
			if w, ok := arg.(*whereItem); ok {
				whereItems = append(whereItems, w)
			} else {
				otherArgs = append(otherArgs, arg)
			}
		}

		// 如果有多个 Where，合并它们
		if len(whereItems) > 0 {
			mergedWhere := &whereItem{}
			for _, w := range whereItems {
				mergedWhere.Conds = append(mergedWhere.Conds, w.Conds...)
			}
			mergedWhere.BuildSQL(sb)
			mergedWhere.BuildArgs(&stmtArgs)
		}

		// 处理其他参数
		for _, arg := range otherArgs {
			arg.BuildSQL(sb)
			arg.BuildArgs(&stmtArgs)
		}

		item.SQL = sb.String()
		putSQLBuilder(sb) // 释放字符串构建器

		// 存储到缓存
		if t.Cfg.Reuse {
			cache.Store(shapeKey, item)
		}
	}

	if t.Cfg.Debug {
		log.Println(item.SQL, stmtArgs)
	}

	res, err := t.DB.ExecContext(t.ctx, item.SQL, stmtArgs...)
	if err != nil {
		return 0, err
	}

	row, _ := res.RowsAffected()
	return int(row), nil
}

// Exec executes a raw SQL statement with optional parameters
// Returns the number of affected rows and any error
func (t *ZormTable) Exec(query string, args ...interface{}) (int, error) {
	if config.Mock {
		pc, fileName, _, _ := runtime.Caller(1)
		if ok, _, n, e := checkMock(t.Name, "Exec", runtime.FuncForPC(pc).Name(), fileName, path.Dir(fileName)); ok {
			return n, e
		}
	}

	if t.Cfg.Debug {
		log.Println(query, args)
	}

	// 检查是否是 SELECT 语句，SELECT 语句不应该使用 Exec
	queryTrimmed := strings.TrimSpace(strings.ToUpper(query))
	if strings.HasPrefix(queryTrimmed, "SELECT") {
		// SELECT 语句返回 0 行受影响
		return 0, nil
	}

	res, err := t.DB.ExecContext(t.ctx, query, args...)
	if err != nil {
		return 0, err
	}

	rowsAffected, _ := res.RowsAffected()
	return int(rowsAffected), nil
}

func (t *ZormTable) inputArgs(stmtArgs *[]interface{}, cols []reflect2.StructField, argIndex [][]int, rtPtr, s reflect2.Type, ptr bool, x unsafe.Pointer) {
	// 使用 reflect 包获取结构体值，以便正确处理嵌入结构体
	var rv reflect.Value
	if ptr {
		// 对于指针类型，x 可能是指向指针的指针（如 **User），需要先解引用
		// 先获取 *User 的值
		ptrVal := *(*unsafe.Pointer)(x)
		if ptrVal == nil {
			// 如果指针为 nil，无法获取字段值，为每个字段添加 nil
			for range cols {
				*stmtArgs = append(*stmtArgs, nil)
			}
			return
		}
		// rtPtr 是 *User 类型，PackEFace 需要切片元素的 **User 地址。
		structPtr := rtPtr.PackEFace(x)
		rv = reflect.ValueOf(structPtr)
		// 确保 rv 不是指针，如果是则解引用
		for rv.Kind() == reflect.Ptr {
			if rv.IsNil() {
				// 如果指针为 nil，为每个字段添加 nil
				for range cols {
					*stmtArgs = append(*stmtArgs, nil)
				}
				return
			}
			rv = rv.Elem()
		}
	} else {
		// 对于值类型，直接获取
		structVal := s.PackEFace(x)
		rv = reflect.ValueOf(structVal)
		// 确保 rv 不是指针，如果是则解引用
		for rv.Kind() == reflect.Ptr {
			if rv.IsNil() {
				return
			}
			rv = rv.Elem()
		}
	}

	for i, col := range cols {
		var v interface{}

		var fieldVal reflect.Value
		if len(argIndex) == len(cols) && len(argIndex[i]) > 0 {
			fieldVal = rv.FieldByIndex(argIndex[i])
		} else {
			// 使用 reflect 包获取字段值（支持嵌入结构体）
			fieldVal = rv.FieldByName(col.Name())
			if !fieldVal.IsValid() {
				// 如果直接字段名找不到，尝试在嵌入结构体中查找
				for i := 0; i < rv.NumField(); i++ {
					field := rv.Type().Field(i)
					if field.Anonymous {
						embeddedVal := rv.Field(i)
						if embeddedVal.Kind() == reflect.Struct {
							fieldVal = embeddedVal.FieldByName(col.Name())
							if fieldVal.IsValid() {
								break
							}
						}
					}
				}
			}
		}

		if fieldVal.IsValid() {
			// 指针字段：根据是否为 nil 决定写入 NULL 还是实际值
			if fieldVal.Kind() == reflect.Ptr {
				if fieldVal.IsNil() {
					v = nil
				} else {
					v = fieldVal.Elem().Interface()
				}
			} else {
				v = fieldVal.Interface()
			}
		}

		// 时间类型特殊处理（同时支持 time.Time 和 *time.Time）
		if v != nil {
			if t.Cfg.ToTimestamp {
				switch tv := v.(type) {
				case *time.Time:
					if tv != nil {
						v = tv.UTC().Unix()
					} else {
						v = nil
					}
				case time.Time:
					v = tv.UTC().Unix()
				}
			} else {
				switch tv := v.(type) {
				case *time.Time:
					if tv != nil {
						v = tv.UTC().Format(_timeLayout)
					} else {
						v = nil
					}
				case time.Time:
					v = tv.UTC().Format(_timeLayout)
				}
			}
		}

		*stmtArgs = append(*stmtArgs, v)
	}
}

// ZormDBIFace .
type ZormDBIFace interface {
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

// ZormTxIFace 事务接口
type ZormTxIFace interface {
	ZormDBIFace
	Commit() error
	Rollback() error
}

type stmtPreparer interface {
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
}

type stmtCacheEntry struct {
	query string
	stmt  *sql.Stmt
	elem  *list.Element
}

// StmtCacheDB wraps a database handle with a bounded prepared-statement cache.
type StmtCacheDB struct {
	db       ZormDBIFace
	preparer stmtPreparer
	capacity int

	mu      sync.Mutex
	entries map[string]*stmtCacheEntry
	lru     *list.List
}

func NewStmtCacheDB(db ZormDBIFace, capacity int) *StmtCacheDB {
	preparer, ok := db.(stmtPreparer)
	if !ok || capacity <= 0 {
		return nil
	}
	return &StmtCacheDB{
		db:       db,
		preparer: preparer,
		capacity: capacity,
		entries:  make(map[string]*stmtCacheEntry, capacity),
		lru:      list.New(),
	}
}

func wrapDefaultStmtCacheDB(db ZormDBIFace) ZormDBIFace {
	if db == nil {
		return nil
	}
	if _, ok := db.(*StmtCacheDB); ok {
		return db
	}
	if _, ok := db.(*sql.Tx); ok {
		return db
	}
	if _, ok := db.(stmtPreparer); !ok {
		return db
	}
	if key, ok := stmtCacheWrapperKey(db); ok {
		if cached, ok := _stmtCacheWrapperCache.Load(key); ok {
			return cached.(ZormDBIFace)
		}
		wrapped := NewStmtCacheDB(db, defaultStmtCacheSize)
		if wrapped == nil {
			return db
		}
		actual, _ := _stmtCacheWrapperCache.LoadOrStore(key, wrapped)
		return actual.(ZormDBIFace)
	}
	if wrapped := NewStmtCacheDB(db, defaultStmtCacheSize); wrapped != nil {
		return wrapped
	}
	return db
}

func stmtCacheWrapperKey(db ZormDBIFace) (interface{}, bool) {
	t := reflect.TypeOf(db)
	if t == nil || !t.Comparable() {
		return nil, false
	}
	return db, true
}

func (db *StmtCacheDB) getStmt(ctx context.Context, query string) (*sql.Stmt, error) {
	db.mu.Lock()
	if entry, ok := db.entries[query]; ok {
		db.lru.MoveToFront(entry.elem)
		stmt := entry.stmt
		db.mu.Unlock()
		return stmt, nil
	}

	stmt, err := db.preparer.PrepareContext(ctx, query)
	if err != nil {
		db.mu.Unlock()
		return nil, err
	}

	entry := &stmtCacheEntry{query: query, stmt: stmt}
	entry.elem = db.lru.PushFront(entry)
	db.entries[query] = entry

	var evicted *sql.Stmt
	if len(db.entries) > db.capacity {
		back := db.lru.Back()
		if back != nil {
			old := back.Value.(*stmtCacheEntry)
			delete(db.entries, old.query)
			db.lru.Remove(back)
			evicted = old.stmt
		}
	}
	db.mu.Unlock()

	if evicted != nil {
		_ = evicted.Close()
	}
	return stmt, nil
}

func (db *StmtCacheDB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	stmt, err := db.getStmt(ctx, query)
	if err != nil {
		return db.db.QueryRowContext(ctx, query, args...)
	}
	return stmt.QueryRowContext(ctx, args...)
}

func (db *StmtCacheDB) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	stmt, err := db.getStmt(ctx, query)
	if err != nil {
		return db.db.QueryContext(ctx, query, args...)
	}
	return stmt.QueryContext(ctx, args...)
}

func (db *StmtCacheDB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	stmt, err := db.getStmt(ctx, query)
	if err != nil {
		return db.db.ExecContext(ctx, query, args...)
	}
	return stmt.ExecContext(ctx, args...)
}

func (db *StmtCacheDB) Unwrap() ZormDBIFace {
	return db.db
}

// ZormTable .
type ZormTable struct {
	DB   ZormDBIFace
	Name string
	Cfg  Config
	ctx  context.Context

	lastBindingCache atomic.Pointer[dataBindingCache]
}

type structFieldMeta struct {
	DBName string
	Field  reflect2.StructField
}

type structFieldSet struct {
	Ordered       []structFieldMeta
	DefaultSQL    string
	FieldMap      map[string]reflect2.StructField
	AutoIncrement reflect2.StructField
}

// camelToSnake converts camelCase to snake_case
func camelToSnake(s string) string {
	if s == "" {
		return s
	}

	var result []rune
	runes := []rune(s)

	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) {
			// Add underscore before uppercase letter (except if previous char is also uppercase)
			if i > 0 && !unicode.IsUpper(runes[i-1]) {
				result = append(result, '_')
			}
		}
		result = append(result, unicode.ToLower(r))
	}

	return string(result)
}

// getFieldName returns the database field name for a struct field
// Supported formats:
// - zorm:"-" - ignore field
// - zorm:"field_name,auto_incr" - use field_name as DB column, auto_incr as supplement
// - zorm:"field_name" - use field_name as DB column
// - zorm:"auto_incr" - use field name converted to snake_case, auto_incr as supplement
// - empty tag - convert field name from camelCase to snake_case
func getFieldName(f reflect2.StructField) string {
	ft := f.Tag().Get("zorm")

	// If tag is "-", skip this field
	if ft == "-" {
		return ""
	}

	// If tag is empty, convert field name to snake_case
	if ft == "" {
		return camelToSnake(f.Name())
	}

	// Parse tag: "field_name,auto_incr" or "field_name" or "auto_incr"
	tags := strings.Split(ft, ",")
	fieldName := strings.TrimSpace(tags[0])

	// If field name is empty or just "auto_incr", use converted field name
	if fieldName == "" || fieldName == "auto_incr" {
		return camelToSnake(f.Name())
	}

	// Use the specified field name
	return fieldName
}

func fieldEscape(sb *strings.Builder, field string) {
	if field == "" {
		return
	}
	if !strings.ContainsAny(field, ",( `.") {
		sb.WriteByte('`')
		sb.WriteString(field)
		sb.WriteByte('`')
	} else {
		// TODO: 处理alias场景
		sb.WriteString(field)
	}
}

func setReflectMapValue(mapVal reflect.Value, field string, raw interface{}) {
	key := reflect.ValueOf(field)
	if mapKeyType := mapVal.Type().Key(); key.Type() != mapKeyType {
		key = key.Convert(mapKeyType)
	}

	valueType := mapVal.Type().Elem()
	if raw == nil {
		mapVal.SetMapIndex(key, reflect.Zero(valueType))
		return
	}

	value := reflect.ValueOf(raw)
	if value.Type().AssignableTo(valueType) {
		mapVal.SetMapIndex(key, value)
		return
	}
	if value.Type().ConvertibleTo(valueType) {
		mapVal.SetMapIndex(key, value.Convert(valueType))
		return
	}
	if valueType.Kind() == reflect.Interface {
		mapVal.SetMapIndex(key, value)
		return
	}
	mapVal.SetMapIndex(key, reflect.Zero(valueType))
}

func (t *ZormTable) getStructFieldSet(s reflect2.StructType) *structFieldSet {
	typeKey := s.Type1()
	if cached, ok := _structFieldCache.Load(typeKey); ok {
		return cached.(*structFieldSet)
	}

	set := &structFieldSet{FieldMap: make(map[string]reflect2.StructField)}
	t.collectStructFieldSet(s, set)
	set.DefaultSQL = buildFieldsSQL(setFieldNames(set.Ordered))
	_structFieldCache.Store(typeKey, set)
	return set
}

func (t *ZormTable) getStructFieldMap(s reflect2.StructType) map[string]reflect2.StructField {
	return t.getStructFieldSet(s).FieldMap
}

func (t *ZormTable) collectStructFieldSet(s reflect2.StructType, set *structFieldSet) {
	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		ft := f.Tag().Get("zorm")

		if ft == "-" {
			continue
		}
		if f.Name() == "ZormLastId" {
			continue
		}
		if f.Anonymous() && f.Type().Kind() == reflect.Struct {
			t.collectStructFieldSet(f.Type().(reflect2.StructType), set)
			continue
		}
		if set.AutoIncrement == nil && isAutoIncrementField(f) {
			set.AutoIncrement = f
		}

		dbFieldName := getFieldName(f)
		if dbFieldName == "" {
			continue
		}

		set.Ordered = append(set.Ordered, structFieldMeta{DBName: dbFieldName, Field: f})
		set.FieldMap[dbFieldName] = f
	}
}

func setFieldNames(fields []structFieldMeta) []string {
	names := make([]string, 0, len(fields))
	for _, field := range fields {
		names = append(names, field.DBName)
	}
	return names
}

// isAutoIncrementField 检查字段是否为自增主键
// 支持格式：
// - zorm:"field_name,auto_incr" - 指定字段名且为自增
// - zorm:"auto_incr" - 使用转换后的字段名且为自增
func isAutoIncrementField(f reflect2.StructField) bool {
	ft := f.Tag().Get("zorm")
	if ft == "" || ft == "-" {
		return false
	}

	for {
		part := ft
		if idx := strings.IndexByte(ft, ','); idx >= 0 {
			part = ft[:idx]
			ft = ft[idx+1:]
		} else {
			ft = ""
		}
		if strings.TrimSpace(part) == "auto_incr" {
			return true
		}
		if ft == "" {
			return false
		}
	}
}

// getAutoIncrementField 获取自增主键字段（支持嵌入结构体）
func (t *ZormTable) getAutoIncrementField(s reflect2.StructType) reflect2.StructField {
	return t.getStructFieldSet(s).AutoIncrement
}

// collectFieldsForInsert 收集字段用于INSERT操作，支持embedded struct
func (t *ZormTable) collectFieldsForInsert(s reflect2.StructType, sb *strings.Builder, cols *[]reflect2.StructField) {
	t.collectFieldsForInsertWithPrefix(s, sb, cols, "")
}

// collectFieldsForInsertWithPrefix 递归收集字段，支持embedded struct和前缀
// 只支持 zorm:"auto_incr" 和 zorm:"-" 标签
// 自动将驼峰命名转换为蛇形命名
func (t *ZormTable) collectFieldsForInsertWithPrefix(s reflect2.StructType, sb *strings.Builder, cols *[]reflect2.StructField, prefix string) {
	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		ft := f.Tag().Get("zorm")

		// 忽略zorm tag为"-"的字段
		if ft == "-" {
			continue
		}

		// 忽略 ZormLastId 字段（向后兼容字段，不是数据库列）
		if f.Name() == "ZormLastId" {
			continue
		}

		// 忽略自增主键字段
		if isAutoIncrementField(f) {
			continue
		}

		// 处理embedded struct
		if f.Anonymous() && f.Type().Kind() == reflect.Struct {
			embeddedStruct := f.Type().(reflect2.StructType)
			t.collectFieldsForInsertWithPrefix(embeddedStruct, sb, cols, prefix)
			continue
		}

		// 处理普通字段 - 总是包含，忽略UseNameWhenTagEmpty配置
		if len(*cols) > 0 {
			sb.WriteString(",")
		}

		// 获取数据库字段名（自动转换驼峰为蛇形）
		dbFieldName := getFieldName(f)
		if dbFieldName == "" {
			continue // 跳过被忽略的字段
		}

		// 构建完整的字段名（包含前缀）
		fullFieldName := dbFieldName
		if prefix != "" {
			fullFieldName = prefix + "." + dbFieldName
		}

		fieldEscape(sb, fullFieldName)
		*cols = append(*cols, f)
	}
}

// collectFieldsGeneric 通用字段收集函数，支持struct和map
func (t *ZormTable) collectFieldsGeneric(objs interface{}, rt reflect2.Type, sb *strings.Builder, fieldInfos *[]FieldInfo) error {
	switch rt.Kind() {
	case reflect.Struct:
		return t.collectStructFieldsGeneric(rt.(reflect2.StructType), sb, fieldInfos, "")
	case reflect.Map:
		return t.collectMapFieldsGeneric(objs, rt.(reflect2.MapType), sb, fieldInfos)
	default:
		return errors.New("unsupported type for field collection")
	}
}

// collectStructFieldsGeneric 收集struct字段，返回通用FieldInfo
// 自动将驼峰命名转换为蛇形命名
func (t *ZormTable) collectStructFieldsGeneric(s reflect2.StructType, sb *strings.Builder, fieldInfos *[]FieldInfo, prefix string) error {
	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		ft := f.Tag().Get("zorm")

		// 忽略zorm tag为"-"的字段
		if ft == "-" {
			continue
		}

		// 忽略 ZormLastId 字段（向后兼容字段，不是数据库列）
		if f.Name() == "ZormLastId" {
			continue
		}

		// 处理embedded struct
		if f.Anonymous() && f.Type().Kind() == reflect.Struct {
			embeddedStruct := f.Type().(reflect2.StructType)
			if err := t.collectStructFieldsGeneric(embeddedStruct, sb, fieldInfos, prefix); err != nil {
				return err
			}
			continue
		}

		// 获取数据库字段名（自动转换驼峰为蛇形）
		dbFieldName := getFieldName(f)
		if dbFieldName == "" {
			continue // 跳过被忽略的字段
		}

		if len(*fieldInfos) > 0 {
			sb.WriteString(",")
		}

		// 构建完整的字段名（包含前缀）
		fullFieldName := dbFieldName
		if prefix != "" {
			fullFieldName = prefix + "." + dbFieldName
		}

		fieldEscape(sb, fullFieldName)
		*fieldInfos = append(*fieldInfos, &StructFieldInfo{field: f})
	}
	return nil
}

// collectMapFieldsGeneric 收集map字段，返回通用FieldInfo
func (t *ZormTable) collectMapFieldsGeneric(objs interface{}, mapType reflect2.MapType, sb *strings.Builder, fieldInfos *[]FieldInfo) error {
	// 检查key类型
	keyType := mapType.Key()
	if keyType.Kind() != reflect.String {
		return errors.New("map key must be string type")
	}

	// 使用reflect包获取map的迭代器
	rv := reflect.ValueOf(objs)
	mapIter := rv.MapRange()

	// 用于存储字段信息的临时结构
	type mapFieldData struct {
		key       string
		value     interface{}
		valueType reflect2.Type
	}

	var fieldDataList []mapFieldData

	// 收集map中的所有字段
	for mapIter.Next() {
		key := mapIter.Key()
		value := mapIter.Value()
		keyStr := key.String()
		fieldDataList = append(fieldDataList, mapFieldData{
			key:       keyStr,
			value:     value.Interface(),
			valueType: mapType.Elem(),
		})
	}

	// 按key排序，确保字段顺序一致
	sort.Slice(fieldDataList, func(i, j int) bool {
		return fieldDataList[i].key < fieldDataList[j].key
	})

	// 构建SQL字段部分和FieldInfo
	for i, fieldData := range fieldDataList {
		if i > 0 {
			sb.WriteString(",")
		}
		fieldEscape(sb, fieldData.key)

		*fieldInfos = append(*fieldInfos, &MapFieldInfo{
			key:       fieldData.key,
			value:     fieldData.value,
			valueType: fieldData.valueType,
		})
	}

	return nil
}

// FieldInfo 通用字段信息接口
type FieldInfo interface {
	GetName() string
	GetValue(ptr unsafe.Pointer) interface{}
	GetType() reflect2.Type
}

// StructFieldInfo struct字段信息实现
type StructFieldInfo struct {
	field reflect2.StructField
}

func (f *StructFieldInfo) GetName() string {
	return f.field.Name()
}

func (f *StructFieldInfo) GetValue(ptr unsafe.Pointer) interface{} {
	return f.field.UnsafeGet(ptr)
}

func (f *StructFieldInfo) GetType() reflect2.Type {
	return f.field.Type()
}

// MapFieldInfo map字段信息实现
type MapFieldInfo struct {
	key       string
	value     interface{}
	valueType reflect2.Type
}

func (f *MapFieldInfo) GetName() string {
	return f.key
}

func (f *MapFieldInfo) GetValue(ptr unsafe.Pointer) interface{} {
	return f.value
}

func (f *MapFieldInfo) GetType() reflect2.Type {
	return f.valueType
}

// ZormItem .
type ZormItem interface {
	Type() int
	BuildSQL(*strings.Builder)
	BuildArgs(*[]interface{})
}

type fieldsItem struct {
	Fields []string
	SQL    string
	Shape  string
}

func (w *fieldsItem) Type() int {
	return _fields
}

func (w *fieldsItem) ShapeKeyPart() string {
	return w.Shape
}

func (w *fieldsItem) BuildSQL(sb *strings.Builder) {
	if w.SQL != "" {
		sb.WriteString(w.SQL)
		return
	}
	for i, field := range w.Fields {
		if i > 0 {
			sb.WriteString(",")
		}
		if field == "*" {
			sb.WriteString("*")
		} else {
			fieldEscape(sb, field)
		}
	}
}

func (w *fieldsItem) BuildArgs(stmtArgs *[]interface{}) {
}

type onConflictDoUpdateSetItem struct {
	Conds string
}

func (w *onConflictDoUpdateSetItem) Type() int {
	return _onConflictDoUpdateSet
}

func (w *onConflictDoUpdateSetItem) ShapeKeyPart() string {
	return buildItemShapePart(_onConflictDoUpdateSet, w.Conds)
}

func (w *onConflictDoUpdateSetItem) BuildSQL(sb *strings.Builder) {
	sb.WriteString(w.Conds)
}

func (w *onConflictDoUpdateSetItem) BuildArgs(stmtArgs *[]interface{}) {
	// OnConflictDoUpdateSet 使用 excluded. 语法，不需要额外的参数
}

type joinItem struct {
	Stmt     string        // 原始语句（向后兼容）
	JoinType string        // 连接类型：LEFT JOIN, RIGHT JOIN, INNER JOIN, FULL OUTER JOIN
	Table    string        // 表名
	On       []interface{} // ON 条件
}

func (w *joinItem) Type() int {
	return _join
}

func (w *joinItem) BuildSQL(sb *strings.Builder) {
	sb.WriteString(" ")

	// 如果有原始语句，直接使用（向后兼容）
	if w.Stmt != "" {
		sb.WriteString(w.Stmt)
		return
	}

	// 构建新的联表查询语句
	if w.JoinType != "" && w.Table != "" {
		sb.WriteString(w.JoinType)
		sb.WriteString(" ")
		fieldEscape(sb, w.Table)

		if len(w.On) > 0 {
			sb.WriteString(" ON ")
			// 构建 ON 条件，类似 Where 的处理方式
			if s, ok := w.On[0].(string); ok {
				// 字符串格式：直接使用
				sb.WriteString(s)
			} else if whereItem, ok := w.On[0].(*whereItem); ok {
				// 如果是 whereItem，提取其条件并构建 ON 子句
				for i, c := range whereItem.Conds {
					if i > 0 {
						sb.WriteString(" AND ")
					}
					if cond, ok := c.(*ormCond); ok {
						cond.BuildSQL(sb)
					} else if expr, ok := c.(*ormExpr); ok {
						expr.BuildSQL(sb)
					} else if condEx, ok := c.(*ormCondEx); ok {
						if len(condEx.Conds) > 1 {
							sb.WriteString("(")
						}
						condEx.BuildSQL(sb)
						if len(condEx.Conds) > 1 {
							sb.WriteString(")")
						}
					}
				}
			} else {
				// 条件对象格式：使用类似 Where 的处理
				for i, raw := range w.On {
					if i > 0 {
						sb.WriteString(" AND ")
					}
					if condEx, ok := raw.(*ormCondEx); ok {
						if len(condEx.Conds) > 1 {
							sb.WriteString("(")
						}
						condEx.BuildSQL(sb)
						if len(condEx.Conds) > 1 {
							sb.WriteString(")")
						}
					} else if cond, ok := raw.(*ormCond); ok {
						cond.BuildSQL(sb)
					} else if expr, ok := raw.(*ormExpr); ok {
						expr.BuildSQL(sb)
					}
				}
			}
		}
	}
}

func (w *joinItem) BuildArgs(stmtArgs *[]interface{}) {
	// 处理 ON 条件中的参数
	if len(w.On) > 0 {
		if _, ok := w.On[0].(string); ok {
			// 字符串格式：检查是否有占位符参数
			if len(w.On) > 1 {
				*stmtArgs = append(*stmtArgs, w.On[1:]...)
			}
		} else if whereItem, ok := w.On[0].(*whereItem); ok {
			// 如果是 whereItem，提取其条件并构建参数
			for _, c := range whereItem.Conds {
				if cond, ok := c.(*ormCond); ok {
					cond.BuildArgs(stmtArgs)
				} else if expr, ok := c.(*ormExpr); ok {
					expr.BuildArgs(stmtArgs)
				} else if condEx, ok := c.(*ormCondEx); ok {
					condEx.BuildArgs(stmtArgs)
				}
			}
		} else {
			// 条件对象格式：处理条件中的参数
			for _, raw := range w.On {
				if condEx, ok := raw.(*ormCondEx); ok {
					condEx.BuildArgs(stmtArgs)
				} else if cond, ok := raw.(*ormCond); ok {
					cond.BuildArgs(stmtArgs)
				} else if expr, ok := raw.(*ormExpr); ok {
					expr.BuildArgs(stmtArgs)
				}
			}
		}
	}
}

type indexedByItem struct {
	idx string
}

func (w *indexedByItem) Type() int {
	return _indexedBy
}

func (w *indexedByItem) ShapeKeyPart() string {
	return buildItemShapePart(_indexedBy, " indexed by "+w.idx)
}

func (w *indexedByItem) BuildSQL(sb *strings.Builder) {
	sb.WriteString(" indexed by " + w.idx)
}

func (w *indexedByItem) BuildArgs(stmtArgs *[]interface{}) {}

type whereItem struct {
	Conds []interface{}
	SQL   string
	Shape string
}

func (w *whereItem) Type() int {
	return _where
}

func (w *whereItem) ShapeKeyPart() string {
	return w.Shape
}

func (w *whereItem) BuildSQL(sb *strings.Builder) {
	if w.SQL != "" {
		sb.WriteString(w.SQL)
		return
	}
	writeCondListSQL(sb, w.Conds, " where ")
}

func (w *whereItem) BuildArgs(stmtArgs *[]interface{}) {
	for _, c := range w.Conds {
		if cond, ok := c.(*ormCond); ok {
			*stmtArgs = append(*stmtArgs, cond.Args...)
		} else if expr, ok := c.(*ormExpr); ok {
			expr.BuildArgs(stmtArgs)
		} else if condEx, ok := c.(*ormCondEx); ok {
			condEx.BuildArgs(stmtArgs)
		}
	}
}

type groupByItem struct {
	Fields []string
	SQL    string
	Shape  string
}

func (w *groupByItem) Type() int {
	return _groupBy
}

func (w *groupByItem) ShapeKeyPart() string {
	return w.Shape
}

func (w *groupByItem) BuildSQL(sb *strings.Builder) {
	if w.SQL != "" {
		sb.WriteString(w.SQL)
		return
	}
	sb.WriteString(buildGroupBySQL(w.Fields))
}

func (w *groupByItem) BuildArgs(stmtArgs *[]interface{}) {
}

type havingItem struct {
	Conds []interface{}
	SQL   string
	Shape string
}

func (h *havingItem) Type() int {
	return _having
}

func (h *havingItem) ShapeKeyPart() string {
	return h.Shape
}

func (h *havingItem) BuildSQL(sb *strings.Builder) {
	if h.SQL != "" {
		sb.WriteString(h.SQL)
		return
	}
	writeCondListSQL(sb, h.Conds, " having ")
}

func (h *havingItem) BuildArgs(stmtArgs *[]interface{}) {
	for _, c := range h.Conds {
		if cond, ok := c.(*ormCond); ok {
			*stmtArgs = append(*stmtArgs, cond.Args...)
		} else if expr, ok := c.(*ormExpr); ok {
			expr.BuildArgs(stmtArgs)
		} else if condEx, ok := c.(*ormCondEx); ok {
			condEx.BuildArgs(stmtArgs)
		}
	}
}

type orderByItem struct {
	Orders []string
	SQL    string
	Shape  string
}

func (o *orderByItem) Type() int {
	return _orderBy
}

func (o *orderByItem) ShapeKeyPart() string {
	return o.Shape
}

func (o *orderByItem) BuildSQL(sb *strings.Builder) {
	if o.SQL != "" {
		sb.WriteString(o.SQL)
		return
	}
	sb.WriteString(buildOrderBySQL(o.Orders))
}

func (o *orderByItem) BuildArgs(stmtArgs *[]interface{}) {
}

type limitItem struct {
	I     []interface{}
	SQL   string
	Shape string
}

func (l *limitItem) Type() int {
	return _limit
}

func (l *limitItem) ShapeKeyPart() string {
	return l.Shape
}

func (l *limitItem) BuildSQL(sb *strings.Builder) {
	if l.SQL != "" {
		sb.WriteString(l.SQL)
		return
	}
	sb.WriteString(buildLimitSQL(l.I))
}

func (l *limitItem) BuildArgs(stmtArgs *[]interface{}) {
	*stmtArgs = append(*stmtArgs, l.I...)
}

func strconvErr(err error) error {
	if ne, ok := err.(*strconv.NumError); ok {
		return ne.Err
	}
	return err
}

type scanner struct {
	Type reflect2.Type
	Val  unsafe.Pointer
}

func numberToString(k reflect.Kind, src interface{}) string {
	switch k {
	case reflect.Bool:
		if src.(bool) {
			return "true"
		}
		return "false"
	case reflect.Int64:
		return fmt.Sprintf("%d", src.(int64))
	case reflect.Float64:
		return fmt.Sprintf("%g", src.(float64))
	}
	return ""
}

func toUnix(year, month, day, hour, min, sec int) int64 {
	if month < 1 || month > 12 {
		return -1
	}

	leap := 0
	if (year%4 == 0 && (year%100 != 0 || year%400 == 0)) && month >= 3 {
		leap = 1 // February 29
	}

	return int64((365*year-719528+day-1+(year+3)/4-(year+99)/100+(year+399)/400+int([]int{0, 31, 59, 90, 120, 151, 181, 212, 243, 273, 304, 334}[month-1])+leap)*86400 + hour*3600 + min*60 + sec)
}

// parseTimeString 优化的时间字符串解析函数
func parseTimeString(s string) (time.Time, error) {
	// 处理空字符串或NULL值
	if s == "" || s == "NULL" || s == "null" {
		return time.Time{}, nil
	}

	// 快速检查字符串长度和特征
	if len(s) < 10 {
		return time.Time{}, fmt.Errorf("time string too short")
	}

	// 检查是否包含时区信息
	hasTimezone := false
	hasNano := false

	// 检查时区标识符
	if s[len(s)-1] == 'Z' ||
		(len(s) >= 6 && (s[len(s)-6:] == "+00:00" || s[len(s)-6:] == "-00:00")) ||
		(len(s) >= 5 && (s[len(s)-5:] == "+0000" || s[len(s)-5:] == "-0000")) {
		hasTimezone = true
	} else if len(s) >= 6 {
		// 检查是否有 +/- 时区标识
		for i := len(s) - 6; i < len(s); i++ {
			if s[i] == '+' || s[i] == '-' {
				hasTimezone = true
				break
			}
		}
	}

	// 特殊处理：纯日期格式（如 2019-03-01）不应该被当作有时区处理
	if len(s) == 10 && s[4] == '-' && s[7] == '-' {
		hasTimezone = false
	}

	// 检查是否有毫秒/微秒
	if len(s) > 19 && s[19] == '.' {
		hasNano = true
	}

	// 根据特征选择解析格式
	var layout string
	if hasTimezone {
		if hasNano {
			// 带时区和毫秒的格式
			if len(s) > 10 && s[10] == 'T' {
				layout = time.RFC3339Nano // 2006-01-02T15:04:05.999999999Z07:00
			} else {
				layout = _timeLayoutWithNanoTZ
			}
		} else {
			// 带时区但无毫秒的格式
			if len(s) > 10 && s[10] == 'T' {
				layout = time.RFC3339 // 2006-01-02T15:04:05Z07:00
			} else if s[len(s)-1] == 'Z' {
				layout = _timeLayoutWithZ
			} else {
				layout = _timeLayoutWithTZ
			}
		}
	} else {
		// 无时区信息，根据长度选择格式
		if len(s) == 10 && s[4] == '-' && s[7] == '-' {
			// 纯日期格式
			layout = "2006-01-02"
		} else {
			// 标准日期时间格式
			layout = "2006-01-02 15:04:05"
		}
	}

	return time.Parse(layout, s)
}

func scanFromString(isTime bool, st reflect2.Type, dt reflect2.Type, ptrVal unsafe.Pointer, tmp string) error {
	dk := dt.Kind()

	// 时间格式(DATE/DATETIME) => number/time.Time
	if isTime || (dk >= reflect.Int && dk <= reflect.Float64) {
		// 优化的时间解析：先分析字符串特征，再选择对应的解析方法
		if isTime {
			parsedTime, err := parseTimeString(tmp)
			if err == nil {
				*(*time.Time)(ptrVal) = parsedTime.UTC()
				return nil
			}
		}

		// 如果带时区解析失败，尝试原有的简单格式解析
		var year, month, day, hour, min, sec int
		n, _ := fmt.Sscanf(tmp, "%4d-%2d-%2d %2d:%2d:%2d", &year, &month, &day, &hour, &min, &sec)
		if n == 3 || n == 6 {
			if isTime {
				*(*time.Time)(ptrVal) = time.Unix(toUnix(year, month, day, hour, min, sec), 0).UTC()
				return nil
			}
			ts := toUnix(year, month, day, hour, min, sec)
			switch dk {
			case reflect.Int:
				*(*int)(ptrVal) = int(ts)
			case reflect.Int8:
				*(*int8)(ptrVal) = int8(ts)
			case reflect.Int16:
				*(*int16)(ptrVal) = int16(ts)
			case reflect.Int32:
				*(*int32)(ptrVal) = int32(ts)
			case reflect.Int64:
				*(*int64)(ptrVal) = ts
			case reflect.Uint:
				*(*uint)(ptrVal) = uint(ts)
			case reflect.Uint8:
				*(*uint8)(ptrVal) = uint8(ts)
			case reflect.Uint16:
				*(*uint16)(ptrVal) = uint16(ts)
			case reflect.Uint32:
				*(*uint32)(ptrVal) = uint32(ts)
			case reflect.Uint64:
				*(*uint64)(ptrVal) = uint64(ts)
			case reflect.Float32:
				*(*float32)(ptrVal) = float32(ts)
			case reflect.Float64:
				*(*float64)(ptrVal) = float64(ts)
			}
			return nil
		} else if isTime {
			// 获取数值时间戳
			i64, err := strconv.ParseInt(tmp, 10, 64)
			if err != nil {
				return fmt.Errorf("converting driver.Value type %s (%s) to a %s: %v", st.String(), tmp, dk, strconvErr(err))
			}
			*(*time.Time)(ptrVal) = time.Unix(i64, 0).UTC()
			return nil
		}
	}

	// 非时间格式
	switch dk {
	case reflect.Bool:
		*(*bool)(ptrVal) = (tmp == "true")
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i64, err := strconv.ParseInt(tmp, 10, dt.Type1().Bits())
		if err != nil {
			return fmt.Errorf("converting driver.Value type %s (%s) to a %s: %v", st.String(), tmp, dk, strconvErr(err))
		}
		switch dk {
		case reflect.Int64:
			*(*int64)(ptrVal) = i64
		case reflect.Int32:
			*(*int32)(ptrVal) = int32(i64)
		case reflect.Int16:
			*(*int16)(ptrVal) = int16(i64)
		case reflect.Int8:
			*(*int8)(ptrVal) = int8(i64)
		case reflect.Int:
			*(*int)(ptrVal) = int(i64)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u64, err := strconv.ParseUint(tmp, 10, dt.Type1().Bits())
		if err != nil {
			return fmt.Errorf("converting driver.Value type %s (%s) to a %s: %v", st.String(), tmp, dk, strconvErr(err))
		}
		switch dk {
		case reflect.Uint64:
			*(*uint64)(ptrVal) = u64
		case reflect.Uint32:
			*(*uint32)(ptrVal) = uint32(u64)
		case reflect.Uint16:
			*(*uint16)(ptrVal) = uint16(u64)
		case reflect.Uint8:
			*(*uint8)(ptrVal) = uint8(u64)
		case reflect.Uint:
			*(*uint)(ptrVal) = uint(u64)
		}
	case reflect.Float32, reflect.Float64:
		f64, err := strconv.ParseFloat(tmp, dt.Type1().Bits())
		if err != nil {
			return fmt.Errorf("converting driver.Value type %s (%s) to a %s: %v", st.String(), tmp, dk, strconvErr(err))
		}
		if dk == reflect.Float32 {
			*(*float32)(ptrVal) = float32(f64)
		} else {
			*(*float64)(ptrVal) = f64
		}
	case reflect.String:
		*(*string)(ptrVal) = tmp
	default:
		// string => []byte
		if dk == reflect.Slice && dt.(reflect2.SliceType).Elem().Kind() == reflect.Uint8 {
			*(*[]byte)(ptrVal) = reflect2.UnsafeCastString(tmp)
			return nil
		}
		// TODO 自定义类型，尝试转换
		return fmt.Errorf("converting driver.Value type %s (%s) to a %s", st.String(), tmp, dt.String())
	}
	return nil
}

func setSignedIntegerValue(ptr unsafe.Pointer, kind reflect.Kind, v int64) error {
	switch kind {
	case reflect.Int:
		if strconv.IntSize == 32 && (v < math.MinInt32 || v > math.MaxInt32) {
			return fmt.Errorf("value out of range")
		}
		*(*int)(ptr) = int(v)
	case reflect.Int8:
		if v < math.MinInt8 || v > math.MaxInt8 {
			return fmt.Errorf("value out of range")
		}
		*(*int8)(ptr) = int8(v)
	case reflect.Int16:
		if v < math.MinInt16 || v > math.MaxInt16 {
			return fmt.Errorf("value out of range")
		}
		*(*int16)(ptr) = int16(v)
	case reflect.Int32:
		if v < math.MinInt32 || v > math.MaxInt32 {
			return fmt.Errorf("value out of range")
		}
		*(*int32)(ptr) = int32(v)
	case reflect.Int64:
		*(*int64)(ptr) = v
	default:
		return fmt.Errorf("unsupported signed integer kind %s", kind)
	}
	return nil
}

func setUnsignedIntegerValue(ptr unsafe.Pointer, kind reflect.Kind, v int64) error {
	if v < 0 {
		return fmt.Errorf("value out of range")
	}
	uv := uint64(v)
	switch kind {
	case reflect.Uint:
		if strconv.IntSize == 32 && uv > math.MaxUint32 {
			return fmt.Errorf("value out of range")
		}
		*(*uint)(ptr) = uint(uv)
	case reflect.Uint8:
		if uv > math.MaxUint8 {
			return fmt.Errorf("value out of range")
		}
		*(*uint8)(ptr) = uint8(uv)
	case reflect.Uint16:
		if uv > math.MaxUint16 {
			return fmt.Errorf("value out of range")
		}
		*(*uint16)(ptr) = uint16(uv)
	case reflect.Uint32:
		if uv > math.MaxUint32 {
			return fmt.Errorf("value out of range")
		}
		*(*uint32)(ptr) = uint32(uv)
	case reflect.Uint64:
		*(*uint64)(ptr) = uv
	default:
		return fmt.Errorf("unsupported unsigned integer kind %s", kind)
	}
	return nil
}

func setFloatValue(ptr unsafe.Pointer, kind reflect.Kind, v float64) error {
	switch kind {
	case reflect.Float32:
		if v > math.MaxFloat32 || v < -math.MaxFloat32 {
			return fmt.Errorf("value out of range")
		}
		*(*float32)(ptr) = float32(v)
	case reflect.Float64:
		*(*float64)(ptr) = v
	default:
		return fmt.Errorf("unsupported float kind %s", kind)
	}
	return nil
}

func (dest *scanner) Scan(src interface{}) error {
	var (
		st = reflect2.TypeOf(src)
		dt = dest.Type
	)

	// NULL值
	if src == nil || st.UnsafeIsNil(reflect2.PtrOf(src)) {
		// 设置成默认值，如果是指针，那么是空指针
		dt.UnsafeSet(dest.Val, dt.UnsafeNew())
		return nil
	}

	var (
		sk = st.Kind()
		dk = dt.Kind()
	)

	// 目标是指针类型（例如 *int, *string, *time.Time）
	// 对指针字段的支持逻辑：
	//   - 源为 NULL 时上面的分支已经将指针设为 nil
	//   - 源非 NULL 时，为指针分配一个新的底层值，并复用现有的转换逻辑
	if dk == reflect.Ptr {
		// 底层元素类型
		elemType := dt.(reflect2.PtrType).Elem()

		// 为底层元素分配空间
		elemPtr := elemType.UnsafeNew()

		// 使用一个临时 scanner 复用现有的转换逻辑
		inner := scanner{
			Type: elemType,
			Val:  elemPtr,
		}

		if err := inner.Scan(src); err != nil {
			return err
		}

		// 直接写入指针值，dest.Val 指向字段本身（类型为 *T）
		*(*unsafe.Pointer)(dest.Val) = elemPtr
		return nil
	}

	// 相同类型，直接赋值
	if dk == sk {
		dt.UnsafeSet(dest.Val, reflect2.PtrOf(src))
		return nil
	}

	isTime := dt.String() == "time.Time"
	// int64 => time.Time
	if sk == reflect.Int64 && isTime {
		*(*time.Time)(dest.Val) = time.Unix(src.(int64), 0).UTC()
		return nil
	}

	if sk == reflect.String {
		return scanFromString(isTime, st, dt, dest.Val, src.(string))
	} else if sk == reflect.Slice && st.(reflect2.SliceType).Elem().Kind() == reflect.Uint8 {
		return scanFromString(isTime, st, dt, dest.Val, string(src.([]byte)))
	} else if st.String() == "time.Time" {
		if dk == reflect.String {
			return dest.Scan(src.(time.Time).UTC().Format(_timeLayout))
		}
		return dest.Scan(src.(time.Time).UTC().Unix())
	}

	switch dk {
	case reflect.Bool:
		switch sk {
		case reflect.Int64:
			*(*bool)(dest.Val) = (src.(int64) != 0)
		case reflect.Float64:
			*(*bool)(dest.Val) = (src.(float64) != 0)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		switch sk {
		case reflect.Bool:
			if src.(bool) {
				return setSignedIntegerValue(dest.Val, dk, 1)
			}
			return setSignedIntegerValue(dest.Val, dk, 0)
		case reflect.Int64:
			if err := setSignedIntegerValue(dest.Val, dk, src.(int64)); err != nil {
				return fmt.Errorf("converting driver.Value type %s (%v) to a %s: %v", st.String(), src, dk, err)
			}
			return nil
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		switch sk {
		case reflect.Bool:
			if src.(bool) {
				return setUnsignedIntegerValue(dest.Val, dk, 1)
			}
			return setUnsignedIntegerValue(dest.Val, dk, 0)
		case reflect.Int64:
			if err := setUnsignedIntegerValue(dest.Val, dk, src.(int64)); err != nil {
				return fmt.Errorf("converting driver.Value type %s (%v) to a %s: %v", st.String(), src, dk, err)
			}
			return nil
		}
	case reflect.Float32, reflect.Float64:
		switch sk {
		case reflect.Bool:
			if src.(bool) {
				return setFloatValue(dest.Val, dk, 1)
			}
			return setFloatValue(dest.Val, dk, 0)
		case reflect.Int64:
			if err := setFloatValue(dest.Val, dk, float64(src.(int64))); err != nil {
				return fmt.Errorf("converting driver.Value type %s (%v) to a %s: %v", st.String(), src, dk, err)
			}
			return nil
		case reflect.Float64:
			if err := setFloatValue(dest.Val, dk, src.(float64)); err != nil {
				return fmt.Errorf("converting driver.Value type %s (%v) to a %s: %v", st.String(), src, dk, err)
			}
			return nil
		}
	case reflect.String:
		*(*string)(dest.Val) = numberToString(sk, src)
		return nil
	default:
		// number => []byte
		if dk == reflect.Slice && dt.(reflect2.SliceType).Elem().Kind() == reflect.Uint8 {
			*(*[]byte)(dest.Val) = reflect2.UnsafeCastString(numberToString(sk, src))
			return nil
		}
		return scanFromString(isTime, st, dt, dest.Val, fmt.Sprint(src))
	}
	return nil
}

type ormCond struct {
	Field string
	Op    string
	Args  []interface{}
	SQL   string
}

func (c *ormCond) Type() int {
	return _cond
}

func (c *ormCond) ShapeKeyPart() string {
	return buildItemShapePart(_cond, c.SQL)
}

func (c *ormCond) BuildSQL(sb *strings.Builder) {
	if c.SQL != "" {
		sb.WriteString(c.SQL)
		return
	}
	if c.Field != "" {
		fieldEscape(sb, c.Field)
	}
	sb.WriteString(c.Op)
}

func (c *ormCond) BuildArgs(stmtArgs *[]interface{}) {
	*stmtArgs = append(*stmtArgs, c.Args...)
}

// ormExpr is a custom SQL condition fragment with args.
// It is designed to be used inside Where/And/Or trees.
type ormExpr struct {
	SQL  string
	Args []interface{}
}

func (e *ormExpr) ShapeKeyPart() string {
	return buildItemShapePart(_cond, e.SQL)
}

func (e *ormExpr) BuildSQL(sb *strings.Builder) { sb.WriteString(e.SQL) }
func (e *ormExpr) BuildArgs(stmtArgs *[]interface{}) {
	*stmtArgs = append(*stmtArgs, e.Args...)
}

type ormCondEx struct {
	Ty    int
	Conds []interface{}
	SQL   string
}

func (cx *ormCondEx) Type() int {
	return cx.Ty
}

func (cx *ormCondEx) ShapeKeyPart() string {
	return buildItemShapePart(cx.Ty, cx.SQL)
}

func (cx *ormCondEx) BuildSQL(sb *strings.Builder) {
	if cx.SQL != "" {
		sb.WriteString(cx.SQL)
		return
	}
	for i, c := range cx.Conds {
		if i > 0 {
			switch cx.Ty {
			case _andCondEx:
				sb.WriteString(" and ")
			case _orCondEx:
				sb.WriteString(" or ")
			}
		}
		if cond, ok := c.(*ormCond); ok {
			cond.BuildSQL(sb)
		} else if expr, ok := c.(*ormExpr); ok {
			expr.BuildSQL(sb)
		} else if condEx, ok := c.(*ormCondEx); ok {
			if len(condEx.Conds) > 1 && len(cx.Conds) > 1 {
				sb.WriteString("(")
			}
			condEx.BuildSQL(sb)
			if len(condEx.Conds) > 1 && len(cx.Conds) > 1 {
				sb.WriteString(")")
			}
		}
	}
}

func (cx *ormCondEx) BuildArgs(stmtArgs *[]interface{}) {
	for _, c := range cx.Conds {
		if cond, ok := c.(*ormCond); ok {
			*stmtArgs = append(*stmtArgs, cond.Args...)
		} else if expr, ok := c.(*ormExpr); ok {
			expr.BuildArgs(stmtArgs)
		} else if condEx, ok := c.(*ormCondEx); ok {
			condEx.BuildArgs(stmtArgs)
		}
	}
}

/*
   条件逻辑运算
*/

// And .
func And(conds ...interface{}) *ormCondEx {
	return &ormCondEx{Ty: _andCondEx, Conds: conds, SQL: buildCondExSQL(_andCondEx, conds)}
}

// Or .
func Or(conds ...interface{}) *ormCondEx {
	return &ormCondEx{Ty: _orCondEx, Conds: conds, SQL: buildCondExSQL(_orCondEx, conds)}
}

// Cond .
func Cond(c string, args ...interface{}) *ormCond {
	return &ormCond{Op: c, Args: args, SQL: c}
}

// Expr allows building a custom SQL condition fragment with args, and is usable inside And/Or.
// Example:
//
//	Where(And(Eq("a", 1), Expr("json_extract(data,'$.x') = ?", 2)))
func Expr(sql string, args ...interface{}) *ormExpr {
	return &ormExpr{SQL: sql, Args: args}
}

// Eq .
func Eq(field string, i interface{}) *ormCond {
	return &ormCond{Field: field, Op: "=?", Args: []interface{}{i}, SQL: buildCondSQL(field, "=?")}
}

// Neq .
func Neq(field string, i interface{}) *ormCond {
	return &ormCond{Field: field, Op: "<>?", Args: []interface{}{i}, SQL: buildCondSQL(field, "<>?")}
}

// Gt .
func Gt(field string, i interface{}) *ormCond {
	return &ormCond{Field: field, Op: ">?", Args: []interface{}{i}, SQL: buildCondSQL(field, ">?")}
}

// Gte .
func Gte(field string, i interface{}) *ormCond {
	return &ormCond{Field: field, Op: ">=?", Args: []interface{}{i}, SQL: buildCondSQL(field, ">=?")}
}

// Lt .
func Lt(field string, i interface{}) *ormCond {
	return &ormCond{Field: field, Op: "<?", Args: []interface{}{i}, SQL: buildCondSQL(field, "<?")}
}

// Lte .
func Lte(field string, i interface{}) *ormCond {
	return &ormCond{Field: field, Op: "<=?", Args: []interface{}{i}, SQL: buildCondSQL(field, "<=?")}
}

// Between .
func Between(field string, i interface{}, j interface{}) *ormCond {
	return &ormCond{Field: field, Op: " between ? and ?", Args: []interface{}{i, j}, SQL: buildCondSQL(field, " between ? and ?")}
}

// Like .
func Like(field string, pattern string) *ormCond {
	return &ormCond{Field: field, Op: " like ?", Args: []interface{}{pattern}, SQL: buildCondSQL(field, " like ?")}
}

// GLOB .
func GLOB(field string, pattern string) *ormCond {
	return &ormCond{Field: field, Op: " glob ?", Args: []interface{}{pattern}, SQL: buildCondSQL(field, " glob ?")}
}

// In .
func In(field string, args ...interface{}) *ormCond {
RETRY:
	switch len(args) {
	case 0:
		return &ormCond{Op: "1=1", SQL: "1=1"}
	case 1:
		rt := reflect2.TypeOf(args[0])
		// 如果第一个参数是数组，转化成interface数组
		if rt.Kind() == reflect.Slice {
			len := rt.(reflect2.SliceType).UnsafeLengthOf(reflect2.PtrOf(args[0]))
			argsAux := make([]interface{}, len)
			rtElem := rt.(reflect2.ListType).Elem()
			for i := 0; i < len; i++ {
				argsAux[i] = rtElem.PackEFace(rt.(reflect2.ListType).UnsafeGetIndex(reflect2.PtrOf(args[0]), i))
			}
			args = argsAux
			goto RETRY
		} else {
			// 即使只有一个元素也使用IN，保持SQL形状一致，防止缓存失效
			var sb strings.Builder
			sb.WriteString(" in (?)")
			return &ormCond{Field: field, Op: sb.String(), Args: []interface{}{args[0]}, SQL: buildCondSQL(field, sb.String())}
		}
	}

	var sb strings.Builder
	sb.WriteString(" in (")
	for i := 0; i < len(args); i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("?")
	}
	sb.WriteString(")")
	return &ormCond{Field: field, Op: sb.String(), Args: args, SQL: buildCondSQL(field, sb.String())}
}

// IsNot builds an "IS NOT" condition.
// If i is nil, it becomes "IS NOT NULL" (no args).
func IsNot(field string, i interface{}) *ormCond {
	if i == nil {
		return &ormCond{Field: field, Op: " is not null", SQL: buildCondSQL(field, " is not null")}
	}
	return &ormCond{Field: field, Op: " is not ?", Args: []interface{}{i}, SQL: buildCondSQL(field, " is not ?")}
}

// IsNotNull is a convenience for "IS NOT NULL".
func IsNotNull(field string) *ormCond {
	return &ormCond{Field: field, Op: " is not null", SQL: buildCondSQL(field, " is not null")}
}

// Is builds an "IS" condition.
// If i is nil, it becomes "IS NULL" (no args).
func Is(field string, i interface{}) *ormCond {
	if i == nil {
		return &ormCond{Field: field, Op: " is null", SQL: buildCondSQL(field, " is null")}
	}
	return &ormCond{Field: field, Op: " is ?", Args: []interface{}{i}, SQL: buildCondSQL(field, " is ?")}
}

// IsNull is a convenience for "IS NULL".
func IsNull(field string) *ormCond {
	return &ormCond{Field: field, Op: " is null", SQL: buildCondSQL(field, " is null")}
}

/*
	data-binding相关
*/

// DataBindingItem .

type DataBindingItem struct {
	SQL      string
	Cols     []interface{}
	Type     reflect2.Type
	Elem     interface{}
	Fields   []string // 用于Map类型的字段名
	SCols    []reflect2.StructField
	ArgIndex [][]int
}

type selectScanBinding struct {
	Cols     []interface{}
	Scanners []scanner
}

func bindSelectScanCols(item *DataBindingItem, elem interface{}) *selectScanBinding {
	n := len(item.SCols)
	if n == 0 {
		n = 1
	}
	binding := getSelectScanBinding(n)
	if len(item.SCols) > 0 {
		ptr := reflect2.PtrOf(elem)
		for i, field := range item.SCols {
			binding.Scanners[i] = scanner{Type: field.Type(), Val: field.UnsafeGet(ptr)}
			binding.Cols[i] = &binding.Scanners[i]
		}
		binding.Scanners = binding.Scanners[:len(item.SCols)]
		binding.Cols = binding.Cols[:len(item.SCols)]
		return binding
	}

	binding.Scanners[0] = scanner{Type: item.Type, Val: reflect2.PtrOf(elem)}
	binding.Cols[0] = &binding.Scanners[0]
	binding.Scanners = binding.Scanners[:1]
	binding.Cols = binding.Cols[:1]
	return binding
}

func getSelectScanBinding(n int) *selectScanBinding {
	binding := _selectScanBindingPool.Get().(*selectScanBinding)
	if cap(binding.Cols) < n {
		binding.Cols = make([]interface{}, n)
	} else {
		binding.Cols = binding.Cols[:n]
	}
	if cap(binding.Scanners) < n {
		binding.Scanners = make([]scanner, n)
	} else {
		binding.Scanners = binding.Scanners[:n]
	}
	return binding
}

func putSelectScanBinding(binding *selectScanBinding) {
	for i := range binding.Cols {
		binding.Cols[i] = nil
	}
	for i := range binding.Scanners {
		binding.Scanners[i] = scanner{}
	}
	_selectScanBindingPool.Put(binding)
}

type mapScanBinding struct {
	Values []interface{}
	Ptrs   []interface{}
}

func getMapScanBinding(n int) *mapScanBinding {
	binding := _mapScanBindingPool.Get().(*mapScanBinding)
	if cap(binding.Values) < n {
		binding.Values = make([]interface{}, n)
	} else {
		binding.Values = binding.Values[:n]
	}
	if cap(binding.Ptrs) < n {
		binding.Ptrs = make([]interface{}, n)
	} else {
		binding.Ptrs = binding.Ptrs[:n]
	}
	for i := 0; i < n; i++ {
		binding.Values[i] = nil
		binding.Ptrs[i] = &binding.Values[i]
	}
	return binding
}

func putMapScanBinding(binding *mapScanBinding) {
	for i := range binding.Values {
		binding.Values[i] = nil
	}
	for i := range binding.Ptrs {
		binding.Ptrs[i] = nil
	}
	_mapScanBindingPool.Put(binding)
}

func fillReflectMap(mapVal reflect.Value, fields []string, values []interface{}) {
	for i, field := range fields {
		setReflectMapValue(mapVal, field, values[i])
	}
}

func isStringAnyMapType(rt reflect2.Type) bool {
	if rt == nil || rt.Kind() != reflect.Map {
		return false
	}
	return rt.(reflect2.MapType).Type1() == mapStringAnyType
}

func buildStringAnyMap(fields []string, values []interface{}) map[string]interface{} {
	m := make(map[string]interface{}, len(fields))
	for i, field := range fields {
		m[field] = values[i]
	}
	return m
}

func buildStructArgIndexes(s reflect2.StructType, cols []reflect2.StructField) [][]int {
	if len(cols) == 0 {
		return nil
	}

	rt := s.Type1()
	if rt == nil {
		return nil
	}

	indexes := make([][]int, len(cols))
	for i, col := range cols {
		field, ok := rt.FieldByName(col.Name())
		if !ok {
			continue
		}
		index := make([]int, len(field.Index))
		copy(index, field.Index)
		indexes[i] = index
	}
	return indexes
}

type dataBindingCacheScope struct {
	Table  string
	Model  reflect.Type
	DB     interface{}
	Config Config
	Mode   string
	Rows   int
}

type dataBindingCache struct {
	sync.Map
	scope dataBindingCacheScope
}

func (t *ZormTable) getDataBindingCache(callSite *cachedCallSite, model reflect.Type, mode string, rows int) *dataBindingCache {
	scope := dataBindingCacheScope{Table: t.Name, Model: model, DB: t.DB, Config: t.Cfg, Mode: mode, Rows: rows}
	// Cached scopes contain comparable DB identities, so comparing a raw DB value is safe.
	if cache := t.lastBindingCache.Load(); cache != nil && cache.scope == scope {
		return cache
	}
	hint := callSite.bindingCache.Load()
	if hint != nil && hint.scope == scope {
		t.lastBindingCache.Store(hint)
		return hint
	}
	if _, ok := stmtCacheWrapperKey(t.DB); !ok {
		scope.DB = t
	}
	cached, ok := _dataBindingCaches.Load(scope)
	if !ok {
		cached, _ = _dataBindingCaches.LoadOrStore(scope, &dataBindingCache{scope: scope})
	}
	cache := cached.(*dataBindingCache)
	t.lastBindingCache.Store(cache)
	// Pin the first scope so mixed-scope callers do not keep rewriting a shared hint.
	if hint == nil {
		callSite.bindingCache.CompareAndSwap(nil, cache)
	}
	return cache
}

var _dataBindingCaches sync.Map

/*
Mock相关
*/
var (
	_mockData []*MockMatcher
	_mutex    sync.Mutex
)

func matchString(src string, matcher string, caseSens bool) bool {
	if matcher == "" {
		return true
	}

	isAlpha := func(x byte) bool {
		return x >= 0x61 && x <= 0x7A
	}

	caseSensEq := func(x byte, y byte, cs bool) bool {
		return (y == '?' || x == y || (!cs && isAlpha(x|0x20) && (x^y) == 0x20))
	}

	s := make([][]int, 0)
	i := 0
	j := 0
	ml := len(matcher)
	// scan from start
	for i < len(src) {
		// match asterisk
		if j < ml && matcher[j] == '*' {
			// skip continuous asterisks
			for j < ml && matcher[j] == '*' {
				j++
			}
			// forward to first match char of src
			for i < len(src) && (j >= ml || !caseSensEq(src[i], matcher[j], caseSens)) {
				i++
			}
			// record current position for back-track
			s = append(s, []int{i + 1, j - 1})
		} else if j < ml && caseSensEq(src[i], matcher[j], caseSens) {
			// eat one character
			i++
			j++
		} else {
			// hit mismatch, then back-track
			if len(s) <= 0 {
				return false
			}
			i, j = s[0][0], s[0][1]
			s = s[1:]
		}
	}
	// ignore ending asterisks
	for j < ml && matcher[j] == '*' {
		j++
	}
	return i == len(src) && j == ml
}

// MockMatcher .
type MockMatcher struct {
	Tbl    string
	Func   string
	Caller string
	File   string
	Pkg    string
	Data   interface{}
	Ret    int
	Err    error
}

func checkMock(tbl, fun, caller, file, pkg string) (mocked bool, data interface{}, ret int, err error) {
	_mutex.Lock()
	defer _mutex.Unlock()

	for i := 0; i < len(_mockData); i++ {
		data := _mockData[i]
		if matchString(tbl, data.Tbl, false) &&
			matchString(fun, data.Func, false) &&
			matchString(caller, data.Caller, false) &&
			matchString(file, data.File, false) &&
			matchString(pkg, data.Pkg, false) {
			_mockData = append(_mockData[0:i], _mockData[i+1:]...)
			return true, data.Data, data.Ret, data.Err
		}
	}
	return false, nil, 0, nil
}

func checkInTestFile(fileName string) {
	if !strings.HasSuffix(fileName, "_test.go") {
		panic("DONT USE THIS FUNCTION IN PRODUCTION ENVIRONMENT!")
	}
}

// ZormMock .
func ZormMock(tbl, fun, caller, file, pkg string, data interface{}, ret int, err error) {
	_, fileName, _, _ := runtime.Caller(1)
	checkInTestFile(fileName)

	config.Mock = true

	_mutex.Lock()
	defer _mutex.Unlock()

	_mockData = append(_mockData, &MockMatcher{
		Tbl:    tbl,
		Func:   fun,
		Caller: caller,
		File:   file,
		Pkg:    pkg,
		Data:   data,
		Ret:    ret,
		Err:    err,
	})
}

// ZormMockFinish .
func ZormMockFinish() error {
	_mutex.Lock()
	defer _mutex.Unlock()

	mockData := _mockData
	_mockData = make([]*MockMatcher, 0)
	if len(mockData) > 0 {
		return fmt.Errorf("Some of the mock data left behind: %+v", mockData)
	}
	return nil
}

// 优化后的缓存操作函数
func buildCacheKey(file string, line int) string {
	builder := _cacheKeyPool.Get().(*strings.Builder)
	builder.Reset()
	builder.WriteString(file)
	builder.WriteString(":")
	builder.WriteString(strconv.Itoa(line))
	key := builder.String()
	_cacheKeyPool.Put(builder)
	return key
}

func getCallSite() *cachedCallSite {
	var direct [1]uintptr
	if n := runtime.Callers(3, direct[:]); n == 1 && direct[0] != 0 {
		callerPC := direct[0] - 1
		if cached, ok := _callSiteCache.Load(callerPC); ok {
			return cached.(*cachedCallSite)
		}
		if fn := runtime.FuncForPC(callerPC); fn != nil && !strings.HasPrefix(fn.Name(), zormPackagePrefix) {
			file, line := fn.FileLine(callerPC)
			callSite := &cachedCallSite{CallSite: CallSite{File: file, Line: line, Key: buildCacheKey(file, line)}}
			_callSiteCache.Store(callerPC, callSite)
			return callSite
		}
	}

	var pcs [8]uintptr
	n := runtime.Callers(2, pcs[:])
	for i := 0; i < n; i++ {
		if pcs[i] == 0 {
			continue
		}
		callerPC := pcs[i] - 1
		if cached, ok := _callSiteCache.Load(callerPC); ok {
			return cached.(*cachedCallSite)
		}
		fn := runtime.FuncForPC(callerPC)
		if fn == nil || strings.HasPrefix(fn.Name(), zormPackagePrefix) {
			continue
		}
		file, line := fn.FileLine(callerPC)
		callSite := &cachedCallSite{CallSite: CallSite{File: file, Line: line, Key: buildCacheKey(file, line)}}
		_callSiteCache.Store(callerPC, callSite)
		return callSite
	}

	_, file, line, _ := runtime.Caller(1)
	return &cachedCallSite{CallSite: CallSite{File: file, Line: line, Key: buildCacheKey(file, line)}}
}

var (
	mapStringAnyType  = reflect.TypeOf(map[string]interface{}{})
	zormPackagePrefix = reflect.TypeOf(ZormTable{}).PkgPath() + "."
	_callSiteCache    sync.Map // map[uintptr]*cachedCallSite
	_cacheKeyPool     = sync.Pool{
		New: func() interface{} {
			return &strings.Builder{}
		},
	}
	// 字符串构建器池，用于SQL构建
	_sqlBuilderPool = sync.Pool{
		New: func() interface{} {
			return &strings.Builder{}
		},
	}
	// 参数切片池，减少slice分配
	_argsPool = sync.Pool{
		New: func() interface{} {
			return make([]interface{}, 0, 16)
		},
	}
	_selectScanBindingPool = sync.Pool{
		New: func() interface{} {
			return &selectScanBinding{}
		},
	}
	_mapScanBindingPool = sync.Pool{
		New: func() interface{} {
			return &mapScanBinding{}
		},
	}
	_shapeKeyBufferPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, 0, 128)
		},
	}
	_shapeKeyStringCache   sync.Map
	_stmtCacheWrapperCache sync.Map
	_structFieldCache      sync.Map // Field metadata depends only on the model type.
)

type CallSite struct {
	File string
	Line int
	Key  string
}

type cachedCallSite struct {
	CallSite
	bindingCache atomic.Pointer[dataBindingCache]
}

// getSQLBuilder 从池中获取字符串构建器
func getSQLBuilder() *strings.Builder {
	return _sqlBuilderPool.Get().(*strings.Builder)
}

// putSQLBuilder 将字符串构建器放回池中
func putSQLBuilder(sb *strings.Builder) {
	sb.Reset()
	_sqlBuilderPool.Put(sb)
}

// getArgsSlice 从池中获取参数切片
func getArgsSlice() []interface{} {
	return _argsPool.Get().([]interface{})
}

// putArgsSlice 将参数切片放回池中
func putArgsSlice(args []interface{}) {
	args = args[:0]
	_argsPool.Put(args)
}

// Begin 开始事务
func Begin(db ZormDBIFace) (ZormTxIFace, error) {
	if txDB, ok := db.(interface {
		Begin() (*sql.Tx, error)
	}); ok {
		tx, err := txDB.Begin()
		if err != nil {
			return nil, err
		}
		return &ZormTx{tx: tx}, nil
	}
	return nil, errors.New("database does not support transactions")
}

// BeginContext 带上下文开始事务
func BeginContext(ctx context.Context, db ZormDBIFace) (ZormTxIFace, error) {
	if txDB, ok := db.(interface {
		BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
	}); ok {
		tx, err := txDB.BeginTx(ctx, nil)
		if err != nil {
			return nil, err
		}
		return &ZormTx{tx: tx}, nil
	}
	return nil, errors.New("database does not support transactions")
}

// ZormTx 事务实现
type ZormTx struct {
	tx *sql.Tx
}

// QueryRowContext 实现 ZormDBIFace 接口
func (tx *ZormTx) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return tx.tx.QueryRowContext(ctx, query, args...)
}

// QueryContext 实现 ZormDBIFace 接口
func (tx *ZormTx) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return tx.tx.QueryContext(ctx, query, args...)
}

// ExecContext 实现 ZormDBIFace 接口
func (tx *ZormTx) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return tx.tx.ExecContext(ctx, query, args...)
}

// Commit 提交事务
func (tx *ZormTx) Commit() error {
	return tx.tx.Commit()
}

// Rollback 回滚事务
func (tx *ZormTx) Rollback() error {
	return tx.tx.Rollback()
}

// ConnectionPool 连接池配置
type ConnectionPool struct {
	MaxOpenConns    int           // 最大打开连接数
	MaxIdleConns    int           // 最大空闲连接数
	ConnMaxLifetime time.Duration // 连接最大生存时间
	ConnMaxIdleTime time.Duration // 连接最大空闲时间
}

// SetConnectionPool 设置连接池
func SetConnectionPool(db *sql.DB, pool *ConnectionPool) {
	if pool.MaxOpenConns > 0 {
		db.SetMaxOpenConns(pool.MaxOpenConns)
	}
	if pool.MaxIdleConns > 0 {
		db.SetMaxIdleConns(pool.MaxIdleConns)
	}
	if pool.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(pool.ConnMaxLifetime)
	}
	if pool.ConnMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(pool.ConnMaxIdleTime)
	}
}

// DefaultConnectionPool 默认连接池配置
func DefaultConnectionPool() *ConnectionPool {
	return &ConnectionPool{
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
	}
}

// ReadWriteDB 读写分离数据库
type ReadWriteDB struct {
	Master ZormDBIFace   // 主库（写）
	Slaves []ZormDBIFace // 从库（读）
	index  int64         // 轮询索引
}

// DDLConfig DDL configuration
type DDLConfig struct {
	SchemaManagement bool // Whether to enable schema management
}

// DefaultDDLConfig returns default DDL configuration for SQLite
func DefaultDDLConfig() *DDLConfig {
	return &DDLConfig{
		SchemaManagement: true,
	}
}

// NewReadWriteDB 创建读写分离数据库
func NewReadWriteDB(master ZormDBIFace, slaves ...ZormDBIFace) *ReadWriteDB {
	wrappedSlaves := make([]ZormDBIFace, len(slaves))
	for i, slave := range slaves {
		wrappedSlaves[i] = wrapDefaultStmtCacheDB(slave)
	}
	return &ReadWriteDB{
		Master: wrapDefaultStmtCacheDB(master),
		Slaves: wrappedSlaves,
		index:  0,
	}
}

func (rw *ReadWriteDB) nextReadDB() ZormDBIFace {
	if len(rw.Slaves) == 0 {
		return rw.Master
	}
	idx := atomic.AddInt64(&rw.index, 1) - 1
	return rw.Slaves[int(idx%int64(len(rw.Slaves)))]
}

// QueryRowContext 实现 ZormDBIFace 接口（读操作使用从库）
func (rw *ReadWriteDB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return rw.nextReadDB().QueryRowContext(ctx, query, args...)
}

// QueryContext 实现 ZormDBIFace 接口（读操作使用从库）
func (rw *ReadWriteDB) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return rw.nextReadDB().QueryContext(ctx, query, args...)
}

// ExecContext 实现 ZormDBIFace 接口（写操作使用主库）
func (rw *ReadWriteDB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return rw.Master.ExecContext(ctx, query, args...)
}

// CreateTable creates a table from struct definition
func CreateTable(db ZormDBIFace, tableName string, model interface{}, config *DDLConfig) error {
	if config == nil {
		config = DefaultDDLConfig()
	}

	sql, err := generateCreateTableSQL(tableName, model, config)
	if err != nil {
		return err
	}

	// Debug: print generated SQL (remove in production)
	// fmt.Printf("Generated SQL: %s\n", sql)

	_, err = db.ExecContext(context.Background(), sql)
	return err
}

// CreateTables automatically creates table schemas
func CreateTables(db ZormDBIFace, models ...interface{}) error {
	for _, model := range models {
		tableName := getTableName(model)
		if err := createTableFromModel(db, tableName, model); err != nil {
			return err
		}
	}
	return nil
}

// DropTable drops a table if it exists
func DropTable(db ZormDBIFace, tableName string) error {
	sql := fmt.Sprintf("DROP TABLE IF EXISTS `%s`", tableName)
	_, err := db.ExecContext(context.Background(), sql)
	return err
}

// TableExists checks if a table exists
// Priority: SQLite implementation first, MySQL as fallback
func TableExists(db ZormDBIFace, tableName string) (bool, error) {
	var count int

	// Try SQLite first (most common for zorm)
	err := db.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", tableName).Scan(&count)
	if err == nil {
		return count > 0, nil
	}

	// Fallback to MySQL/PostgreSQL
	err = db.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM information_schema.tables WHERE table_name = ?", tableName).Scan(&count)
	if err != nil {
		// Try PostgreSQL as second fallback
		err = db.QueryRowContext(context.Background(),
			"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1", tableName).Scan(&count)
	}

	return count > 0, err
}

// generateCreateTableSQL generates CREATE TABLE SQL from struct definition
func generateCreateTableSQL(tableName string, model interface{}, config *DDLConfig) (string, error) {
	rt := reflect2.TypeOf(model)
	if rt.Kind() == reflect.Ptr {
		rt = rt.(reflect2.PtrType).Elem()
	}

	if rt.Kind() != reflect.Struct {
		return "", errors.New("model must be a struct")
	}

	s := rt.(reflect2.StructType)
	sb := getSQLBuilder()
	defer putSQLBuilder(sb)

	sb.WriteString("CREATE TABLE IF NOT EXISTS `")
	sb.WriteString(tableName)
	sb.WriteString("` (")

	first := true
	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		ft := f.Tag().Get("zorm")

		// Skip fields with zorm tag "-"
		if ft == "-" {
			continue
		}

		if !first {
			sb.WriteString(",")
		}
		first = false

		// Field name
		fieldName := f.Name()
		if ft != "" {
			fieldName = strings.Split(ft, ",")[0]
		}
		sb.WriteString("\n  `")
		sb.WriteString(fieldName)
		sb.WriteString("` ")

		// Auto-increment primary key needs special handling
		if isAutoIncrementField(f) {
			sb.WriteString("INTEGER PRIMARY KEY AUTOINCREMENT")
		} else {
			// Field type
			fieldType := getSQLType(f.Type())
			sb.WriteString(fieldType)
		}

		// NOT NULL
		if !isNullable(f) {
			sb.WriteString(" NOT NULL")
		}

		// Default value (auto-increment fields don't need default values)
		if !isAutoIncrementField(f) {
			if defaultValue := getDefaultValue(f); defaultValue != "" {
				sb.WriteString(" DEFAULT ")
				sb.WriteString(defaultValue)
			}
		}
	}

	sb.WriteString("\n)")

	return sb.String(), nil
}

// getTableName extracts table name from struct type
func getTableName(model interface{}) string {
	rt := reflect2.TypeOf(model)
	if rt.Kind() == reflect.Ptr {
		rt = rt.(reflect2.PtrType).Elem()
	}

	// Try to get table name from struct name
	name := rt.String()
	if name == "" {
		return "table"
	}

	// Extract struct name from full type name
	parts := strings.Split(name, ".")
	if len(parts) > 0 {
		name = parts[len(parts)-1]
	}

	// Simple pluralization (can be extended)
	return strings.ToLower(name) + "s"
}

// createTableFromModel automatically creates a single table from model
func createTableFromModel(db ZormDBIFace, tableName string, model interface{}) error {
	// Check if table exists
	exists, err := TableExists(db, tableName)
	if err != nil {
		return err
	}

	if !exists {
		// Table doesn't exist, create it
		return CreateTable(db, tableName, model, DefaultDDLConfig())
	}

	// Table exists, check if fields need to be added
	// More complex schema management logic can be implemented here
	// Currently only does simple table creation
	return nil
}

// getSQLType maps Go types to SQL types
// Optimized for SQLite, compatible with MySQL/PostgreSQL
func getSQLType(rt reflect2.Type) string {
	switch rt.Kind() {
	case reflect.Bool:
		return "BOOLEAN"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		return "INTEGER"
	case reflect.Int64:
		return "BIGINT"
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		return "INTEGER UNSIGNED"
	case reflect.Uint64:
		return "BIGINT UNSIGNED"
	case reflect.Float32:
		return "FLOAT"
	case reflect.Float64:
		return "DOUBLE"
	case reflect.String:
		return "TEXT"
	case reflect.Slice:
		if rt.(reflect2.SliceType).Elem().Kind() == reflect.Uint8 {
			return "BLOB"
		}
		return "TEXT"
	default:
		if rt.String() == "time.Time" {
			return "DATETIME"
		}
		return "TEXT"
	}
}

// isNullable checks if a field can be null
func isNullable(f reflect2.StructField) bool {
	ft := f.Tag().Get("zorm")
	if ft == "" {
		return true
	}

	tags := strings.Split(ft, ",")
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "not_null" || tag == "notnull" {
			return false
		}
	}

	// Determine based on Go type
	switch f.Type().Kind() {
	case reflect.Ptr, reflect.Slice, reflect.Map, reflect.Interface:
		return true
	default:
		return false
	}
}

// getDefaultValue gets the default value for a field
func getDefaultValue(f reflect2.StructField) string {
	ft := f.Tag().Get("zorm")
	if ft == "" {
		return ""
	}

	tags := strings.Split(ft, ",")
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if strings.HasPrefix(tag, "default:") {
			return strings.TrimPrefix(tag, "default:")
		}
	}

	// Set default value based on type
	switch f.Type().Kind() {
	case reflect.String:
		return "'default'"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "0"
	case reflect.Float32, reflect.Float64:
		return "0.0"
	case reflect.Bool:
		return "0"
	default:
		if f.Type().String() == "time.Time" {
			return "CURRENT_TIMESTAMP"
		}
		return ""
	}
}
