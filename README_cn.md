
# zorm

[![license](https://img.shields.io/badge/license-MIT-brightgreen.svg?style=flat)](https://github.com/IceWhaleTech/zorm/blob/master/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/IceWhaleTech/zorm)](https://goreportcard.com/report/github.com/IceWhaleTech/zorm)
[![Build Status](https://orca-zhang.semaphoreci.com/badges/zorm/branches/main.svg?style=shields)](https://orca-zhang.semaphoreci.com/projects/zorm)
[![codecov](https://codecov.io/github/IceWhaleTech/zorm/branch/main/graph/badge.svg?token=QAXfxhPiur)](https://codecov.io/github/IceWhaleTech/zorm)

🏎️ 用Go开发的简单、超快、可自测试的Zima ORM

[English](README.md) | [中文](README_cn.md)

# 📊 [性能基准测试](https://github.com/benchplus/goorm)

<table>
<thead>
<tr>
<th>测试用例</th>
<th><a href="https://github.com/IceWhaleTech/zorm"><strong>ZORM</strong></a></th>
<th><a href="https://github.com/orca-zhang/borm"><strong>BORM</strong></a></th>
<th><a href="https://bun.uptrace.dev/"><strong>BUN</strong></a></th>
<th><a href="https://github.com/ent/ent"><strong>ENT</strong></a></th>
<th><a href="https://gorm.io/"><strong>GORM</strong></a></th>
<th><a href="https://github.com/jmoiron/sqlx"><strong>SQLX</strong></a></th>
<th><a href="https://xorm.io/"><strong>XORM</strong></a></th>
</tr>
</thead>
<tbody>
<tr><td>单条插入</td><td style="background-color: #4CAF50;">🟢 1x</td><td style="background-color: #4CAF50;">🟢 1x</td><td style="background-color: #FFA500;">🟠 3.13x</td><td style="background-color: #FFA500;">🟠 3.46x</td><td style="background-color: #FF6347;">🔴 7.09x</td><td style="background-color: #FF6347;">🔴 60.61x</td><td style="background-color: #FF6347;">🔴 61.12x</td></tr>
<tr><td>批量插入</td><td style="background-color: #4CAF50;">🟢 1x</td><td style="background-color: #4CAF50;">🟢 1x</td><td style="background-color: #FFC107;">🟡 1.30x</td><td style="background-color: #FFA500;">🟠 2.50x</td><td style="background-color: #FFC107;">🟡 1.89x</td><td style="background-color: #FFA500;">🟠 3.57x</td><td style="background-color: #FFA500;">🟠 3.33x</td></tr>
<tr><td>按ID查询</td><td style="background-color: #4CAF50;">🟢 1x</td><td style="background-color: #4CAF50;">🟢 1x</td><td style="background-color: #FFC107;">🟡 1.52x</td><td style="background-color: #FFC107;">🟡 1.85x</td><td style="background-color: #FFC107;">🟡 1.90x</td><td style="background-color: #FFA500;">🟠 2x</td><td style="background-color: #FFA500;">🟠 3.12x</td></tr>
<tr><td>按IDs查询</td><td style="background-color: #4CAF50;">🟢 1x</td><td style="background-color: #4CAF50;">🟢 1x</td><td style="background-color: #FFC107;">🟡 1.17x</td><td style="background-color: #FFC107;">🟡 1.38x</td><td style="background-color: #FFC107;">🟡 1.39x</td><td style="background-color: #FFC107;">🟡 1.36x</td><td style="background-color: #FFC107;">🟡 1.98x</td></tr>
<tr><td>更新</td><td style="background-color: #4CAF50;">🟢 1x</td><td style="background-color: #4CAF50;">🟢 1x</td><td style="background-color: #FFA500;">🟠 2.67x</td><td style="background-color: #FF6347;">🔴 9.86x</td><td style="background-color: #FF6347;">🔴 7.06x</td><td style="background-color: #FF6347;">🔴 82.52x</td><td style="background-color: #FF6347;">🔴 84x</td></tr>
<tr><td>删除</td><td style="background-color: #4CAF50;">🟢 1x</td><td style="background-color: #4CAF50;">🟢 1x</td><td style="background-color: #FFA500;">🟠 2.31x</td><td style="background-color: #FFA500;">🟠 2.62x</td><td style="background-color: #FF6347;">🔴 6.40x</td><td style="background-color: #FF6347;">🔴 105.84x</td><td style="background-color: #FF6347;">🔴 101.85x</td></tr>
<tr><td>计数</td><td style="background-color: #4CAF50;">🟢 1x</td><td style="background-color: #4CAF50;">🟢 1x</td><td style="background-color: #FFA500;">🟠 2.15x</td><td style="background-color: #FF6347;">🔴 13.40x</td><td style="background-color: #FFA500;">🟠 2.99x</td><td style="background-color: #FFA500;">🟠 4.34x</td><td style="background-color: #FF6347;">🔴 5.95x</td></tr>
<tr><td>查询全部</td><td style="background-color: #4CAF50;">🟢 1x</td><td style="background-color: #4CAF50;">🟢 1x</td><td style="background-color: #FFC107;">🟡 1.14x</td><td style="background-color: #FFC107;">🟡 1.21x</td><td style="background-color: #FFC107;">🟡 1.43x</td><td style="background-color: #FFC107;">🟡 1.18x</td><td style="background-color: #FFC107;">🟡 1.91x</td></tr>
</tbody>
</table>

> **性能倍数**：数值表示相对于最快 ORM 的性能倍数（数值越小越好）
>
> **⭐ Pareto 最优**：表示该 ORM 在该测试用例下 **同时更快且更省内存**（在 **ns/op** 与 **B/op** 这两个维度上为 Pareto 最优，数值越小越好）。⭐ 标记会出现在 **ns/op** 和 **B/op** 两列中。

# 🚀 核心特性

## ⚡ 高性能
- **8.6倍性能提升**：智能缓存与零分配设计
- **默认开启重用**：自动复用SQL和元数据，提升重复操作性能
- **连接池管理**：可配置连接池，为高并发场景提供最优默认值
- **读写分离**：自动路由读写操作，提升整体性能

## 🗺️ 智能数据类型与模式管理
- **Map支持**：无需定义struct，直接使用`map[string]interface{}`
- **自动命名**：驼峰命名自动转换为数据库蛇形命名
- **灵活标签**：支持`zorm:"field_name,auto_incr"`格式
- **原子DDL**：创建、修改、删除表的原子操作

## 🛠️ 完整CRUD操作与监控
- **一行操作**：简单的Insert、Update、Select、Delete API
- **事务支持**：内置事务管理，支持上下文
- **联表查询**：高级JOIN操作，灵活的ON条件
- **SQL审计**：完整的数据库操作审计日志

# 目标
- 易用：SQL-Like（一把梭：One-Line-CRUD）
- KISS：保持小而美（不做大而全）
- 通用：支持struct，map，pb和基本类型
- 可测：支持自mock（因为参数作返回值，大部分mock框架不支持）
    - 非测试向的library不是好library
- As-Is：尽可能不作隐藏设定，防止误用
- 解决核心痛点：
   - 手撸SQL难免有错，组装数据太花时间
   - time.Time无法直接读写的问题
   - SQL函数结果无法直接Scan
   - db操作无法方便的Mock
   - QueryRow的sql.ErrNoRows问题
   - **直接替换系统自带Scanner，完整接管数据读取的类型转换**
- 核心原则：
   - 别像使用其他orm那样把一个表映射到一个model
   - （在zorm里可以用Fields过滤器做到）
   - 尽量保持简单把一个操作映射一个model吧！
- 其他优点：
  - 更自然的where条件（仅在需要加括号时添加，对比gorm）
  - In操作接受各种类型slice
  - 从其他orm库切换无需修改历史代码，无侵入性修改

# 特性矩阵

#### 下面是和一些主流orm库的对比（请不吝开issue勘误）

<table style="text-align: center">
   <tr>
      <td colspan="2">库</td>
      <td><a href="https://github.com/IceWhaleTech/zorm">zorm <strong>(me)</strong></a></td>
      <td><a href="https://github.com/jinzhu/gorm">gorm</a></td>
      <td><a href="https://github.com/go-xorm/xorm">xorm</a></td>
      <td>备注</td>
   </tr>
   <tr>
      <td rowspan="7">易用性</td>
      <td>无需指定类型</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
      <td>:x:</td>
      <td>zorm在tag中无需低频的DDL</td>
   </tr>
   <tr>
      <td>无需指定model</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
      <td>:x:</td>
      <td>gorm/xorm改操作需提供“模版”</td>
   </tr>
   <tr>
      <td>无需指定主键</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
      <td>:x:</td>
      <td>gorm/xorm易误操作，如删/改全表</td>
   </tr>
   <tr>
      <td>学习成本低</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
      <td>:x:</td>
      <td>会SQL就会用zorm</td>
   </tr>
   <tr>
      <td>可复用原生连接</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
      <td>:x:</td>
      <td>zorm重构成本极小</td>
   </tr>
   <tr>
      <td>全类型转换</td>
      <td>:white_check_mark:</td>
      <td>maybe</td>
      <td>:x:</td>
      <td>杜绝类型转换的抛错</td>
   </tr>
   <tr>
      <td>复用查询命令</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
      <td>:x:</td>
      <td>zorm批量和单条使用同一个函数</td>
   </tr>
   <tr>
      <td>Map类型支持</td>
      <td>直接使用map操作数据库</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
      <td>:x:</td>
      <td>无需定义struct，灵活处理动态字段</td>
   </tr>
   <tr>
      <td>可测试性</td>
      <td>自mock</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
      <td>:x:</td>
      <td>zorm非常便于单元测试</td>
   </tr>
   <tr>
      <td rowspan="3">性能</td>
      <td>较原生耗时</td>
      <td><=1x</td>
      <td>2~3x</td>
      <td>2~3x</td>
      <td>xorm使用prepare模式会再慢2～3x</td>
   </tr>
   <tr>
      <td>反射</td>
      <td><a href="https://github.com/modern-go/reflect2">reflect2</a></td>
      <td>reflect</td>
      <td>reflect</td>
      <td>zorm零使用ValueOf</td>
   </tr>
   <tr>
      <td>缓存优化</td>
      <td>:rocket:</td>
      <td>:white_check_mark:</td>
      <td>:white_check_mark:</td>
      <td>8.6x性能提升，零分配设计，调用位置智能缓存</td>
   </tr>
</table>

# 快速入门

1. 引入包
   ``` golang
   import z "github.com/IceWhaleTech/zorm"
   ```

2. 定义Table对象
   ``` golang
   t := z.Table(d.DB, "t_usr")

   t1 := z.TableContext(ctx, d.DB, "t_usr")
   ```

- `d.DB`是支持Exec/Query/QueryRow的数据库连接对象
- `t_usr`可以是表名，或者是嵌套查询语句
- `ctx`是需要传递的Context对象，默认不传为context.Background()
- **重用功能默认开启**，提供2-14倍性能提升，无需额外配置

3. （可选）定义model对象
   ``` golang
   type Info struct {
      ID   int64  `zorm:"id,auto_incr"` // 自增主键
      Name string // 自动转换为"name"
      Tag  string // 自动转换为"tag"
   }
   ```

4. 执行操作

- **CRUD接口返回值为 (影响的条数，错误)**

- **类型`V`为`map[string]interface{}`的缩写形式，参考`gin.H`**

- 插入
   ``` golang
   // o可以是对象/slice/ptr slice
   n, err = t.Insert(&o)
   n, err = t.InsertIgnore(&o)
   n, err = t.ReplaceInto(&o)

   // 只插入部分字段（其他使用缺省）
   n, err = t.Insert(&o, z.Fields("name", "tag"))

   // 解决主键冲突（使用 excluded 值 - SQLite UPSERT 语法）
   // 这是 SQLite 的官方 UPSERT 语法，功能上等价于 MySQL 的 ON DUPLICATE KEY UPDATE
   // 示例: INSERT INTO users (id, name, age) VALUES (1, 'Alice', 18)
   //       ON CONFLICT(id) DO UPDATE SET name = excluded.name, age = excluded.age;
   n, err = t.Insert(&o, z.Fields("id", "name", "age"),
      z.OnConflictDoUpdateSet([]string{"id"}, []string{"name", "age"}))

   // 使用map插入（无需定义struct）
   userMap := map[string]interface{}{
      "name":  "John Doe",
      "email": "john@example.com",
      "age":   30,
   }
   n, err = t.Insert(userMap)

   // 支持embedded struct
   type User struct {
      Name  string `zorm:"name"`
      Email string `zorm:"email"`
      Address struct {
         Street string `zorm:"street"`
         City   string `zorm:"city"`
      } `zorm:"-"` // 嵌入结构体
   }
   n, err = t.Insert(&user)

   // 支持字段忽略
   type User struct {
      Name     string `zorm:"name"`
      Password string `zorm:"-"` // 忽略此字段
      Email    string `zorm:"email"`
   }
   n, err = t.Insert(&user)
   ```

- 查询
   ``` golang
   // o可以是对象/slice/ptr slice
   n, err := t.Select(&o, 
      z.Where("name = ?", name), 
      z.GroupBy("id"), 
      z.Having(z.Gt("id", 0)), 
      z.OrderBy("id", "name"), 
      z.Limit(1))

   // 使用基本类型+Fields获取条目数（n的值为1，因为结果只有1条）
   var cnt int64
   n, err = t.Select(&cnt, z.Fields("count(1)"), z.Where("name = ?", name))

   // 还可以支持数组
   var ids []int64
   n, err = t.Select(&ids, z.Fields("id"), z.Where("name = ?", name))

   // 可以强制索引
   n, err = t.Select(&ids, z.Fields("id"), z.IndexedBy("idx_xxx"), z.Where("name = ?", name))

   // 高级连接查询
   // 使用字符串 ON 条件的简单连接
   var results []UserOrder
   n, err := t.Select(&results,
      z.Fields("users.id", "users.name", "orders.amount"),
      z.InnerJoin("orders", "users.id = orders.user_id"),
      z.Where("orders.status = ?", "completed"),
   )

   // 使用条件对象的复杂连接
   n, err = t.Select(&results,
      z.Fields("users.id", "users.name", "orders.amount"),
      z.LeftJoin("orders", z.Eq("users.id", z.U("orders.user_id"))),
   )
   ```

- Select 到 Map（无需定义 struct）
  ``` golang
  // 单行映射到 map
  var m map[string]interface{}
  n, err := t.Select(&m, z.Fields("id", "name", "age"), z.Where(z.Eq("id", 1)))

  // 多行映射到 []map
  var ms []map[string]interface{}
  n, err = t.Select(&ms, z.Fields("id", "name", "age"), z.Where(z.Gt("age", 18)))
  ```

- 更新
   ``` golang
   // o可以是对象/slice/ptr slice
   n, err = t.Update(&o, z.Where(z.Eq("id", id)))

   // 使用map更新
   n, err = t.Update(z.V{
         "name": "new_name",
         "tag":  "tag1,tag2,tag3",
         "age":  z.U("age+1"), // 使用b.U来处理非变量更新
      }, z.Where(z.Eq("id", id)))

   // 使用map更新部分字段
   n, err = t.Update(z.V{
         "name": "new_name",
         "tag":  "tag1,tag2,tag3",
      }, z.Fields("name"), z.Where(z.Eq("id", id)))

   n, err = t.Update(&o, z.Fields("name"), z.Where(z.Eq("id", id)))
   ```

- CRUD 配合 重用（默认开启）
  ``` golang
  // 重用 默认开启；同一调用点重复调用会复用 SQL/元数据
  // Update 示例
  type User struct { ID int64 `zorm:"id"`; Name string `zorm:"name"`; Age int `zorm:"age"` }
  for _, u := range users {
      _, _ = t.Update(&u, z.Fields("name", "age"), z.Where(z.Eq("id", u.ID)))
  }

  // Insert 示例
  for _, u := range users {
      _, _ = t.Insert(&u)
  }
  ```

- 删除
   ``` golang
   // 根据条件删除
   n, err = t.Delete(z.Where("name = ?", name))
   n, err = t.Delete(z.Where(z.Eq("id", id)))
   ```

- 执行原生SQL
   ``` golang
   // 执行带参数的原生SQL
   n, err = t.Exec("UPDATE users SET status = ? WHERE id = ?", "active", 123)
   n, err = t.Exec("DELETE FROM logs WHERE created_at < ?", time.Now().AddDate(0, 0, -30))
   n, err = t.Exec("CREATE INDEX idx_name ON users (name)")
   ```

- **可变条件**
   ``` golang
   conds := []interface{}{z.Cond("1=1")} // 防止空where条件
   if name != "" {
      conds = append(conds, z.Eq("name", name))
   }
   if id > 0 {
      conds = append(conds, z.Eq("id", id))
   }
   // 执行查询操作
   n, err := t.Select(&o, z.Where(conds...))
   ```

- **联表查询**
   ``` golang
   type Info struct {
      ID   int64  `zorm:"t_usr.id"` // 字段定义加表名
      Name string `zorm:"t_usr.name"`
      Tag  string `zorm:"t_tag.tag"`
   }
   
   // 方法一
   t := z.Table(d.DB, "t_usr join t_tag on t_usr.id=t_tag.id") // 表名用join语句
   var o Info
   n, err := t.Select(&o, z.Where(z.Eq("t_usr.id", id))) // 条件加上表名

   // 方法二
   t = z.Table(d.DB, "t_usr") // 正常表名
   n, err = t.Select(&o, z.Join("join t_tag on t_usr.id=t_tag.id"), z.Where(z.Eq("t_usr.id", id))) // 条件需要加上表名
   ```

-  获取插入的自增id
   ``` golang
   // 现代方法：使用 auto_incr 标签
   type Info struct {
      ID   int64  `zorm:"id,auto_incr"` // 自增主键
      Name string `zorm:"name"`
      Age  int    `zorm:"age"`
   }

   o := Info{
      Name: "OrcaZ",
      Age:  30,
   }
   n, err = t.Insert(&o)

   id := o.ID // 自动获取插入的id
   ```

   **注意**：旧的 `ZormLastId` 字段仍然支持向后兼容，但推荐使用现代的 `auto_incr` 标签方法。

- **新功能示例：Map类型和Embedded Struct**
   ``` golang
   // 1. 使用map类型（无需定义struct）
   userMap := map[string]interface{}{
      "name":     "John Doe",
      "email":    "john@example.com",
      "age":      30,
      "created_at": time.Now(),
   }
   n, err := t.Insert(userMap)

   // 2. 支持embedded struct
   type Address struct {
      Street string `zorm:"street"`
      City   string `zorm:"city"`
      Zip    string `zorm:"zip"`
   }

   type User struct {
      ID      int64  `zorm:"id"`
      Name    string `zorm:"name"`
      Email   string `zorm:"email"`
      Address Address `zorm:"-"` // 嵌入结构体
      Password string `zorm:"-"` // 忽略字段
   }

   user := User{
      Name:  "Jane Doe",
      Email: "jane@example.com",
      Address: Address{
         Street: "123 Main St",
         City:   "New York",
         Zip:    "10001",
      },
      Password: "secret", // 此字段会被忽略
   }
   n, err := t.Insert(&user)

   // 3. 复杂嵌套结构
   type Profile struct {
      Bio     string `zorm:"bio"`
      Website string `zorm:"website"`
   }

   type UserWithProfile struct {
      ID      int64  `zorm:"id"`
      Name    string `zorm:"name"`
      Profile Profile `zorm:"-"` // 嵌套嵌入
   }
   ```
   
- 正在使用其他orm框架（新的接口先切过来吧）
   ``` golang
   // [gorm] db是一个*gorm.DB
   t := z.Table(db.DB(), "tbl")

   // [xorm] db是一个*xorm.EngineGroup
   t := z.Table(db.Master().DB().DB, "tbl")
   // or
   t := z.Table(db.Slave().DB().DB, "tbl")
   ```


# 其他细节

### Table的选项

|选项|说明|
|-|-|
|Debug|打印sql语句|
|Reuse|根据调用位置复用sql和存储方式（**默认开启**，提供2-14倍性能提升）。内建形状感知与多形状缓存|
|NoReuse|关闭Reuse功能（不推荐，会降低性能）|
|ToTimestamp|调用Insert时，使用时间戳，而非格式化字符串|
|Audit|启用SQL审计日志和性能监控|

选项使用示例：
   ``` golang
   n, err = t.Debug().Insert(&o)

   n, err = t.ToTimestamp().Insert(&o)
   
   // Reuse功能默认开启，无需手动调用
   // 如需关闭（不推荐），可调用：
   n, err = t.NoReuse().Insert(&o)

   // 启用审计日志
   n, err = t.Audit(auditLogger, telemetryCollector).Insert(&o)

   // 链式多个选项
   n, err = t.Debug().Audit(auditLogger, telemetryCollector).Insert(&o)

   // 使用链式方法启用审计
   userTable := zorm.Table(db, "users").Audit(nil, nil) // 使用默认日志记录器

   // 或使用自定义日志记录器
   auditLogger := zorm.NewJSONAuditLogger()
   telemetryCollector := zorm.NewDefaultTelemetryCollector()
   userTable := zorm.Table(db, "users").Audit(auditLogger, telemetryCollector)

   // 链式多个选项
   advancedTable := zorm.Table(db, "users").
      Debug().           // 启用调试模式
      Audit(nil, nil)    // 启用审计日志
   ```

### Where

|示例|说明|
|-|-|
|Where("id=? and name=?", id, name)|常规格式化版本|
|Where(Eq("id", id), Eq("name", name)...)|默认为and连接|
|Where(And(Eq("x", x), Eq("y", y), Or(Eq("x", x), Eq("y", y)...)...)...)|And & Or|

### 预置Where条件

|名称|示例|说明|
|-|-|-|
|逻辑与|And(...)|任意个参数，只接受下方的关系运算子|
|逻辑或|Or(...)|任意个参数，只接受下方的关系运算子|
|普通条件|Cond("id=?", id)|参数1为格式化字符串，后面跟占位参数|
|相等|Eq("id", id)|两个参数，id=?|
|不相等|Neq("id", id)|两个参数，id<>?|
|大于|Gt("id", id)|两个参数，id>?|
|大于等于|Gte("id", id)|两个参数，id>=?|
|小于|Lt("id", id)|两个参数，id<?|
|小于等于|Lte("id", id)|两个参数，id<=?|
|在...之间|Between("id", start, end)|三个参数，在start和end之间|
|近似|Like("name", "x%")|两个参数，name like "x%"|
|近似|GLOB("name", "?x*")|两个参数，name glob "?x*"|
|多值选择|In("id", ids)|两个参数，ids是基础类型的slice|

### GroupBy

|示例|说明|
|-|-|
|GroupBy("id", "name"...)|-|

### Having

|示例|说明|
|-|-|
|Having("id=? and name=?", id, name)|常规格式化版本|
|Having(Eq("id", id), Eq("name", name)...)|默认为and连接|
|Having(And(Eq("x", x), Eq("y", y), Or(Eq("x", x), Eq("y", y)...)...)...)|And & Or|

### OrderBy

|示例|说明|
|-|-|
|OrderBy("id desc", "name asc"...)|-|

### Limit

|示例|说明|
|-|-|
|Limit(1)|分页大小为1|
|Limit(3, 2)|分页大小为3，偏移位置为2 **（注意和MySQL的区别）**|

### OnConflictDoUpdateSet

|示例|说明|
|-|-|
|OnConflictDoUpdateSet([]string{"id"}, []string{"name", "age"})|SQLite UPSERT 语法，使用 excluded 值。功能上等价于 MySQL 的 ON DUPLICATE KEY UPDATE。使用 `excluded.` 前缀来引用冲突行的值。|

### Map类型支持

|示例|说明|
|-|-|
|Insert(map[string]interface{}{"name": "John", "age": 30})|使用map插入数据|
|支持所有CRUD操作|Select、Insert、Update、Delete都支持map|

### Embedded Struct支持

|示例|说明|
|-|-|
|struct内嵌其他struct|自动处理组合对象的字段|
|zorm:"-"标签|标记嵌入结构体|

### 字段忽略功能

|示例|说明|
|-|-|
|Password string `zorm:"-"`|忽略此字段，不参与数据库操作|
|适用于敏感字段|如密码、临时字段等|

### IndexedBy

|示例|说明|
|-|-|
|IndexedBy("idx_biz_id")|解决索引选择性差的问题|

# 如何mock

### mock步骤：
- 调用`ZormMock`指定需要mock的操作
- 使用`ZormMockFinish`检查是否命中mock

### 说明：

- 前五个参数分别为`tbl`, `fun`, `caller`, `file`, `pkg`
   - 设置为空默认为匹配
   - 支持通配符'?'和'*'，分别代表匹配一个字符和多个字符
   - 不区分大小写

      |参数|名称|说明|
      |-|-|-|
      |tbl|表名|数据库的表名|
      |fun|方法名|Select/Insert/Update/Delete|
      |caller|调用方方法名|需要带包名|
      |file|文件名|使用处所在文件路径|
      |pkg|包名|使用处所在的包名|

- 后三个参数分别为`返回的数据`，`返回的影响条数`和`错误`
- 只能在测试文件中使用

### 使用示例：

待测函数：

```golang
   package x

   func test(db *sql.DB) (X, int, error) {
      var o X
      tbl := z.Table(db, "tbl")
      n, err := tbl.Select(&o, z.Where("`id` >= ?", 1), z.Limit(100))
      return o, n, err
   }
```

在`x.test`方法中查询`tbl`的数据，我们需要mock数据库的操作

``` golang
   // 必须在_test.go里面设置mock
   // 注意调用方方法名需要带包名
   z.ZormMock("tbl", "Select", "*.test", "", "", &o, 1, nil)

   // 调用被测试函数
   o1, n1, err := test(db)

   So(err, ShouldBeNil)
   So(n1, ShouldEqual, 1)
   So(o1, ShouldResemble, o)

   // 检查是否全部命中
   err = z.ZormMockFinish()
   So(err, ShouldBeNil)
```

#### 性能监控
所有操作都会自动监控遥测数据：
- **持续时间跟踪**：测量操作执行时间
- **缓存命中率**：监控复用效果
- **内存使用**：跟踪分配模式
- **错误率**：监控操作成功/失败率

#### 带审计的DDL管理器
```go
// 创建带审计的DDL管理器
ddlManager := zorm.NewDDLManager(auditableDB, auditLogger)

// 带审计日志的表创建
err := ddlManager.CreateTables(ctx, &User{}, &Product{}, &Order{})

// 所有DDL操作都会自动审计
```

## 📚 文档

- **[性能报告](PERFORMANCE_REPORT_cn.md)** - 详细的性能基准测试和优化分析

# 性能测试结果

## 重用功能性能优化
- **基准测试结果**:
  - 单线程: 8.6x 性能提升
  - 并发场景: 最高14.2x 性能提升
  - 内存优化: 92% 内存使用减少
  - 分配优化: 75% 分配次数减少

- **技术实现**:
  - 调用位置缓存: 使用`runtime.Caller`缓存文件行号
  - 字符串池化: `sync.Pool`复用`strings.Builder`
  - 零分配设计: 避免重复的字符串构建和内存分配
  - 并发安全: `sync.Map`支持高并发访问

- **性能数据**:
  ```
  BenchmarkReuseOptimized-8    	 1000000	      1200 ns/op	     128 B/op	       2 allocs/op
  BenchmarkReuseOriginal-8     	  100000	     10320 ns/op	    1600 B/op	      15 allocs/op
  ```

## 贡献者

这个项目的存在要感谢所有做出贡献的人。

请给我们一个💖star💖来支持我们，谢谢。

并感谢我们所有的支持者！ 🙏
