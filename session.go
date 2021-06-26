package mbt

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"reflect"
	"strconv"
	"time"
)
var (
	ra = rand.Reader
	timeDefault       time.Time
	timeType = reflect.TypeOf(timeDefault)
)
type (
	Tx func()*Session
	H map[interface{}]interface{}
	uuid [16]byte
	Database struct {
		Pkg             string  `yaml:"pkg" toml:"pkg"`                               // 生成的xml文件的包名
		DriverName      string  `yaml:"driver_name" toml:"driver_name"`               // 驱动名称。例如: mysql,postgreSQL...
		DSN             string  `yaml:"dsn" toml:"dsn"`                               // 数据库连接信息。例如: "root:root@(127.0.0.1:3306)/test?charset=utf8&parseTime=True&loc=Local"
		MaxOpenConn     int     `yaml:"max_open_conn" toml:"max_open_conn"`           // 最大的并发打开连接数。例如: 这个值是5则表示==>连接池中最多有5个并发打开的连接，如果5个连接都已经打开被使用，并且应用程序需要另一个连接的话，那么应用程序将被迫等待，直到5个打开的连接其中的一个被释放并变为空闲。
		MaxIdleConn     int     `yaml:"max_idle_conn" toml:"max_idle_conn"`           // 最大的空闲连接数。注意: MaxIdleConn 应该始终小于或等于 MaxOpenConn，设置比 MaxOpenConn 更多的空闲连接数是没有意义的，因为你最多也就能拿到所有打开的连接，剩余的空闲连接依然保持的空闲。
		ConnMaxLifetime int     `yaml:"conn_max_life_time" toml:"conn_max_life_time"` // 单位 time.Minute 连接的最大生命周期(默认值:0)。设置为0的话意味着没有最大生命周期，连接总是可重用。注意: ConnMaxLifetime 越短，从零开始创建连接的频率就越高!
		ConnMaxIdleTime int     `yaml:"conn_max_idle_time" toml:"conn_max_idle_time"` // 单位 time.Minute
		Logger          *logger `yaml:"logger" toml:"logger"`                         // logger日志记录器
		Namespace       string  `yaml:"namespace" toml:"namespace"`                   // dao 结构体的具体相对路径
	}
	logger struct {
		PrintSql bool   `yaml:"print_sql" toml:"print_sql"` // 设置是否打印SQL语句
		PrintXml bool   `yaml:"print_xml" toml:"print_xml"` // 是否打印 xml文件信息
		Path     string `yaml:"path" toml:"path"`           // 日志输出路径
		LinkName string `yaml:"link_name" toml:"link_name"` // 为最新的日志建立软连接
		Interval int    `yaml:"interval" toml:"interval"`   // 设置日志分割的时间，隔多久分割一次
		MaxAge   int    `yaml:"max_age" toml:"max_age"`     // 日志文件被清理前的最长保存时间
		Count    int    `yaml:"count" toml:"count"`         // 日志文件被清理前最多保存的个数,(-1 表示不使用该项)
	}
	result struct {
		LastInsertId int64
		RowsAffected int64
	}
	proxyArg struct {
		TagArgs    []tagArg
		TagArgsLen int
		Args       []reflect.Value
		ArgsLen    int
	}
	tagArg struct {
		Name  string
		Index int
	}
	returnValue struct {
		Error *reflect.Type
		Value *reflect.Type
		Num   int
		Index int
		xml   *element
		nodes []iiNode
		name  string
	}
	Session struct {
		db         *sql.DB
		tx         []*sql.Tx
		stmt       *sql.Stmt
		i          int
		sessionId  string
		driverName string
		dsn        string
		log        *log.Logger
		pkg        string
		printXml   bool
		printSql   bool
		namespace  string
		data       map[reflect.Value]map[string]*returnValue
	}
)
func New(cfg *Database)(*Session,*sql.DB,error){
	db, err := sql.Open(cfg.DriverName, cfg.DSN)
	if err != nil{
		return nil,nil,err
	}
	db.SetConnMaxIdleTime(time.Duration(cfg.ConnMaxIdleTime) * time.Minute)
	db.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Minute)
	db.SetMaxIdleConns(cfg.MaxIdleConn)
	db.SetMaxOpenConns(cfg.MaxOpenConn)
	it := &Session{
		db: db,
		sessionId: newUUID().String(),
		driverName: cfg.DriverName,
		dsn: cfg.DSN,
		printSql: cfg.Logger.PrintSql,
		printXml: cfg.Logger.PrintXml,
		namespace: cfg.Namespace,
		pkg: cfg.Pkg,
		log: log.New(os.Stdout,"[INFO] ",log.LstdFlags),
	}
	return it,db,nil
}
func (it *Session)SetOutPut(w io.Writer)*Session{
	it.log.SetOutput(w)
	return it
}
func (it *Session) Register(h H)*Session{
	for i, v:= range h {
		it.register(i,v)
	}
	return it
}
// 生成雪花算法的ID
func (it *Session) ID(node int64) Id {
	id, _ := newNode(node)
	return id.Generate()
}
// 输入参数为: mysql,mymysql,postgres,sqlite3或者sqlite,mssql,oci8,tidb,cockroachDB
// 返回值为: 这些数据库的驱动地址!
func (it *Session)Driver()string{
	switch it.driverName {
	case "mysql","tidb":
		return "github.com/go-sql-driver/mysql"
	case "mymysql":
		return "github.com/ziutek/mymysql/godrv"
	case "postgres","cockroachDB":
		return "github.com/lib/pq"
	case "sqlite3","sqlite":
		return "github.com/mattn/go-sqlite3"
	case "mssql":
		return "github.com/denisenkom/go-mssqldb"
	case "oci8":
		return "github.com/mattn/go-oci8"
	default:
		return ""
	}
}
func (u uuid) String() string {
	var buf [36]byte
	encodeHex(buf[:], u)
	return string(buf[:])
}
func newUUID()uuid{
	var u uuid
	io.ReadFull(ra, u[:])
	u[6] = (u[6] & 0x0f) | 0x40
	u[8] = (u[8] & 0x3f) | 0x80
	return u
}
func encodeHex(dst []byte, u uuid) {
	hex.Encode(dst, u[:4])
	dst[8] = '-'
	hex.Encode(dst[9:13], u[4:6])
	dst[13] = '-'
	hex.Encode(dst[14:18], u[6:8])
	dst[18] = '-'
	hex.Encode(dst[19:23], u[8:10])
	dst[23] = '-'
	hex.Encode(dst[24:], u[10:])
}
func (it *Session) last() *sql.Tx {
	if it.i == 0 {
		return nil
	}
	return it.tx[it.i-1]
}
func (it *Session) pop() *sql.Tx {
	if it.i == 0 {
		return nil
	}
	it.i--
	ret := it.tx[it.i]
	it.tx = it.tx[0:it.i]
	return ret
}
func (it *Session) push(k *sql.Tx) {
	it.tx = append(it.tx, k)
	it.i++
}
func (it *Session) id() string {
	return it.sessionId
}
func (it *Session) Rollback() error {
	t := it.pop()
	if t != nil{
		err := t.Rollback()
		if err != nil {
			return err
		}
		if it.printSql {
			it.log.Println(" Rollback() tx SessionId == "+"[",it.id(),"]")
		}
	}
	return nil
}
func (it *Session) Commit() error {
	t := it.pop()
	if t != nil {
		err := t.Commit()
		if err != nil {
			return err
		}
		if it.printSql {
			it.log.Println(" Commit() tx SessionId == "+"[",it.id(),"]")
		}
	}
	return nil
}
func (it *Session) Begin() error {
	it.tx = make([]*sql.Tx, 0)
	it.i = 0
	t, err := it.db.Begin()
	if err != nil {
		return err
	}
	it.push(t)
	if it.printSql {
		it.log.Println(" Begin() tx SessionId == "+"[",it.id(),"]")
	}
	return nil
}
func (it *Session) queryPrepare(sqlPrepare string, args ...interface{}) ([]map[string][]byte, error) {
	var (
		rows *sql.Rows
		stmt *sql.Stmt
		err error
		t = it.last()
	)
	if t != nil {
		stmt, err = t.Prepare(sqlPrepare)
		if err != nil {
			return nil, err
		}
		rows, err = stmt.Query(args...)
		if err != nil {
			return nil, err
		}
	} else {
		stmt, err = it.db.Prepare(sqlPrepare)
		if err != nil {
			return nil, err
		}
		rows, err = stmt.Query(args...)
		if err != nil {
			return nil, err
		}
	}
	if stmt != nil {
		defer stmt.Close()
	}
	if rows != nil {
		defer rows.Close()
	}
	if err != nil {
		return nil, err
	} else {
		return rows2maps(rows)
	}
}
func (it *Session) execPrepare(sqlPrepare string, args ...interface{}) (*result, error) {
	var (
		res sql.Result
		stmt *sql.Stmt
		err error
		t = it.last()
	)
	if t != nil {
		stmt, err = t.Prepare(sqlPrepare)
		if err != nil {
			return nil, err
		}
		res, err = stmt.Exec(args...)
		if err != nil {
			return nil, err
		}
	} else {
		stmt, err = it.db.Prepare(sqlPrepare)
		if err != nil {
			return nil, err
		}
		res, err = stmt.Exec(args...)
		if err != nil {
			return nil, err
		}
	}
	if stmt != nil {
		defer stmt.Close()
	}
	if err != nil {
		return nil, err
	} else {
		LastInsertId, _ := res.LastInsertId()
		RowsAffected, _ := res.RowsAffected()
		return &result{
			LastInsertId: LastInsertId,
			RowsAffected: RowsAffected,
		}, nil
	}
}
func rows2maps(rows *sql.Rows) (resultsSlice []map[string][]byte, err error) {
	fields, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		res, err := row2map(rows, fields)
		if err != nil {
			return nil, err
		}
		resultsSlice = append(resultsSlice, res)
	}
	return resultsSlice, nil
}
func row2map(rows *sql.Rows, fields []string) (resultsMap map[string][]byte, err error) {
	res := make(map[string][]byte)
	num := len(fields)
	list := make([]interface{},num)
	for i := 0; i < num; i++ {
		var obj interface{}
		list[i] = &obj
	}
	if err = rows.Scan(list...); err != nil {
		return nil, err
	}
	for j, v := range fields {
		rawValue := reflect.Indirect(reflect.ValueOf(list[j]))
		if rawValue.Interface() == nil {
			res[v] = []byte{}
			continue
		}
		if data, err := value2Bytes(&rawValue); err == nil {
			res[v] = data
		} else {
			return nil, err // REVIEW, should return err or just error logger?
		}
	}
	return res, nil
}
func value2Bytes(rawValue *reflect.Value) ([]byte, error) {
	var (
		str string
		err error
	)
	aa := reflect.TypeOf((*rawValue).Interface())
	vv := reflect.ValueOf((*rawValue).Interface())
	switch aa.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		str = strconv.FormatInt(vv.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		str = strconv.FormatUint(vv.Uint(), 10)
	case reflect.Float32, reflect.Float64:
		str = strconv.FormatFloat(vv.Float(), 'f', -1, 64)
	case reflect.String:
		str = vv.String()
	case reflect.Array, reflect.Slice:
		switch aa.Elem().Kind() {
		case reflect.Uint8:
			data := rawValue.Interface().([]byte)
			str = string(data)
			if str == "\x00" {
				str = "0"
			}
		default:
			err = fmt.Errorf(" Unsupported struct type %v", vv.Type().Name())
		}
	case reflect.Struct:
		if aa.ConvertibleTo(timeType) {
			str = vv.Convert(timeType).Interface().(time.Time).Format(time.RFC3339Nano)
		} else {
			err = fmt.Errorf(" Unsupported struct type %v", vv.Type().Name())
		}
	case reflect.Bool:
		str = strconv.FormatBool(vv.Bool())
	case reflect.Complex128, reflect.Complex64:
		str = fmt.Sprintf("%v", vv.Complex())
	default:
		err = fmt.Errorf(" Unsupported struct type %v", vv.Type().Name())
	}
	return []byte(str),err
}



