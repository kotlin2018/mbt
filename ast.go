package mbt

import (
	"bytes"
	"errors"
	"fmt"
	"go/scanner"
	tk "go/token"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)
type xmlNode int
const (
	nArg2 xmlNode = iota
	nStr
	nIf
	nTrim
	nForeach
	nChoose
	nOtherwise
	nWhen
	nBind
	nInclude
	nWhere
)
func (it xmlNode) ToString() string {
	switch it {
	case nStr:
		return "nString"
	case nIf:
		return "nIf"
	case nTrim:
		return "nTrim"
	case nForeach:
		return "nForeach"
	case nChoose:
		return "nChoose"
	case nOtherwise:
		return "nOtherwise"
	case nWhen:
		return "nWhen"
	case nBind:
		return "nBind"
	case nInclude:
		return "nInclude"
	case nWhere:
		return "nWhere"
	}
	return "Unknown"
}
type iiNode interface {
	Type() xmlNode
	Eval(env map[string]interface{}, array *[]interface{}, stmtConvert Convert) ([]byte, error)
}
func doChildNodes(childNodes []iiNode, env map[string]interface{},array *[]interface{}, stmtConvert Convert) ([]byte, error) {
	if childNodes == nil {
		return nil, nil
	}
	var sql bytes.Buffer
	for _, v := range childNodes {
		r, e := v.Eval(env, array, stmtConvert)
		if e != nil {
			return nil, e
		}
		if r != nil {
			sql.Write(r)
		}
	}
	bytes := sql.Bytes()
	sql.Reset()
	return bytes, nil
}
func replace(findStrS []string, data string, arg map[string]interface{}, engine iExpression, array *[]interface{}, indexConvert Convert) (string, error) {
	for _, findStr := range findStrS {
		argValue := arg[findStr]
		if argValue != nil {
			*array = append(*array, argValue)
		} else {
			evalData, err := engine.LexerAndEval(findStr, arg)
			if err != nil {
				return "", err
			}
			*array = append(*array, evalData)
		}
		indexConvert.Inc()
		data = strings.Replace(data, "#{"+findStr+"}", indexConvert.Convert(), -1)
	}
	return data, nil
}
func replaceRaw(findStrS []string, data string, typeConvert sqlArgTypeConv, arg map[string]interface{}, engine iExpression) (string, error) {
	for _, findStr := range findStrS {
		var (
			evalData interface{}
			argValue = arg[findStr]
			resultStr string
			err error
		)
		if argValue != nil {
			evalData = argValue
		} else {
			evalData, err = engine.LexerAndEval(findStr, arg)
			if err != nil {
				return "", err
			}
		}
		if typeConvert != nil {
			resultStr = typeConvert.Convert(evalData)
		} else {
			resultStr = fmt.Sprint(evalData)
		}
		data = strings.Replace(data, "${"+findStr+"}", resultStr, -1)
	}
	arg = nil
	typeConvert = nil
	return data, nil
}
func findExpress(str string) []string {
	var (
		item []byte
		lastIndex = -1
		startIndex = -1
		strBytes = []byte(str)
		finds = make([]string,0)
		strS = make([]string,0)
	)
	for index, v := range strBytes {
		if v == 35 {
			lastIndex = index
		}
		if v == 123 && lastIndex == (index-1) {
			startIndex = index + 1
		}
		if v == 125 && startIndex != -1 {
			item = strBytes[startIndex:index]
			if bytes.Contains(item, []byte(",")) {
				item = bytes.Split(item, []byte(","))[0]
			}
			finds = append(finds, string(item))
			item = nil
			startIndex = -1
			lastIndex = -1
		}
	}
	item = nil
	strBytes = nil
	for _, k := range finds {
		strS = append(strS, k)
	}
	return strS
}
func findRawExpressString(str string) []string {
	var (
		item []byte
		lastIndex = -1
		startIndex = -1
		strBytes = []byte(str)
		finds = make([]string,0)
		strS = make([]string,0)
	)
	for index, v := range str {
		if v == 36 {
			lastIndex = index
		}
		if v == 123 && lastIndex == (index-1) {
			startIndex = index + 1
		}
		if v == 125 && startIndex != -1 {
			item = strBytes[startIndex:index]
			if bytes.Contains(item, []byte(",")) {
				item = bytes.Split(item, []byte(","))[0]
			}
			finds = append(finds, string(item))
			item = nil
			startIndex = -1
			lastIndex = -1
		}
	}
	item = nil
	strBytes = nil
	for _, k := range finds {
		strS = append(strS, k)
	}
	return strS
}
type nodeString struct {
	value               string
	t                   xmlNode
	expressMap          []string
	noConvertExpressMap []string
	holder              express
}
func (it nodeString) Type() xmlNode {
	return nStr
}
func (it nodeString) Eval(env map[string]interface{}, array *[]interface{}, stmtConvert Convert) ([]byte, error) {
	var (
		data = it.value
		err error
	)
	if it.expressMap != nil {
		data, err = replace(it.expressMap, data, env, it.holder.Proxy, array, stmtConvert)
		if err != nil {
			return nil, err
		}
	}
	if it.noConvertExpressMap != nil {
		data, err = replaceRaw(it.noConvertExpressMap, data, nil, env, it.holder.Proxy)
		if err != nil {
			return nil, err
		}
	}
	return []byte(data), nil
}
type nodeBind struct {
	t      xmlNode
	name   string
	value  string
	holder express
}
func (it nodeBind) Type() xmlNode {
	return nBind
}
func (it nodeBind) Eval(env map[string]interface{}, array *[]interface{}, stmtConvert Convert) ([]byte, error) {
	if it.name == "" {
		panic(`[Error] 元素 <bind name = ""> 名称不能为空!`)
	}
	if it.value == "" {
		env[it.name] = it.value
		return nil, nil
	}
	res, err := it.holder.Proxy.LexerAndEval(it.value, env)
	if err != nil  {
		return nil, err
	}
	env[it.name] = res
	return nil, nil
}
type nodeChoose struct {
	t             xmlNode
	whenNodes     []iiNode
	otherwiseNode iiNode
}
func (it nodeChoose) Type() xmlNode {
	return nChoose
}
func (it nodeChoose) Eval(env map[string]interface{}, array *[]interface{}, stmtConvert Convert) ([]byte, error) {
	if it.whenNodes == nil && it.otherwiseNode == nil {
		return nil, nil
	}
	for _, v := range it.whenNodes {
		var r, e = v.Eval(env, array, stmtConvert)
		if e != nil {
			return nil, e
		}
		if v.Type() == nWhen && r != nil {
			return r, nil
		}
	}
	return it.otherwiseNode.Eval(env, array, stmtConvert)
}
type nodeForeach struct {
	child      []iiNode
	t          xmlNode
	collection string
	index      string
	item       string
	open       string
	close      string
	separator  string
	holder     express
}
func (it nodeForeach) Type() xmlNode {
	return nForeach
}
func (it nodeForeach) Eval(env map[string]interface{}, array *[]interface{}, stmtConvert Convert) ([]byte, error) {
	if it.collection == "" {
		panic(`[Error] collection value can not be "" in <foreach collection=""> !`)
	}
	var tempSql bytes.Buffer
	evalData, err := it.holder.Proxy.LexerAndEval(it.collection, env)
	if err != nil {
		return nil, err
	}
	var (
		collectionValue = reflect.ValueOf(evalData)
		kind = collectionValue.Kind()
		collectionValueLen = collectionValue.Len()
	)
	if kind == reflect.Invalid {
		return nil, errors.New(" collection value is invalid value!")
	}
	if kind != reflect.Slice && kind != reflect.Array && kind != reflect.Map {
		panic(`[Error] collection value must be a slice or map !`)
	}
	if collectionValueLen == 0 {
		return nil, nil
	}
	if it.index == "" {
		it.index = "index"
	}
	if it.item == "" {
		it.item = "item"
	}
	switch collectionValue.Kind() {
	case reflect.Map:
		var (
			mapKeys = collectionValue.MapKeys()
			collectionKeyLen = len(mapKeys)
			tempArgMap = env
		)
		if collectionKeyLen == 0 {
			return nil, nil
		}
		for _, keyValue := range mapKeys {
			var (
				key = keyValue.Interface()
				collectionItem = collectionValue.MapIndex(keyValue)
			)
			if it.item != "" {
				tempArgMap[it.item] = collectionItem.Interface()
			}
			tempArgMap[it.index] = key
			r, err := doChildNodes(it.child, tempArgMap, array, stmtConvert)
			if err != nil {
				return nil, err
			}
			if r != nil {
				tempSql.Write(r)
			}
			tempSql.WriteString(it.separator)
			delete(tempArgMap, it.item)
		}
		break
	case reflect.Slice:
		tempArgMap := env
		for i := 0; i < collectionValueLen; i++ {
			collectionItem := collectionValue.Index(i)
			if it.item != "" {
				tempArgMap[it.item] = collectionItem.Interface()
			}
			if it.index != "" {
				tempArgMap[it.index] = i
			}
			r, err := doChildNodes(it.child, tempArgMap, array, stmtConvert)
			if err != nil {
				return nil, err
			}
			if r != nil {
				tempSql.Write(r)
			}
			tempSql.WriteString(it.separator)
			delete(tempArgMap, it.item)
		}
		break
	}
	var (
		newTempSql bytes.Buffer
		tempSqlString = bytes.Trim(tempSql.Bytes(), it.separator)
	)
	tempSql.Reset()
	newTempSql.WriteString(it.open)
	newTempSql.Write(tempSqlString)
	newTempSql.WriteString(it.close)
	newTempSqlBytes := newTempSql.Bytes()
	newTempSql.Reset()
	return newTempSqlBytes, nil
}
type nodeIf struct {
	child  []iiNode
	t      xmlNode
	test   string
	holder express
}
func (it nodeIf) Type() xmlNode {
	return nIf
}
func (it nodeIf) Eval(env map[string]interface{}, array *[]interface{}, stmtConvert Convert) ([]byte, error) {
	res, err := it.holder.Proxy.LexerAndEval(it.test, env)
	if err != nil {
		err = errors.New(fmt.Sprint("SqlBuilder", "[Error] <test `", it.test, `> fail,`, err.Error()))
	}
	if res.(bool) {
		return doChildNodes(it.child, env, array, stmtConvert)
	}
	return nil, nil
}
type nodeInclude struct {
	child []iiNode
	t     xmlNode
}
func (it nodeInclude) Type() xmlNode {
	return nInclude
}
func (it nodeInclude) Eval(env map[string]interface{}, array *[]interface{}, stmtConvert Convert) ([]byte, error) {
	sql, err := doChildNodes(it.child, env, array, stmtConvert)
	return sql, err
}
type nodeOtherwise struct {
	child []iiNode
	t     xmlNode
}
func (it nodeOtherwise) Type() xmlNode {
	return nOtherwise
}
func (it nodeOtherwise) Eval(env map[string]interface{}, array *[]interface{}, stmtConvert Convert) ([]byte, error) {
	r, e := doChildNodes(it.child, env, array, stmtConvert)
	if e != nil {
		return nil, e
	}
	return r, nil
}
type nodeTrim struct {
	child           []iiNode
	t               xmlNode
	prefix          []byte
	suffix          []byte
	suffixOverrides []byte
	prefixOverrides []byte
}
func (it nodeTrim) Type() xmlNode {
	return nTrim
}
func (it nodeTrim) Eval(env map[string]interface{}, array *[]interface{}, stmtConvert Convert) ([]byte, error) {
	sql, err := doChildNodes(it.child, env, array, stmtConvert)
	if err != nil {
		return nil, err
	}
	if sql == nil {
		return nil, nil
	}
	for {
		if bytes.HasPrefix(sql, []byte(" ")) {
			sql = bytes.Trim(sql, " ")
		} else {
			break
		}
	}
	if it.prefixOverrides != nil {
		prefixOverridesArray := bytes.Split(it.prefixOverrides, []byte("|"))
		if len(prefixOverridesArray) > 0 {
			for _, v := range prefixOverridesArray {
				sql = bytes.TrimPrefix(sql, v)
			}
		}
	}
	if it.suffixOverrides != nil {
		suffixOverrideArray := bytes.Split(it.suffixOverrides, []byte("|"))
		if len(suffixOverrideArray) > 0 {
			for _, v := range suffixOverrideArray {
				sql = bytes.TrimSuffix(sql, v)
			}
		}
	}
	var newBuffer bytes.Buffer
	newBuffer.WriteString(` `)
	newBuffer.Write(it.prefix)
	newBuffer.WriteString(` `)
	newBuffer.Write(sql)
	newBuffer.WriteString(` `)
	newBuffer.Write(it.suffix)
	newBufferBytes := newBuffer.Bytes()
	newBuffer.Reset()
	return newBufferBytes, nil
}
type nodeWhen struct {
	child  []iiNode
	test   string
	t      xmlNode
	holder express
}
func (it nodeWhen) Type() xmlNode {
	return nWhen
}
func (it nodeWhen) Eval(env map[string]interface{}, array *[]interface{}, stmtConvert Convert) ([]byte, error) {
	res, err := it.holder.Proxy.LexerAndEval(it.test, env)
	if err != nil {
		err = errors.New(fmt.Sprint("SqlBuilder", "[Error] <test `", it.test, `> fail,`, err.Error()))
	}
	if res.(bool) {
		return doChildNodes(it.child, env, array, stmtConvert)
	}
	return nil, nil
}
type nodeWhere struct {
	child []iiNode
	t     xmlNode
}
func (it nodeWhere) Type() xmlNode {
	return nWhere
}
func (it nodeWhere) Eval(env map[string]interface{}, array *[]interface{}, stmtConvert Convert) ([]byte, error) {
	sql, err := doChildNodes(it.child, env, array, stmtConvert)
	if err != nil {
		return nil, err
	}
	if sql == nil {
		return nil, nil
	}
	for {
		if bytes.HasPrefix(sql, []byte(" ")) {
			sql = bytes.Trim(sql, " ")
		} else {
			break
		}
	}
	if len(sql) == 0 {
		return sql, nil
	}
	sql = bytes.TrimPrefix(sql, []byte("and"))
	sql = bytes.TrimPrefix(sql, []byte("AND"))
	sql = bytes.TrimPrefix(sql, []byte("And"))
	sql = bytes.TrimPrefix(sql, []byte("or"))
	sql = bytes.TrimPrefix(sql, []byte("OR"))
	sql = bytes.TrimPrefix(sql, []byte("Or"))
	var newBuffer bytes.Buffer
	newBuffer.WriteString(` `)
	newBuffer.WriteString("WHERE")
	newBuffer.WriteString(` `)
	newBuffer.Write(sql)
	newBuffer.WriteString(` `)
	newBufferBytes := newBuffer.Bytes()
	newBuffer.Reset()
	return newBufferBytes, nil
}
type sqlArgTypeConv interface {
	Convert(arg interface{}) string
}
type sqlArgTypeConvert struct {}
func (it sqlArgTypeConvert) Convert(argValue interface{}) string {
	if argValue == nil {
		return "''"
	}
	switch argValue.(type) {
	case string:
		var argStr bytes.Buffer
		argStr.WriteString(`'`)
		argStr.WriteString(argValue.(string))
		argStr.WriteString(`'`)
		return argStr.String()
	case *string:
		v := argValue.(*string)
		if v == nil {
			return "''"
		}
		var argStr bytes.Buffer
		argStr.WriteString(`'`)
		argStr.WriteString(*v)
		argStr.WriteString(`'`)
		return argStr.String()
	case bool:
		if argValue.(bool) {
			return "true"
		} else {
			return "false"
		}
	case *bool:
		v := argValue.(*bool)
		if v == nil {
			return "''"
		}
		if *v {
			return "true"
		} else {
			return "false"
		}
	case time.Time:
		var argStr bytes.Buffer
		argStr.WriteString(`'`)
		argStr.WriteString(argValue.(time.Time).Format(`2006-01-02 15:04:05`))
		argStr.WriteString(`'`)
		return argStr.String()
	case *time.Time:
		timePtr := argValue.(*time.Time)
		if timePtr == nil {
			return "''"
		}
		var argStr bytes.Buffer
		argStr.WriteString(`'`)
		argStr.WriteString(timePtr.Format(`2006-01-02 15:04:05`))
		argStr.WriteString(`'`)
		return argStr.String()
	case int, int16, int32, int64, float32, float64:
		return fmt.Sprint(argValue)
	case *int:
		v := argValue.(*int)
		if v == nil {
			return ""
		}
		return fmt.Sprint(*v)
	case *int16:
		v := argValue.(*int16)
		if v == nil {
			return ""
		}
		return fmt.Sprint(*v)
	case *int32:
		v := argValue.(*int32)
		if v == nil {
			return ""
		}
		return fmt.Sprint(*v)
	case *int64:
		v := argValue.(*int64)
		if v == nil {
			return ""
		}
		return fmt.Sprint(*v)
	case *float32:
		v := argValue.(*float32)
		if v == nil {
			return ""
		}
		return fmt.Sprint(*v)
	case *float64:
		v := argValue.(*float64)
		if v == nil {
			return ""
		}
		return fmt.Sprint(*v)
	}
	return it.toString(argValue)
}
func (it sqlArgTypeConvert) toString(argValue interface{}) string {
	if argValue == nil {
		return ""
	}
	return fmt.Sprint(argValue)
}
type iExpression interface {
	Lexer(lexerArg string) (interface{}, error)
	Eval(lexerResult interface{}, arg interface{}, operation int) (interface{}, error)
	LexerAndEval(lexerArg string,arg interface{})  (interface{}, error)
}
type express struct {
	Proxy iExpression
}
func (it express) Parser(mapperXml []token) []iiNode {
	if it.Proxy == nil {
		panic("NodeParser need a *ExpressionEngineProxy{}!")
	}
	list := make([]iiNode,0)
	for _, item := range mapperXml {
		var nod iiNode
		typeString := reflect.TypeOf(item).String()
		if typeString == "*mbt.charData" {
			chard := item.(*charData)
			str := chard.Data
			str = strings.Replace(str, "\n", " ", -1)
			str = strings.Replace(str, "\t", " ", -1)
			str = strings.Trim(str, " ")
			str = " " + str
			n := nodeString{
				value:               str,
				t:                   nStr,
				expressMap:          findExpress(chard.Data), //表达式需要替换的string
				noConvertExpressMap: findRawExpressString(chard.Data),
				holder:              it,
			}
			if len(n.expressMap) == 0 {
				n.expressMap = nil
			}
			nod = &n
		} else if typeString == "*mbt.element" {
			v := item.(*element)
			childItems := v.Child
			switch v.Tag {
			case "if":
				n := nodeIf{
					t:      nIf,
					test:   v.SelectAttrValue("test", ""),
					child:  []iiNode{},
					holder: it,
				}
				if childItems != nil {
					childNodes := it.Parser(childItems)
					n.child = append(n.child, childNodes...)
				}
				nod = &n
				break
			case "trim":
				n := nodeTrim{
					t:               nTrim,
					prefix:          []byte(v.SelectAttrValue("prefix", "")),
					suffix:          []byte(v.SelectAttrValue("suffix", "")),
					prefixOverrides: []byte(v.SelectAttrValue("prefixOverrides", "")),
					suffixOverrides: []byte(v.SelectAttrValue("suffixOverrides", "")),
					child:           []iiNode{},
				}
				if childItems != nil {
					childNodes := it.Parser(childItems)
					n.child = append(n.child, childNodes...)
				}
				nod = &n
				break
			case "set":
				n := nodeTrim{
					t:     nTrim,
					child: []iiNode{},
					prefix:          []byte(" set "),
					suffix:          nil,
					prefixOverrides: []byte(","),
					suffixOverrides: []byte(","),
				}
				if childItems != nil {
					var childNodes = it.Parser(childItems)
					n.child = append(n.child, childNodes...)
				}
				nod = &n
				break
			case "foreach":
				n := nodeForeach{
					t:          nForeach,
					child:      []iiNode{},
					collection: v.SelectAttrValue("collection", ""),
					index:      v.SelectAttrValue("index", ""),
					item:       v.SelectAttrValue("item", ""),
					open:       v.SelectAttrValue("open", ""),
					close:      v.SelectAttrValue("close", ""),
					separator:  v.SelectAttrValue("separator", ""),
					holder:     it,
				}
				if childItems != nil {
					childNodes := it.Parser(childItems)
					n.child = append(n.child, childNodes...)
				}
				nod = &n
				break
			case "choose":
				n := nodeChoose{
					t:         nChoose,
					whenNodes: []iiNode{},
				}
				if childItems != nil {
					childNodes := it.Parser(childItems)
					for _, v := range childNodes {
						if v.Type() == nWhen {
							n.whenNodes = append(n.whenNodes, childNodes...)
						} else if v.Type() == nOtherwise {
							if n.otherwiseNode != nil {
								panic("element only support one Otherwise node!")
							}
							n.otherwiseNode = v
						} else if v.Type() == nStr {
							continue
						} else {
							panic("not support element type:" + v.Type().ToString())
						}
					}

				} else {
					n.whenNodes = nil
					n.otherwiseNode = nil
				}
				nod = &n
				break
			case "when":
				n := nodeWhen{
					t:      nOtherwise,
					child:  []iiNode{},
					test:   v.SelectAttrValue("test", ""),
					holder: it,
				}
				if childItems != nil {
					childNodes := it.Parser(childItems)
					n.child = append(n.child, childNodes...)
				}
				nod = &n
				break
			case "otherwise":
				n := nodeOtherwise{
					t:     nOtherwise,
					child: []iiNode{},
				}
				if childItems != nil {
					childNodes := it.Parser(childItems)
					n.child = append(n.child, childNodes...)
				}
				nod = &n
				break
			case "where":
				n := nodeWhere{
					t:     nWhere,
					child: []iiNode{},
				}
				if childItems != nil {
					childNodes := it.Parser(childItems)
					n.child = append(n.child, childNodes...)
				}
				nod = &n
				break
			case "bind":
				n := nodeBind{
					t:      nBind,
					value:  v.SelectAttrValue("value", ""),
					name:   v.SelectAttrValue("name", ""),
					holder: it,
				}
				nod = &n
			case "include":
				n := nodeInclude{
					t: nInclude,
				}
				if childItems != nil {
					childNodes := it.Parser(childItems)
					n.child = append(n.child, childNodes...)
				}
				nod = &n
			default:
				continue
			}
		} else {
			continue
		}
		if nod == nil {
			panic("node ni;")
		}
		list = append(list, nod)
	}
	return list
}

