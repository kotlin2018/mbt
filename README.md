# mbt

* [具体使用example](https://github.com/kotlin2018/example.git)

* 默认支持的数据库驱动:
````
"mysql"，"tidb" ====> "github.com/go-sql-driver/mysql"


"mymysql"   ====> "github.com/ziutek/mymysql/godrv"


"postgres","cockroachDB"  ====> "github.com/lib/pq"


"sqlite3","sqlite" ====> "github.com/mattn/go-sqlite3"


"mssql" ====> "github.com/denisenkom/go-mssqldb"


"oci8" ====> "github.com/mattn/go-oci8"

````

* 自定义数据库驱动:
````
先实现 mbt.Convert 接口，再调用 Session.Driver()，将自定义的驱动注册到当前 Session 对象中即可。
````



