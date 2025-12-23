
# zorm

[![license](https://img.shields.io/badge/license-MIT-brightgreen.svg?style=flat)](https://github.com/IceWhaleTech/zorm/blob/master/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/IceWhaleTech/zorm)](https://goreportcard.com/report/github.com/IceWhaleTech/zorm)
[![Build Status](https://orca-zhang.semaphoreci.com/badges/zorm/branches/master.svg?style=shields)](https://orca-zhang.semaphoreci.com/projects/zorm)

🏎️ Zima ORM library that is simple, ultra-fast and self-mockable for Go

[English](README.md) | [中文](README_cn.md)

# 🚀 Key Features

## ⚡ High Performance
- **8.6x Performance Improvement**: Smart caching with zero allocation design
- **Reuse Enabled by Default**: Automatic SQL and metadata reuse for repeated operations
- **Connection Pool Management**: Configurable pool with optimal defaults for high concurrency
- **Read-Write Separation**: Automatic routing of read/write operations for better performance

## 🗺️ Smart Data Types & Schema Management
- **Map Support**: Use `map[string]interface{}` without struct definitions
- **Auto Naming**: CamelCase to snake_case conversion for database fields
- **Flexible Tags**: Support `zorm:"field_name,auto_incr"` format
- **Atomic DDL**: Create, alter, and drop tables with atomic operations

## 🛠️ Complete CRUD Operations & Monitoring
- **One-Line Operations**: Simple Insert, Update, Select, Delete APIs
- **Transaction Support**: Built-in transaction management with context support
- **Join Queries**: Advanced JOIN operations with flexible ON conditions
- **SQL Audit**: Complete audit logging for all database operations


# Goals
- **Easy to use**: SQL-Like (One-Line-CRUD)
- **KISS**: Keep it small and beautiful (not big and comprehensive)
- **Universal**: Support struct, map, pb and basic types
- **Testable**: Support self-mock (because parameters as return values, most mock frameworks don't support)
    - A library that is not test-oriented is not a good library
- **As-Is**: Try not to make hidden settings to prevent misuse
- **Solve core pain points**:
   - Manual SQL is error-prone, data assembly takes too much time
   - time.Time cannot be read/written directly
   - SQL function results cannot be scanned directly
   - Database operations cannot be easily mocked
   - QueryRow's sql.ErrNoRows problem
   - **Directly replace the built-in Scanner, completely take over data reading type conversion**
- **Core principles**:
   - Don't map a table to a model like other ORMs
   - (In zorm, you can use Fields filter to achieve this)
   - Try to keep it simple, map one operation to one model!
- **Other advantages**:
  - More natural where conditions (only add parentheses when needed, compared to gorm)
  - In operation accepts various types of slices
  - Switching from other ORM libraries requires no historical code modification, non-invasive modification

# Feature Matrix

#### Below is a comparison with mainstream ORM libraries (please don't hesitate to open issues for corrections)

<table style="text-align: center">
   <tr>
      <td colspan="2">Library</td>
      <td><a href="https://github.com/orca-zhang/zorm">zorm <strong>(me)</strong></a></td>
      <td><a href="https://github.com/jinzhu/gorm">gorm</a></td>
      <td><a href="https://github.com/go-xorm/xorm">xorm</a></td>
      <td>Notes</td>
   </tr>
   <tr>
      <td rowspan="7">Usability</td>
      <td>No type specification needed</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
      <td>:x:</td>
      <td>zorm doesn't need low-frequency DDL in tags</td>
   </tr>
   <tr>
      <td>No model specification needed</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
      <td>:x:</td>
      <td>gorm/xorm modification operations need to provide "template"</td>
   </tr>
   <tr>
      <td>No primary key specification needed</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
      <td>:x:</td>
      <td>gorm/xorm prone to misoperation, such as deleting/updating entire table</td>
   </tr>
   <tr>
      <td>Low learning cost</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
      <td>:x:</td>
      <td>If you know SQL, you can use zorm</td>
   </tr>
   <tr>
      <td>Reuse native connections</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
      <td>:x:</td>
      <td>zorm has minimal refactoring cost</td>
   </tr>
   <tr>
      <td>Full type conversion</td>
      <td>:white_check_mark:</td>
      <td>maybe</td>
      <td>:x:</td>
      <td>Eliminate type conversion errors</td>
   </tr>
   <tr>
      <td>Reuse query commands</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
      <td>:x:</td>
      <td>zorm uses the same function for batch and single operations</td>
   </tr>
   <tr>
      <td>Map type support</td>
      <td>Operate database with map</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
      <td>:x:</td>
      <td>Without defining struct, flexible handling of dynamic fields</td>
   </tr>
   <tr>
      <td>Testability</td>
      <td>Self-mock</td>
      <td>:white_check_mark:</td>
      <td>:x:</td>
      <td>:x:</td>
      <td>zorm is very convenient for unit testing</td>
   </tr>
   <tr>
      <td rowspan="3">Performance</td>
      <td>Compared to native time</td>
      <td><=1x</td>
      <td>2~3x</td>
      <td>2~3x</td>
      <td>xorm using prepare mode will be 2~3x slower</td>
   </tr>
   <tr>
      <td>Reflection</td>
      <td><a href="https://github.com/modern-go/reflect2">reflect2</a></td>
      <td>reflect</td>
      <td>reflect</td>
      <td>zorm zero use of ValueOf</td>
   </tr>
   <tr>
      <td>Cache Optimization</td>
      <td>:rocket:</td>
      <td>:white_check_mark:</td>
      <td>:white_check_mark:</td>
      <td>8.6x performance improvement, zero allocation design, smart call-site caching</td>
   </tr>
</table>

# Quick Start

1. Import package
   ``` golang
   import z "github.com/IceWhaleTech/zorm"
   ```

2. Define Table object
   ``` golang
   t := z.Table(d.DB, "t_usr")

   t1 := z.TableContext(ctx, d.DB, "t_usr")
   ```

- `d.DB` is a database connection object that supports Exec/Query/QueryRow
- `t_usr` can be a table name or nested query statement
- `ctx` is the Context object to pass, defaults to context.Background() if not provided
- **Reuse functionality is enabled by default**, providing 2-14x performance improvement, no additional configuration needed

3. (Optional) Define model object
   ``` golang
   type Info struct {
      ID   int64  `zorm:"id,auto_incr"` // Auto-increment primary key
      Name string // Auto-converted to "name"
      Tag  string // Auto-converted to "tag"
   }
   ```

4. Execute operations

- **CRUD interfaces return (affected rows, error)**

- **Type `V` is an abbreviation for `map[string]interface{}`, similar to `gin.H`**

- Insert
   ``` golang
   // o can be object/slice/ptr slice
   n, err = t.Insert(&o)
   n, err = t.InsertIgnore(&o)
   n, err = t.ReplaceInto(&o)

   // Insert only partial fields (others use defaults)
   n, err = t.Insert(&o, z.Fields("name", "tag"))

   // Resolve primary key conflicts
   n, err = t.Insert(&o, z.Fields("name", "tag"),
      z.OnConflictDoUpdateSet([]string{"id"}, z.V{
         "name": "new_name",
         "age":  z.U("age+1"), // Use z.U to handle non-variable updates
      }))

   // Use map insert (no need to define struct)
   userMap := map[string]interface{}{
      "name":  "John Doe",
      "email": "john@example.com",
      "age":   30,
   }
   n, err = t.Insert(userMap)

   // Support embedded struct
   type User struct {
      Name  string `zorm:"name"`
      Email string `zorm:"email"`
      Address struct {
         Street string `zorm:"street"`
         City   string `zorm:"city"`
      } `zorm:"-"` // embedded struct
   }
   n, err = t.Insert(&user)

   // Support field ignore
   type User struct {
      Name     string `zorm:"name"`
      Password string `zorm:"-"` // ignore this field
      Email    string `zorm:"email"`
   }
   n, err = t.Insert(&user)

   // Auto-increment primary key
   type User struct {
      ID   int64  `zorm:"id,auto_incr"` // Auto-increment primary key
      Name string `zorm:"name"`
      Age  int    `zorm:"age"`
   }
   user := User{Name: "Alice", Age: 25}
   n, err = t.Insert(&user)
   // user.ID will be automatically set to the generated ID

   // Support both pointer and non-pointer types
   user := User{Name: "Bob", Age: 30}        // Non-pointer
   users := []User{{Name: "Charlie", Age: 35}} // Non-pointer slice
   n, err = t.Insert(user)   // Non-pointer struct
   n, err = t.Insert(&users) // Non-pointer slice
   ```

- Select
   ``` golang
   // o can be object/slice/ptr slice
   n, err := t.Select(&o,
      z.Where("name = ?", name),
      z.GroupBy("id"),
      z.Having(z.Gt("id", 0)),
      z.OrderBy("id", "name"),
      z.Limit(1))

   // Use basic type + Fields to get count (n value is 1, because result has only 1 row)
   var cnt int64
   n, err = t.Select(&cnt, z.Fields("count(1)"), z.Where("name = ?", name))

   // Also support arrays
   var ids []int64
   n, err = t.Select(&ids, z.Fields("id"), z.Where("name = ?", name))

   // Can force index
   n, err = t.Select(&ids, z.Fields("id"), z.IndexedBy("idx_xxx"), z.Where("name = ?", name))

   // Advanced Join Queries
   // Simple join with string ON condition
   var results []UserOrder
   n, err := t.Select(&results,
      z.Fields("users.id", "users.name", "orders.amount"),
      z.InnerJoin("orders", "users.id = orders.user_id"),
      z.Where("orders.status = ?", "completed"),
   )

   // Complex join with condition objects
   n, err = t.Select(&results,
      z.Fields("users.id", "users.name", "orders.amount"),
      z.LeftJoin("orders", z.Eq("users.id", z.U("orders.user_id"))),
   )
   ```

- Select to Map (no struct needed)
  ``` golang
  // single row to map
  var m map[string]interface{}
  n, err := t.Select(&m, z.Fields("id", "name", "age"), z.Where(z.Eq("id", 1)))

  // multiple rows to []map
  var ms []map[string]interface{}
  n, err = t.Select(&ms, z.Fields("id", "name", "age"), z.Where(z.Gt("age", 18)))
  ```

- Update
   ``` golang
   // o can be object/slice/ptr slice
   n, err = t.Update(&o, z.Where(z.Eq("id", id)))

   // Use map update
   n, err = t.Update(z.V{
         "name": "new_name",
         "tag":  "tag1,tag2,tag3",
         "age":  z.U("age+1"), // Use z.U to handle non-variable updates
      }, z.Where(z.Eq("id", id)))

   // Use map update partial fields
   n, err = t.Update(z.V{
         "name": "new_name",
         "tag":  "tag1,tag2,tag3",
      }, z.Fields("name"), z.Where(z.Eq("id", id)))

   n, err = t.Update(&o, z.Fields("name"), z.Where(z.Eq("id", id)))
   ```

- CRUD with Reuse (enabled by default)
  ``` golang
  // Reuse is on by default; repeated calls at the same call-site reuse SQL/metadata
  // Update example
  type User struct { ID int64 `zorm:"id"`; Name string `zorm:"name"`; Age int `zorm:"age"` }
  for _, u := range users {
      _, _ = t.Update(&u, z.Fields("name", "age"), z.Where(z.Eq("id", u.ID)))
  }

  // Insert example
  for _, u := range users {
      _, _ = t.Insert(&u)
  }
  ```

- Delete
   ``` golang
   // Delete by condition
   n, err = t.Delete(z.Where("name = ?", name))
   n, err = t.Delete(z.Where(z.Eq("id", id)))
   ```

- Exec (Raw SQL)
   ``` golang
   // Execute raw SQL with parameters
   n, err = t.Exec("UPDATE users SET status = ? WHERE id = ?", "active", 123)
   n, err = t.Exec("DELETE FROM logs WHERE created_at < ?", time.Now().AddDate(0, 0, -30))
   n, err = t.Exec("CREATE INDEX idx_name ON users (name)")
   ```

- **Variable conditions**
   ``` golang
   conds := []interface{}{z.Cond("1=1")} // prevent empty where condition
   if name != "" {
      conds = append(conds, z.Eq("name", name))
   }
   if id > 0 {
      conds = append(conds, z.Eq("id", id))
   }
   // Execute query operation
   n, err := t.Select(&o, z.Where(conds...))
   ```

- **Join queries**
   ``` golang
   type Info struct {
      ID   int64  `zorm:"users.id"` // field definition with table name
      Name string `zorm:"users.name"`
      Tag  string `zorm:"tags.tag"`
   }

   t := z.Table(d.DB, "users")
   n, err := t.Select(&o, 
      z.InnerJoin("tags", "users.id = tags.user_id"),
      z.Where(z.Eq("users.id", id)))
   ```

- Get inserted auto-increment id
   ``` golang
   // Modern approach: Use auto_incr tag
   type Info struct {
      ID   int64  `zorm:"id,auto_incr"` // Auto-increment primary key
      Name string `zorm:"name"`
      Age  int    `zorm:"age"`
   }

   o := Info{
      Name: "OrcaZ",
      Age:  30,
   }
   n, err = t.Insert(&o)

   id := o.ID // get the inserted id automatically set
   ```

   **Note**: The old `ZormLastId` field is still supported for backward compatibility, but the modern `auto_incr` tag approach is recommended.

- **New features example: Map types and Embedded Struct**
   ``` golang
   // 1. Use map type (no need to define struct)
   userMap := map[string]interface{}{
      "name":     "John Doe",
      "email":    "john@example.com",
      "age":      30,
      "created_at": time.Now(),
   }
   n, err := t.Insert(userMap)

   // 2. Support embedded struct
   type Address struct {
      Street string `zorm:"street"`
      City   string `zorm:"city"`
      Zip    string `zorm:"zip"`
   }

   type User struct {
      ID      int64  `zorm:"id"`
      Name    string `zorm:"name"`
      Email   string `zorm:"email"`
      Address Address `zorm:"-"` // embedded struct
      Password string `zorm:"-"` // ignore field
   }

   user := User{
      Name:  "Jane Doe",
      Email: "jane@example.com",
      Address: Address{
         Street: "123 Main St",
         City:   "New York",
         Zip:    "10001",
      },
      Password: "secret", // this field will be ignored
   }
   n, err := t.Insert(&user)

   // 3. Complex nested structure
   type Profile struct {
      Bio     string `zorm:"bio"`
      Website string `zorm:"website"`
   }

   type UserWithProfile struct {
      ID      int64  `zorm:"id"`
      Name    string `zorm:"name"`
      Profile Profile `zorm:"-"` // nested embedding
   }
   ```

- Currently using other ORM frameworks (new interfaces can be switched first)
   ``` golang
   // [gorm] db is a *gorm.DB
   t := z.Table(db.DB(), "tbl")

   // [xorm] db is a *xorm.EngineGroup
   t := z.Table(db.Master().DB().DB, "tbl")
   // or
   t := z.Table(db.Slave().DB().DB, "tbl")
   ```


# Other Details

### Table Options

| Option      | Description                                                                                                                         |
|-------------|-------------------------------------------------------------------------------------------------------------------------------------|
| Debug       | Print SQL statements                                                                                                                |
| Reuse       | Reuse SQL and storage based on call location (**enabled by default**, 2-14x improvement). Shape-aware multi-shape cache is built-in |
| NoReuse     | Disable Reuse functionality (not recommended, will reduce performance)                                                              |
| ToTimestamp | Use timestamp for Insert, not formatted string                                                                                      |
| Audit       | Enable SQL audit logging and performance monitoring                                                                                 |

Option usage example:
   ``` golang
   n, err = t.Debug().Insert(&o)

   n, err = t.ToTimestamp().Insert(&o)

   // Reuse functionality is enabled by default, no manual call needed
   // If you need to disable it (not recommended), you can call:
   n, err = t.NoReuse().Insert(&o)

   // Enable audit with chain-style method
   userTable := zorm.Table(db, "users").Audit(nil, nil) // Uses default loggers

   // Or with custom loggers
   auditLogger := zorm.NewJSONAuditLogger()
   telemetryCollector := zorm.NewDefaultTelemetryCollector()
   userTable := zorm.Table(db, "users").Audit(auditLogger, telemetryCollector)

   // Chain multiple options
   advancedTable := zorm.Table(db, "users").
      Debug().           // Enable debug mode
      Audit(nil, nil)    // Enable audit logging
   ```

### Where

| Example                                                             | Description               |
|---------------------------------------------------------------------|---------------------------|
| Where("id=? and name=?", id, name)                                  | Regular formatted version |
| Where(Eq("id", id), Eq("name", name)...)                            | Default to and connection |
| Where(And(Eq("x", x), Eq("y", y), Or(Eq("x", x), Eq("y", y)...)...) | And & Or                  |

### Predefined Where Conditions

| Name                     | Example                   | Description                                                         |
|--------------------------|---------------------------|---------------------------------------------------------------------|
| Logical AND              | And(...)                  | Any number of parameters, only accepts relational operators below   |
| Logical OR               | Or(...)                   | Any number of parameters, only accepts relational operators below   |
| Normal condition         | Cond("id=?", id)          | Parameter 1 is formatted string, followed by placeholder parameters |
| Equal                    | Eq("id", id)              | Two parameters, id=?                                                |
| Not equal                | Neq("id", id)             | Two parameters, id<>?                                               |
| Greater than             | Gt("id", id)              | Two parameters, id>?                                                |
| Greater than or equal    | Gte("id", id)             | Two parameters, id>=?                                               |
| Less than                | Lt("id", id)              | Two parameters, id<?                                                |
| Less than or equal       | Lte("id", id)             | Two parameters, id<=?                                               |
| Between                  | Between("id", start, end) | Three parameters, between start and end                             |
| Like                     | Like("name", "x%")        | Two parameters, name like "x%"                                      |
| GLOB                     | GLOB("name", "?x*")       | Two parameters, name glob "?x*"                                     |
| Multiple value selection | In("id", ids)             | Two parameters, ids is basic type slice                             |

### GroupBy

| Example                  | Description |
|--------------------------|-------------|
| GroupBy("id", "name"...) | -           |

### Having

| Example                                                              | Description               |
|----------------------------------------------------------------------|---------------------------|
| Having("id=? and name=?", id, name)                                  | Regular formatted version |
| Having(Eq("id", id), Eq("name", name)...)                            | Default to and connection |
| Having(And(Eq("x", x), Eq("y", y), Or(Eq("x", x), Eq("y", y)...)...) | And & Or                  |

### OrderBy

| Example                           | Description |
|-----------------------------------|-------------|
| OrderBy("id desc", "name asc"...) | -           |

### Limit

| Example     | Description                                                           |
|-------------|-----------------------------------------------------------------------|
| Limit(1)    | Page size 1                                                           |
| Limit(3, 2) | Page size 3, offset position 2 **（Note the difference from MySQL）** |

### OnConflictDoUpdateSet

| Example                                                 | Description                             |
|---------------------------------------------------------|-----------------------------------------|
| OnConflictDoUpdateSet([]string{"id"}, V{"name": "new"}) | Update to resolve primary key conflicts |

### Map Type Support

| Example                                                   | Description                                    |
|-----------------------------------------------------------|------------------------------------------------|
| Insert(map[string]interface{}{"name": "John", "age": 30}) | Use map to insert data                         |
| Support all CRUD operations                               | Select, Insert, Update, Delete all support map |

### Embedded Struct Support

| Example                    | Description                                  |
|----------------------------|----------------------------------------------|
| struct embeds other struct | Automatically handle composite object fields |
| zorm:"-" tag               | Mark embedded struct                         |

### Field Ignore Functionality

| Example                       | Description                                               |
|-------------------------------|-----------------------------------------------------------|
| Password string `zorm:"-"`    | Ignore this field, not participate in database operations |
| Suitable for sensitive fields | Such as passwords, temporary fields, etc.                 |

### IndexedBy

| Example                 | Description                    |
|-------------------------|--------------------------------|
| IndexedBy("idx_biz_id") | Solve index selectivity issues |

# How to Mock

### Mock steps:
- Call `ZormMock` to specify operations to mock
- Use `ZormMockFinish` to check if mock was hit

### Description:

- First five parameters are `tbl`, `fun`, `caller`, `file`, `pkg`
   - Set to empty for default matching
   - Support wildcards '?' and '*', representing match one character and multiple characters respectively
   - Case insensitive

      | Parameter | Name               | Description                  |
      |-----------|--------------------|------------------------------|
      | tbl       | Table name         | Database table name          |
      | fun       | Method name        | Select/Insert/Update/Delete  |
      | caller    | Caller method name | Need to include package name |
      | file      | File name          | File path where used         |
      | pkg       | Package name       | Package name where used      |

- Last three parameters are `return data`, `return affected rows` and `error`
- Can only be used in test files

### Usage example:

Function to test:

```golang
   package x

   func test(db *sql.DB) (X, int, error) {
      var o X
      tbl := z.Table(db, "tbl")
      n, err := tbl.Select(&o, z.Where("`id` >= ?", 1), z.Limit(100))
      return o, n, err
   }
```

In the `x.test` method querying `tbl` data, we need to mock database operations

``` golang
   // Must set mock in _test.go file
   // Note caller method name needs to include package name
   z.ZormMock("tbl", "Select", "*.test", "", "", &o, 1, nil)

   // Call the function under test
   o1, n1, err := test(db)

   So(err, ShouldBeNil)
   So(n1, ShouldEqual, 1)
   So(o1, ShouldResemble, o)

   // Check if all hits
   err = z.ZormMockFinish()
   So(err, ShouldBeNil)
```

#### Performance Monitoring
All operations are automatically monitored with telemetry data:
- **Duration tracking**: Measure operation execution time
- **Cache hit rates**: Monitor reuse effectiveness  
- **Memory usage**: Track allocation patterns
- **Error rates**: Monitor operation success/failure rates

**Supported struct tags:**
- `zorm:"field_name"` - Field name mapping
- `zorm:"field_name,auto_incr"` - Auto-increment primary key
- `zorm:"auto_incr"` - Use converted field name with auto-increment
- `zorm:"-"` - Ignore field
- No tag - Auto-convert camelCase to snake_case

## 📚 Documentation

- **[Performance Report](PERFORMANCE_REPORT.md)** - Detailed performance benchmarks and optimization analysis

# Performance Test Results

- **8.6x Performance Improvement**: Smart caching with zero allocation design
- **Memory Optimization**: 92% memory usage reduction, 75% allocation count reduction
- **Concurrent Safe**: Optimized for high concurrency scenarios with `sync.Map`

## Reuse Function Performance Optimization
- **Benchmark Results**:
  - Single thread: 8.6x performance improvement
  - Concurrent scenarios: Up to 14.2x performance improvement
  - Memory optimization: 92% memory usage reduction
  - Allocation optimization: 75% allocation count reduction

- **Technical Implementation**:
  - Call site caching: Use `runtime.Caller` to cache file line numbers
  - String pooling: `sync.Pool` reuses `strings.Builder`
  - Zero allocation design: Avoid redundant string building and memory allocation
  - Concurrent safe: `sync.Map` supports high concurrency access

- **Performance Data**:
  ```
  BenchmarkReuseOptimized-8    	 1000000	      1200 ns/op	     128 B/op	       2 allocs/op
  BenchmarkReuseOriginal-8     	  100000	     10320 ns/op	    1600 B/op	      15 allocs/op
  ```

## Contributors

The existence of this project is thanks to all contributors.

Please give us a 💖star💖 to support us, thank you.

And thank you to all our supporters! 🙏