type Convert interface {
	Convert() string
	Inc()
	Get()int
}
type mysql struct {}
func (it *mysql) Convert() string {
	return " ? "
}
func (it *mysql)Inc() {}

func (it *mysql)Get()int  {
	return 0
}
type shenTong struct {
	sync.RWMutex
	counter int
}
func (s *shenTong) Convert() string {
	return fmt.Sprint(" :", s.Get(), " ")
}
func (s *shenTong) Inc() {
	s.Lock()
	defer s.Unlock()
	s.counter++
}
func (s *shenTong) Get() int {
	s.RLock()
	defer s.RUnlock()
	return s.counter
}
type oracle struct {
	sync.RWMutex
	counter int
}
func (it *oracle) Inc() {
	it.Lock()
	defer it.Unlock()
	it.counter++
}
func (it *oracle) Get() int {
	it.RLock()
	defer it.RUnlock()
	return it.counter
}
func (it *oracle) Convert() string {
	return fmt.Sprint(" :val", it.Get(), " ")
}
type postgreSQL struct {
	sync.RWMutex
	counter int
}
func (p *postgreSQL) Inc() {
	p.Lock()
	defer p.Unlock()
	p.counter++
}
func (p *postgreSQL) Get() int {
	p.RLock()
	defer p.RUnlock()
	return p.counter
}
func (p *postgreSQL) Convert() string {
	return fmt.Sprint(" $", p.Get(), " ")
}
func (it *Session)stmtConvert() Convert {
	switch it.driverName {
	case "mysql", "mymysql", "mssql", "sqlite3","sqlite","dm","gbase":
		return &mysql{}
	case "postgres","kingbase":
		return &postgreSQL{
			sync.RWMutex{},
			0,
		}
	case "oci8":
		return &oracle{sync.RWMutex{},
			0}
	case "shentong":
		return &shenTong{
			sync.RWMutex{},
			0,
		}
	default:
		driverType := it.driver[it.driverName]
		if driverType != nil {
			return driverType
		}else {
			it.log.SetPrefix("[Fatal] ")
			it.log.Fatalln(`un support driverName :`+it.driverName+"only support (mysql、mymysql、mssql、sqlite3、postgres、oci8")
		}
	}
	return nil
}
func (it *Session)slaveConvert() Convert {
	switch it.slaveDriver {
	case "mysql", "mymysql", "mssql", "sqlite3","sqlite","dm","gbase":
		return &mysql{}
	case "postgres","kingbase":
		return &postgreSQL{
			sync.RWMutex{},
			0,
		}
	case "oci8":
		return &oracle{sync.RWMutex{},
			0}
	case "shentong":
		return &shenTong{
			sync.RWMutex{},
			0,
		}
	default:
		driverType := it.driver[it.slaveDriver]
		if driverType != nil {
			return driverType
		}else {
			it.log.SetPrefix("[Fatal] ")
			it.log.Fatalln(`un support driverName :`+it.driverName+"only support (mysql、mymysql、mssql、sqlite3、postgres、oci8")
		}
	}
	return nil
}
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





