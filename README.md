# mbt

# 1、Useage
* 1、
````go
package main

import (
	_ "github.com/go-sql-driver/mysql"
	"github.com/kotlin2018/mbt"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"io"
	"time"
)

var activity = ActivityDao{}

// 这是数据库中表 biz_activity 的实体类,注意 json 的 tag 就是表 biz_activity 的具体字段
type BizActivity struct {
	Id         string    `json:"id"`
	Name       string    `json:"name"`
	PcLink     string    `json:"pc_link"`
	H5Link     string    `json:"h5_link"`
	Remark     string    `json:"remark"`
	Sort       int       `json:"sort"`
	Status     int       `json:"status"`
	Version    int       `json:"version"`
	CreateTime time.Time `json:"create_time"`
	DeleteFlag int       `json:"delete_flag"`
	Salary     float64   `json:"salary"`
}

func main() {
	// 第一步: 初始化配置信息
	cfg := &mbt.Database{
		Pkg:        "./xml",                                                                                 // 生成的xml文件地址(即:xml文件放在哪个目录下)
		DriverName: "mysql",                                                                                 // 驱动名称
		DSN:        "root:root@tcp(127.0.0.1:3306)/test?charset=utf8mb4&parseTime=true&loc=Asia%2FShanghai", // 数据连接信息
		Logger: &mbt.Logger{
			PrintSql: true,           // 输出 sql 语句
			Path:     "/tmp",         // 日志文件的路径
			LinkName: "./rotate.log", // 最新的日志软连接名
			MaxAge:   180,            // 日志文件清理前的最长保存时间(单位分钟)
			Interval: 2,              // 日志分割的时间，隔多久分割一次(单位分钟)
		},
	}

	// 第二步: 创建一个 session 对象,一个 session 对象操作一个 SQL数据库。
	session := mbt.New(cfg)

	// 第三步: 配置日志输出（可选)
	session.SetOutPut(initLogger(cfg.Logger.Path, cfg.Logger.MaxAge, cfg.Logger.Interval))

	// 第四步: 将mapper结构体注册到服务中，并启动服务。
	session.Run(&activity)
}

// 具体的 增、删、改、查 方法
type ActivityDao struct {
	BizActivity BizActivity
	Insert func(arg BizActivity) int64
	Delete func(arg BizActivity) int64
	Update func(arg BizActivity) int64
	Select func(arg BizActivity) BizActivity
}

func initLogger (path string,maxAge,interval int)io.Writer{
	writer, _ := rotatelogs.New(
		path+".%Y%m%d%H%M",
		rotatelogs.WithLinkName(path),
		rotatelogs.WithMaxAge(time.Duration(maxAge)*time.Minute),
		rotatelogs.WithRotationTime(time.Duration(interval)*time.Minute),
	)
	return writer
}
````
* 2、运行 main 函数，则在当前项目根目录下生成 xml 目录,在 xml 目录下自动生成 `ActivityDao.xml`文件

* 3、编辑 `ActivityDao.xml`文件，即: 在 `ActivityDao.xml` 中写具体的 `SQL`语句
````xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE mapper PUBLIC "-//mybatis.org//DTD Mapper 3.0//EN"
       "https://github.com/kotlin2018/mbt/blob/master/mybatis.dtd">
<mapper namespace=".ActivityDao">
    <!--logic_enable 逻辑删除字段-->
    <!--logic_deleted 逻辑删除已删除字段-->
    <!--logic_undelete 逻辑删除 未删除字段-->
    <!--version_enable 乐观锁版本字段,支持int,int8,int16,int32,int64-->
	<resultMap id="base" table="biz_activity">
    	<result column="id" property="Id" langType="string"  />
	<result column="name" property="Name" langType="string"  />
	<result column="pc_link" property="PcLink" langType="string"  />
	<result column="h5_link" property="H5Link" langType="string"  />
	<result column="remark" property="Remark" langType="string"  />
	<result column="sort" property="Sort" langType="int"  />
	<result column="status" property="Status" langType="int"  />
	<result column="version" property="Version" langType="int"  />
	<result column="create_time" property="CreateTime" langType="time.Time"  />
	<result column="delete_flag" property="DeleteFlag" langType="int"  />
	<result column="salary" property="Salary" langType="float64"  />
    </resultMap>

	<!--插入模板:默认id="insert" 支持批量插入 -->
	<insert id="insert" resultMap="base" />

	<!--删除模板:默认id="delete",where自动设置逻辑删除字段-->
	<delete id="delete" resultMap="base" where=""/>

	<!--更新模板:默认id="update",set自动设置乐观锁版本号-->
	<update id="update" set="" resultMap="base" where=""/>

	<!--查询模板:默认id="select",where自动设置逻辑删除字段-->
	<select id="select" column="" resultMap="base" where=""/>
</mapper>

````

# 2、默认支持的数据库驱动:
````
"mysql"，"tidb" ====> "github.com/go-sql-driver/mysql"


"mymysql"   ====> "github.com/ziutek/mymysql/godrv"


"postgres","cockroachDB"  ====> "github.com/lib/pq"


"sqlite3","sqlite" ====> "github.com/mattn/go-sqlite3"


"mssql" ====> "github.com/denisenkom/go-mssqldb"


"oci8" ====> "github.com/mattn/go-oci8"


"clickhouse" ====> "github.com/ClickHouse/clickhouse-go"

````

* 自定义数据库驱动:
````
先实现 mbt.Convert 接口，再调用 Session.Driver()，将自定义的驱动注册到当前 Session 对象中即可。
````

* [查询是否要使用事务](https://blog.csdn.net/weixin_34157892/article/details/114553584)

* [具体使用example](https://github.com/kotlin2018/example.git)
