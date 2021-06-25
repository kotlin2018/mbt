package mbt

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"go/scanner"
	tk "go/token"
	"io"
	"log"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)
type (
	Tx func()Session
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
	H map[interface{}]interface{}
)
type Engine struct {
	s         Session
	m         sync.Map //用来缓存*Session
	log       *log.Logger
	pkg       string
	printXml  bool
	printSql  bool
	namespace string
	data      map[reflect.Value]map[string]*returnValue
}
func New(cfg *Database)(*Engine,*sql.DB,error){
	db, err := sql.Open(cfg.DriverName, cfg.DSN)
	if err != nil{
		return nil,nil,err
	}
	it := new(Engine)
	it.log = log.New(os.Stdout,"[INFO] ",log.LstdFlags)
	it.printXml = cfg.Logger.PrintXml
	it.printSql = cfg.Logger.PrintSql
	it.pkg = cfg.Pkg
	it.namespace = cfg.Namespace
	db.SetConnMaxIdleTime(time.Duration(cfg.ConnMaxIdleTime) * time.Minute)
	db.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Minute)
	db.SetMaxIdleConns(cfg.MaxIdleConn)
	db.SetMaxOpenConns(cfg.MaxOpenConn)
	it.s =Session(&session{
		SessionId:   newUUID().String(),
		db:          db,
		tx:          make([]*sql.Tx, 0),
		i:           0,
		driverType:  cfg.DriverName,
		dsn:         cfg.DSN,
		printLog:    it.printSql,
		log:         it.log,
	})
	return it,db,nil
}
func (it *Engine)SetOutPut(w io.Writer)*Engine{
	it.log.SetOutput(w)
	return it
}
func (it *Engine) Register(h H)*Engine{
	for i, v:= range h {
		it.register(i,v)
	}
	return it
}
// 生成雪花算法的ID
func (it *Engine) ID(node int64) Id {
	id, _ := newNode(node)
	return id.Generate()
}
// 缓存*Session
func (it *Engine)put(k int64,s Session){
	it.m.Store(k,s)
}
func (it *Engine)get(k int64)Session{
	v, ok := it.m.Load(k)
	if ok {
		return v.(*session)
	}else {
		return nil
	}
}
func (it *Engine)delete(k int64){
	it.m.Delete(k)
}
func goroutineID() int64{
	b := make([]byte, 64)
	b = b[:runtime.Stack(b, false)]
	b = bytes.TrimPrefix(b, []byte("goroutine "))
	b = b[:bytes.IndexByte(b, ' ')]
	n, _ := strconv.ParseInt(string(b), 10, 64)
	return n
}
// 输入参数为: mysql,mymysql,postgres,sqlite3或者sqlite,mssql,oci8,tidb,cockroachDB
// 返回值为: 这些数据库的驱动地址!
func (it *Engine)Driver(driverType string)string{
	switch driverType {
	case "mysql":
		return "github.com/go-sql-driver/mysql"
	case "mymysql":
		return "github.com/ziutek/mymysql/godrv"
	case "postgres":
		return "github.com/lib/pq"
	case "sqlite3","sqlite":
		return "github.com/mattn/go-sqlite3"
	case "mssql":
		return "github.com/denisenkom/go-mssqldb"
	case "oci8":
		return "github.com/mattn/go-oci8"
	case "tidb":
		return "github.com/go-sql-driver/mysql"
	case "cockroachDB":
		return "github.com/lib/pq"
	default:
		return "github.com/go-sql-driver/mysql"
	}
}
func isMethodElement(tag string) bool {
	switch tag {
	case "insert", "delete", "update", "select":
		return true
	}
	return false
}
var (
	timeDefault       time.Time
	timeType = reflect.TypeOf(timeDefault)
)
type (
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
)
func newArg(tagArgs []tagArg,args []reflect.Value)proxyArg{
	return proxyArg{
		TagArgs:    tagArgs,
		Args:       args,
		TagArgsLen: len(tagArgs),
		ArgsLen:    len(args),
	}
}
type nodeType int
const (
	nArg    nodeType = iota
	nString
	nFloat
	nInt
	nUInt
	nBool
	nNil
	nBinary
	nOpt
)
func (it nodeType) ToString() string {
	switch it {
	case nArg:
		return "NArg"
	case nString:
		return "NString"
	case nFloat:
		return "NFloat"
	case nInt:
		return "NInt"
	case nUInt:
		return "NUInt"
	case nBool:
		return "NBool"
	case nNil:
		return "NNil"
	case nBinary:
		return "NBinary"
	case nOpt:
		return "NOpt"
	}
	return "Unknown"
}
type operator = string
const (
	add    operator = "+"
	reduce operator = "-"
	ride   operator = "*"
	divide operator = "/"
	and       operator = "&&"
	or        operator = "||"
	equal     operator = "=="
	unEqual   operator = "!="
	less      operator = "<"
	lessEqual operator = "<="
	more      operator = ">"
	moreEqual operator = ">="
	nils  operator = "nil"
	null operator = "null"
)

var priorityArray = []operator{ride, divide, add, reduce,
	lessEqual, less, moreEqual, more,
	unEqual, equal, and, or}

var notSupportOptMap = map[string]bool{
	"=": true,
	"!": true,
	"@": true,
	"#": true,
	"$": true,
	"^": true,
	"&": true,
	"(": true,
	")": true,
	"`": true,
}
var priorityMap = map[operator]int{}
func init() {
	for k, v := range priorityArray {
		priorityMap[v] = k
	}
}
func parser(express string) (iNode, error) {
	opts := parserOperators(express)
	var list []iNode
	for _, v := range opts {
		item, err := parserNode(express, v)
		if err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	err := checkNodes(express, list)
	if err != nil {
		return nil, err
	}
	for _, v := range priorityArray {
		e := findReplaceOpt(express, v, &list)
		if e != nil {
			return nil, e
		}
	}
	if len(list) == 0 || list[0] == nil {
		return nil, errors.New(" parser node fail!")
	}
	return list[0], nil
}
func checkNodes(express string, nodes []iNode) error {
	var nodesLen = len(nodes)
	for nIndex, n := range nodes {
		if n.Type() == nOpt {
			var after iNode
			var befor iNode
			if nIndex > 0 {
				befor = nodes[nIndex-1]
			}
			if nIndex < (nodesLen - 1) {
				after = nodes[nIndex+1]
			}
			if after != nil && after.Type() == nOpt {
				return errors.New("express have more than 2 opt!express=" + express)
			}
			if befor != nil && befor.Type() == nOpt {
				return errors.New("express have more than 2 opt!express=" + express)
			}
		}
	}
	return nil
}
func parserNode(express string, v operator) (iNode, error) {
	if v == nils || v == null {
		var inode = nilNode{
			t: nNil,
		}
		return inode, nil
	}
	if notSupportOptMap[v] {
		return nil, errors.New("find not support opt = '" + v + "',express=" + express)
	}
	if isOperatorsAction(v) {
		var opt = optNode{
			value: v,
			t:     nOpt,
		}
		return opt, nil
	}
	if v == "true" || v == "false" {
		b, e := strconv.ParseBool(v)
		if e == nil {
			var inode = boolNode{
				value: b,
				t:     nBool,
			}
			return inode, nil
		}
	}
	if strings.Index(v, "'") == 0 && strings.LastIndex(v, "'") == (len(v)-1) {
		var inode = stringNode{
			value: string([]byte(v)[1 : len(v)-1]),
			t:     nString,
		}
		return inode, nil
	}
	if strings.Index(v, "`") == 0 && strings.LastIndex(v, "`") == (len(v)-1) {
		var inode = stringNode{
			value: string([]byte(v)[1 : len(v)-1]),
			t:     nString,
		}
		return inode, nil
	}
	i, e := strconv.ParseInt(v, 0, 64)
	if e == nil {
		var inode = intNode{
			express: v,
			value:   int64(i),
			t:       nInt,
		}
		return inode, nil
	}
	u, _ := strconv.ParseUint(v, 0, 64)
	if e == nil {
		var inode = uIntNode{
			express: v,
			value:   u,
			t:       nUInt,
		}
		return inode, nil
	}
	f, e := strconv.ParseFloat(v, 64)
	if e == nil {
		var inode = floatNode{
			express: v,
			value:   f,
			t:       nFloat,
		}
		return inode, nil
	}
	e = nil
	values := strings.Split(v, ".")
	arg := argNode{
		value:  v,
		values: values,
		length: len(values),
		t:      nArg,
	}
	return arg, nil
}
func findReplaceOpt(express string, operator operator, list *[]iNode) error {
	array := *list
	for nIndex, n := range array {
		if n.Type() == nOpt {
			opt := n.(optNode)
			if opt.value != operator {
				continue
			}
			nod := binaryNode{
				left:  array[nIndex-1],
				right: array[nIndex+1],
				opt:   opt.value,
				t:     nBinary,
			}
			var newNodes []iNode
			newNodes = append(array[:nIndex-1], nod)
			newNodes = append(newNodes, array[nIndex+2:]...)
			if haveOpt(newNodes) {
				findReplaceOpt(express, operator, &newNodes)
			}
			*list = newNodes
			break
		}
	}
	return nil
}
func haveOpt(nodes []iNode) bool {
	for _, v := range nodes {
		if v.Type() == nOpt {
			return true
		}
	}
	return false
}
func parserOperators(express string) []operator {
	var (
		newResult []string
		ss scanner.Scanner
		lastToken tk.Token
	)
	src := []byte(express)
	fset := tk.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))
	ss.Init(file, src, nil, 0)
	index := 0
	for {
		_, tok, lit := ss.Scan()
		if tok == tk.EOF || lit == "\n" {
			break
		}
		s := toStr(lit, tok)
		if lit == "" && tok != tk.ILLEGAL {
			lastToken = tok
		}
		if tok == tk.PERIOD || lastToken == tk.PERIOD {
			newResult[len(newResult)-1] = newResult[len(newResult)-1] + s
			continue
		}
		if index >= 1 && isNumber(s) && newResult[index-1] == "-" {
			if index == 1 {
				newResult = []string{}
				s = "-" + s
				index -= 1
			} else {
				if index > 2 && isOperatorsAction(newResult[index-2]) {
					newResult = newResult[:2]
					s = "-" + s
					index -= 1
				}
			}
		}
		newResult = append(newResult, s)
		index += 1
	}
	return newResult
}
func isNumber(s string) bool {
	var o0 = rune([]byte("0")[0])
	var o1 = rune([]byte("1")[0])
	var o2 = rune([]byte("2")[0])
	var o3 = rune([]byte("3")[0])
	var o4 = rune([]byte("4")[0])
	var o5 = rune([]byte("5")[0])
	var o6 = rune([]byte("6")[0])
	var o7 = rune([]byte("7")[0])
	var o8 = rune([]byte("8")[0])
	var o9 = rune([]byte("9")[0])
	var o10 = rune([]byte("9")[0])
	var o11 = rune([]byte(".")[0])
	for _, v := range s {
		if o0 != v &&
			o1 != v &&
			o2 != v &&
			o3 != v &&
			o4 != v &&
			o5 != v &&
			o6 != v &&
			o7 != v &&
			o8 != v &&
			o9 != v &&
			o10 != v &&
			o11 != v {
			return false
		}
	}
	return true
}
func toStr(lit string, tok tk.Token) string {
	if lit == "" {
		return tok.String()
	} else {
		return lit
	}
}
func isOperatorsAction(arg string) bool {
	if arg == add ||
		arg == reduce ||
		arg == ride ||
		arg == divide ||
		arg == and ||
		arg == or ||
		arg == equal ||
		arg == unEqual ||
		arg == less ||
		arg == lessEqual ||
		arg == more ||
		arg == moreEqual {
		return true
	}
	return false
}
type nodeExpress struct {}
func (it *nodeExpress) Lexer(expression string) (interface{}, error) {
	expression = it.replaceExpression(expression)
	res, err := parser(expression)
	return res, err
}
func (it *nodeExpress) Eval(lexerResult interface{}, arg interface{}, operation int) (interface{}, error) {
	output, err := lexerResult.(iNode).Eval(arg)
	return output, err
}
func (it *nodeExpress) LexerAndEval(expression string, arg interface{}) (interface{}, error) {
	funcItem := arg.(map[string]interface{})["func_"+expression]
	if funcItem != nil {
		f := funcItem.(func(arg map[string]interface{}) interface{})
		return f(arg.(map[string]interface{})), nil
	}
	res, err := it.Lexer(expression)
	if err != nil {
		return false,err
	}
	res1, err := it.Eval(res, arg, 0)
	if err != nil {
		return false,err
	}
	return res1, nil
}
func (it *nodeExpress) replaceExpression(expression string) string {
	if expression == "" {
		return expression
	}
	expression = strings.Replace(expression, ` and `, " && ", -1)
	expression = strings.Replace(expression, ` or `, " || ", -1)
	return expression
}
type iNode interface {
	Type() nodeType
	Eval(env interface{}) (interface{}, error)
	Express() string
}
type optNode struct {
	value operator
	t     nodeType
}
func (it optNode) Type() nodeType {
	return nOpt
}
func (it optNode) IsCalculationOperator() bool {
	if it.value == add ||
		it.value == reduce ||
		it.value == ride ||
		it.value == divide {
		return true
	}
	return false
}
func (it optNode) Express() string {
	return it.value
}
func (it optNode) Eval(env interface{}) (interface{}, error) {
	return it.value, nil
}
type argNode struct {
	value  string
	values []string
	length int
	t      nodeType
}
func (it argNode) Type() nodeType {
	return nArg
}
func (it argNode) Express() string {
	return it.value
}
func (it argNode) Eval(env interface{}) (interface{}, error) {
	return evalTake(it, env)
}
type stringNode struct {
	value string
	t     nodeType
}
func (it stringNode) Type() nodeType {
	return nString
}
func (it stringNode) Express() string {
	return it.value
}
func (it stringNode) Eval(env interface{}) (interface{}, error) {
	return it.value, nil
}
type floatNode struct {
	express string
	value   float64
	t       nodeType
}
func (it floatNode) Express() string {
	return it.express
}
func (it floatNode) Type() nodeType {
	return nFloat
}
func (it floatNode) Eval(env interface{}) (interface{}, error) {
	return it.value, nil
}
type intNode struct {
	express string
	value   int64
	t       nodeType
}
func (it intNode) Express() string {
	return it.express
}
func (it intNode) Type() nodeType {
	return nInt
}
func (it intNode) Eval(env interface{}) (interface{}, error) {
	return it.value, nil
}
type uIntNode struct {
	express string
	value   uint64
	t       nodeType
}
func (it uIntNode) Type() nodeType {
	return nUInt
}
func (it uIntNode) Express() string {
	return it.express
}
func (it uIntNode) Eval(env interface{}) (interface{}, error) {
	return it.value, nil
}
type boolNode struct {
	value bool
	t     nodeType
}
func (it boolNode) Type() nodeType {
	return nBool
}
func (it boolNode) Express() string {
	if it.value {
		return "true"
	} else {
		return "false"
	}
}
func (it boolNode) Eval(env interface{}) (interface{}, error) {
	return it.value, nil
}
type nilNode struct {
	t nodeType
}
func (it nilNode) Type() nodeType {
	return nNil
}
func (it nilNode) Express() string {
	return "nil"
}
func (nilNode) Eval(env interface{}) (interface{}, error) {
	return nil, nil
}
type binaryNode struct {
	left  iNode
	right iNode
	opt   operator
	t     nodeType
}
func (it binaryNode) Type() nodeType {
	return nBinary
}
func (it binaryNode) Express() string {
	var s = ""
	if it.left != nil {
		s += it.left.Express()
	}
	if it.right != nil {
		s += it.right.Express()
	}
	return s
}
func (it binaryNode) Eval(env interface{}) (interface{}, error) {
	var left interface{}
	var right interface{}
	var e error
	if it.left != nil {
		left, e = it.left.Eval(env)
		if e != nil {
			return nil, e
		}
	}
	if it.right != nil {
		right, e = it.right.Eval(env)
		if e != nil {
			return nil, e
		}
	}
	return eval(it.Express(), it.opt, left, right)
}
func evalTake(argNode argNode, arg interface{}) (interface{}, error) {
	if arg == nil || argNode.values == nil {
		return nil, nil
	}
	if argNode.value == "" || argNode.length == 0 {
		return arg, nil
	}
	var av = reflect.ValueOf(arg)
	if av.Kind() == reflect.Map {
		var m = arg.(map[string]interface{})
		if argNode.length == 1 {
			return m[argNode.value], nil
		}
		return takeValue(argNode.value, av.MapIndex(reflect.ValueOf(argNode.values[0])), argNode.values[1:])
	} else {
		if argNode.length == 1 {
			return arg, nil
		}
		return takeValue(argNode.value, av, argNode.values[1:])
	}
}
func takeValue(key string, arg reflect.Value, field []string) (interface{}, error) {
	if arg.IsValid() == false {
		return nil, nil
	}
	for _, v := range field {
		argItem, e := getObj(key, v, arg)
		if e != nil || argItem == nil {
			return nil, e
		}
		arg = *argItem
	}
	if !arg.IsValid() {
		return nil, nil
	}
	if arg.CanInterface() {
		inter := arg.Interface()
		return inter, nil
	} else {
		return nil, nil
	}
}
func getObj(key string, operator operator, av reflect.Value) (*reflect.Value, error) {
	if av.Kind() == reflect.Ptr || av.Kind() == reflect.Interface {
		av = getDeepPtr(av)
	}
	if av.Kind() == reflect.Map {
		var mapV = av.MapIndex(reflect.ValueOf(operator))
		return &mapV, nil
	}
	if av.Kind() != reflect.Struct {
		return nil, errors.New("[express] " + key + " get value  " + key + "  fail :" + av.String() + ",value key:" + operator)
	}
	av = av.FieldByName(operator)
	if av.Kind() == reflect.Ptr || av.Kind() == reflect.Interface {
		av = getDeepPtr(av)
	}
	if av.IsValid() && av.CanInterface() {
		return &av, nil
	} else {
		return nil, nil
	}
}
func eval(express string, operator operator, a interface{}, b interface{}) (interface{}, error) {
	av := reflect.ValueOf(a)
	bv := reflect.ValueOf(b)
	switch operator {
	case and:
		if a == nil || b == nil {
			return nil, errors.New("[express] " + express + " eval fail,value can not be nil")
		}
		a, av = getDeepValue(av, a)
		b, bv = getDeepValue(bv, b)
		var ab = a.(bool)
		var bb = b.(bool)
		return ab == true && bb == true, nil
	case or:
		if a == nil || b == nil {
			return nil, errors.New("[express] " + express + " eval fail,value can not be nil")
		}
		a, av = getDeepValue(av, a)
		b, bv = getDeepValue(bv, b)
		var ab = a.(bool)
		var bb = b.(bool)
		return ab == true || bb == true, nil
	case equal, moreEqual, more, less, lessEqual:
		return doEqualAction(express, operator, a, b, av, bv)
	case unEqual:
		var r, e = doEqualAction(express, operator, a, b, av, bv)
		if e != nil {
			return nil, e
		}
		return !r, nil
	case add, reduce, ride, divide:
		return doCalculationAction(express, operator, a, b, av, bv)
	}
	return nil, errors.New("[express] " + express + " find not support operator :" + operator)
}
func doEqualAction(express string, operator operator, a interface{}, b interface{}, av reflect.Value, bv reflect.Value) (bool, error) {
	switch operator {
	case unEqual:
		fallthrough
	case equal:
		if av.Kind() == reflect.Ptr && av.IsNil() == true {
			a = nil
		}
		if bv.Kind() == reflect.Ptr && bv.IsNil() == true {
			b = nil
		}
		if a == nil || b == nil {
			if a != nil || b != nil {
				return false, nil
			}
			if a == nil && b == nil {
				return true, nil
			}
		}
		if av.Kind() == reflect.Ptr {
			a, av = getDeepValue(av, a)
		}
		if bv.Kind() == reflect.Ptr {
			b, bv = getDeepValue(bv, b)
		}
		if av.Kind() == reflect.Float64 && bv.Kind() == reflect.Float64 {
			return a.(float64) == b.(float64), nil
		}
		if av.Kind() == reflect.Float32 && bv.Kind() == reflect.Float32 {
			return a.(float32) == b.(float32), nil
		}
		if av.Kind() == reflect.Int && bv.Kind() == reflect.Int {
			return a.(int) == b.(int), nil
		}
		if av.Kind() == reflect.Int8 && bv.Kind() == reflect.Int8 {
			return a.(int8) == b.(int8), nil
		}
		if av.Kind() == reflect.Int16 && bv.Kind() == reflect.Int16 {
			return a.(int16) == b.(int16), nil
		}
		if av.Kind() == reflect.Int32 && bv.Kind() == reflect.Int32 {
			return a.(int32) == b.(int32), nil
		}
		if av.Kind() == reflect.Int64 && bv.Kind() == reflect.Int64 {
			return a.(int64) == b.(int64), nil
		}
		if av.Kind() == reflect.Bool && bv.Kind() == reflect.Bool {
			return a.(bool) == b.(bool), nil
		}
		if av.Kind() == reflect.String && bv.Kind() == reflect.String {
			return a.(string) == b.(string), nil
		}
		if av.Kind() == reflect.Struct && bv.Kind() == reflect.String {
			return fmt.Sprint(a) == b.(string), nil
		}
		if bv.Kind() == reflect.Struct && av.Kind() == reflect.String {
			return fmt.Sprint(b) == a.(string), nil
		}
		a = toNumberType(av)
		b = toNumberType(bv)
		return a.(float64) == b.(float64), nil
	case less:
		if a == nil || b == nil {
			return false, errors.New("[express] " + express + "can not parser '<' , arg have nil object!")
		}
		a, av = getDeepValue(av, a)
		b, bv = getDeepValue(bv, b)
		a = toNumberType(av)
		b = toNumberType(bv)
		return a.(float64) < b.(float64), nil
	case more:
		if a == nil || b == nil {
			return false, errors.New("[express] " + express + "can not parser '>' , arg have nil object!")
		}
		a, av = getDeepValue(av, a)
		b, bv = getDeepValue(bv, b)
		a = toNumberType(av)
		b = toNumberType(bv)
		return a.(float64) > b.(float64), nil
	case moreEqual:
		if a == nil || b == nil {
			return false, errors.New("[express] " + express + "can not parser '>=' , arg have nil object!")
		}
		a, av = getDeepValue(av, a)
		b, bv = getDeepValue(bv, b)
		a = toNumberType(av)
		b = toNumberType(bv)
		return a.(float64) >= b.(float64), nil
	case lessEqual:
		if a == nil || b == nil {
			return false, errors.New("[express] " + express + "can not parser '<=' , arg have nil object!")
		}
		a, av = getDeepValue(av, a)
		b, bv = getDeepValue(bv, b)
		a = toNumberType(av)
		b = toNumberType(bv)
		return a.(float64) <= b.(float64), nil
	}
	return false, errors.New("[express] " + express + " find not support equal operator :" + operator)
}
func doCalculationAction(express string, operator operator, a interface{}, b interface{}, av reflect.Value, bv reflect.Value) (interface{}, error) {
	if a == nil || b == nil {
		return false, errors.New("[express] " + express + " have not a action operator!")
	}
	a, av = getDeepValue(av, a)
	b, bv = getDeepValue(bv, b)
	switch operator {
	case add:
		if av.Kind() == reflect.String {
			return a.(string) + b.(string), nil
		}
		a = toNumberType(av)
		b = toNumberType(bv)
		return a.(float64) + b.(float64), nil
	case reduce:
		a = toNumberType(av)
		b = toNumberType(bv)
		return a.(float64) - b.(float64), nil
	case ride:
		a = toNumberType(av)
		b = toNumberType(bv)
		return a.(float64) * b.(float64), nil
	case divide:
		a = toNumberType(av)
		b = toNumberType(bv)
		if b.(float64) == 0 {
			return nil, errors.New("[express] " + express + "can not divide zero value!")
		}
		return a.(float64) / b.(float64), nil
	}
	return "", errors.New("[express] " + express + "find not support operator :" + operator)
}
func getDeepPtr(v reflect.Value) reflect.Value {
	if v.Kind() != reflect.Ptr && v.Kind() != reflect.Interface {
		return v
	}
	if v.IsValid() {
		v = v.Elem()
		if v.IsValid() && (v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface) {
			getDeepPtr(v)
		}
	}
	return v
}
func getDeepValue(av reflect.Value, arg interface{}) (interface{}, reflect.Value) {
	if av.Kind() != reflect.Ptr {
		return arg, av
	}
	av = getDeepPtr(av)
	if av.IsValid() && av.CanInterface() {
		return av.Interface(), av
	}
	return arg, av
}
func toNumberType(v reflect.Value) float64 {
	r, ok := castType(v)
	if ok {
		return r
	}
	var vValue interface{}
	if v.IsValid() && v.CanInterface() {
		vValue = v.Interface()
	}
	panic(fmt.Sprint("[express] cannot convert ", vValue, " (type "+v.Type().String()+") to type float64"))
}
func castType(v reflect.Value) (float64, bool) {
	switch v.Kind() {
	case reflect.Float32, reflect.Float64:
		return v.Float(), true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(v.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(v.Uint()), true // TODO: Check if uint64 fits into float64.
	}
	return 0, false
}

