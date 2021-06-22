package mbt

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
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
	nForEach
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
	case nForEach:
		return "nFor"
	case nChoose:
		return "nSwitch"
	case nOtherwise:
		return "nDefault"
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
	Eval(env map[string]interface{}, array *[]interface{}, stmtConvert iConvert) ([]byte, error)
}
func doChildNodes(childNodes []iiNode, env map[string]interface{},array *[]interface{}, stmtConvert iConvert) ([]byte, error) {
	if childNodes == nil {
		return nil, nil
	}
	var sql bytes.Buffer
	for _, v := range childNodes {
		var r, e = v.Eval(env, array, stmtConvert)
		if e != nil {
			return nil, e
		}
		if r != nil {
			sql.Write(r)
		}
	}
	var bytes = sql.Bytes()
	sql.Reset()
	return bytes, nil
}
func replace(findStrS []string, data string, arg map[string]interface{}, engine iExpression, array *[]interface{}, indexConvert iConvert) (string, error) {
	for _, findStr := range findStrS {
		var argValue = arg[findStr]
		if argValue != nil {
			*array = append(*array, argValue)
		} else {
			var err error
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
		var evalData interface{}
		var argValue = arg[findStr]
		if argValue != nil {
			evalData = argValue
		} else {
			var err error
			evalData, err = engine.LexerAndEval(findStr, arg)
			if err != nil {
				return "", err
			}
		}
		var resultStr string
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
	finds := make([]string,0)
	var item []byte
	var lastIndex = -1
	var startIndex = -1
	var strBytes = []byte(str)
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
	strS := make([]string,0)
	for _, k := range finds {
		strS = append(strS, k)
	}
	return strS
}
func findRawExpressString(str string) []string {
	finds := make([]string,0)
	var item []byte
	var lastIndex = -1
	var startIndex = -1
	var strBytes = []byte(str)
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
	strS := make([]string,0)
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
func (it nodeString) Eval(env map[string]interface{}, array *[]interface{}, stmtConvert iConvert) ([]byte, error) {
	var data = it.value
	var err error
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
func (it nodeBind) Eval(env map[string]interface{}, array *[]interface{}, stmtConvert iConvert) ([]byte, error) {
	if it.name == "" {
		panic(`[Error] 元素 <bind name = ""> 名称不能为空!`)
	}
	if it.value == "" {
		env[it.name] = it.value
		return nil, nil
	}
	res, err := it.holder.Proxy.LexerAndEval(it.value, env)
	if err != nil {
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
func (it nodeChoose) Eval(env map[string]interface{}, array *[]interface{}, stmtConvert iConvert) ([]byte, error) {
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
	return nForEach
}
func (it nodeForeach) Eval(env map[string]interface{}, array *[]interface{}, stmtConvert iConvert) ([]byte, error) {
	if it.collection == "" {
		panic(`[Error] collection value can not be "" in <foreach collection=""> !`)
	}
	var tempSql bytes.Buffer
	var err error
	evalData, err := it.holder.Proxy.LexerAndEval(it.collection, env)
	if err != nil {
		return nil, err
	}
	var collectionValue = reflect.ValueOf(evalData)
	var kind = collectionValue.Kind()
	if kind == reflect.Invalid {
		return nil, errors.New(" collection value is invalid value!")
	}
	if kind != reflect.Slice && kind != reflect.Array && kind != reflect.Map {
		panic(`[Error] collection value must be a slice or map !`)
	}
	var collectionValueLen = collectionValue.Len()
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
		var mapKeys = collectionValue.MapKeys()
		var collectionKeyLen = len(mapKeys)
		if collectionKeyLen == 0 {
			return nil, nil
		}
		var tempArgMap = env
		for _, keyValue := range mapKeys {
			var key = keyValue.Interface()
			var collectionItem = collectionValue.MapIndex(keyValue)
			if it.item != "" {
				tempArgMap[it.item] = collectionItem.Interface()
			}
			tempArgMap[it.index] = key
			var r, err = doChildNodes(it.child, tempArgMap, array, stmtConvert)
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
		var tempArgMap = env
		for i := 0; i < collectionValueLen; i++ {
			var collectionItem = collectionValue.Index(i)
			if it.item != "" {
				tempArgMap[it.item] = collectionItem.Interface()
			}
			if it.index != "" {
				tempArgMap[it.index] = i
			}
			var r, err = doChildNodes(it.child, tempArgMap, array, stmtConvert)
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
	var newTempSql bytes.Buffer
	var tempSqlString = bytes.Trim(tempSql.Bytes(), it.separator)
	tempSql.Reset()
	newTempSql.WriteString(it.open)
	newTempSql.Write(tempSqlString)
	newTempSql.WriteString(it.close)
	var newTempSqlBytes = newTempSql.Bytes()
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
func (it nodeIf) Eval(env map[string]interface{}, array *[]interface{}, stmtConvert iConvert) ([]byte, error) {
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
func (it nodeInclude) Eval(env map[string]interface{}, array *[]interface{}, stmtConvert iConvert) ([]byte, error) {
	var sql, err = doChildNodes(it.child, env, array, stmtConvert)
	return sql, err
}
type nodeOtherwise struct {
	child []iiNode
	t     xmlNode
}
func (it nodeOtherwise) Type() xmlNode {
	return nOtherwise
}
func (it nodeOtherwise) Eval(env map[string]interface{}, array *[]interface{}, stmtConvert iConvert) ([]byte, error) {
	var r, e = doChildNodes(it.child, env, array, stmtConvert)
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
func (it nodeTrim) Eval(env map[string]interface{}, array *[]interface{}, stmtConvert iConvert) ([]byte, error) {
	var sql, err = doChildNodes(it.child, env, array, stmtConvert)
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
		var prefixOverridesArray = bytes.Split(it.prefixOverrides, []byte("|"))
		if len(prefixOverridesArray) > 0 {
			for _, v := range prefixOverridesArray {
				sql = bytes.TrimPrefix(sql, []byte(v))
			}
		}
	}
	if it.suffixOverrides != nil {
		var suffixOverrideArray = bytes.Split(it.suffixOverrides, []byte("|"))
		if len(suffixOverrideArray) > 0 {
			for _, v := range suffixOverrideArray {
				sql = bytes.TrimSuffix(sql, []byte(v))
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
func (it nodeWhen) Eval(env map[string]interface{}, array *[]interface{}, stmtConvert iConvert) ([]byte, error) {
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
func (it nodeWhere) Eval(env map[string]interface{}, array *[]interface{}, stmtConvert iConvert) ([]byte, error) {
	var sql, err = doChildNodes(it.child, env, array, stmtConvert)
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
		var v = argValue.(*string)
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
		var v = argValue.(*bool)
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
		argStr.WriteString(argValue.(time.Time).Format(adapterFormatDate))
		argStr.WriteString(`'`)
		return argStr.String()
	case *time.Time:
		var timePtr = argValue.(*time.Time)
		if timePtr == nil {
			return "''"
		}
		var argStr bytes.Buffer
		argStr.WriteString(`'`)
		argStr.WriteString(timePtr.Format(adapterFormatDate))
		argStr.WriteString(`'`)
		return argStr.String()
	case int, int16, int32, int64, float32, float64:
		return fmt.Sprint(argValue)
	case *int:
		var v = argValue.(*int)
		if v == nil {
			return ""
		}
		return fmt.Sprint(*v)
	case *int16:
		var v = argValue.(*int16)
		if v == nil {
			return ""
		}
		return fmt.Sprint(*v)
	case *int32:
		var v = argValue.(*int32)
		if v == nil {
			return ""
		}
		return fmt.Sprint(*v)
	case *int64:
		var v = argValue.(*int64)
		if v == nil {
			return ""
		}
		return fmt.Sprint(*v)
	case *float32:
		var v = argValue.(*float32)
		if v == nil {
			return ""
		}
		return fmt.Sprint(*v)
	case *float64:
		var v = argValue.(*float64)
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
		var typeString = reflect.TypeOf(item).String()
		if typeString == "*mbt.charData" {
			chard := item.(*charData)
			var str = chard.Data
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
			var v = item.(*element)
			var childItems = v.Child
			switch v.Tag {
			case "if":
				n := nodeIf{
					t:      nIf,
					test:   v.SelectAttrValue("test", ""),
					child:  []iiNode{},
					holder: it,
				}
				if childItems != nil {
					var childNodes = it.Parser(childItems)
					n.child = append(n.child, childNodes...)
				}
				nod = &n
				break
			case "trim":
				n := nodeTrim{
					t:               nTrim,
					prefix:          []byte(v.SelectAttrValue("prefix", "")),
					suffix:          []byte(v.SelectAttrValue("suffix", "")),
					prefixOverrides: []byte(v.SelectAttrValue("trimPrefix", "")),
					suffixOverrides: []byte(v.SelectAttrValue("trimSuffix", "")),
					child:           []iiNode{},
				}
				if childItems != nil {
					var childNodes = it.Parser(childItems)
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
			case "for":
				n := nodeForeach{
					t:          nForEach,
					child:      []iiNode{},
					collection: v.SelectAttrValue("list", ""),
					index:      v.SelectAttrValue("index", ""),
					item:       v.SelectAttrValue("item", ""),
					open:       v.SelectAttrValue("open", ""),
					close:      v.SelectAttrValue("close", ""),
					separator:  v.SelectAttrValue("separator", ""),
					holder:     it,
				}
				if childItems != nil {
					var childNodes = it.Parser(childItems)
					n.child = append(n.child, childNodes...)
				}
				nod = &n
				break
			case "switch":
				n := nodeChoose{
					t:         nChoose,
					whenNodes: []iiNode{},
				}
				if childItems != nil {
					var childNodes = it.Parser(childItems)
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
			case "case":
				n := nodeWhen{
					t:      nOtherwise,
					child:  []iiNode{},
					test:   v.SelectAttrValue("test", ""),
					holder: it,
				}
				if childItems != nil {
					var childNodes = it.Parser(childItems)
					n.child = append(n.child, childNodes...)
				}
				nod = &n
				break
			case "default":
				n := nodeOtherwise{
					t:     nOtherwise,
					child: []iiNode{},
				}
				if childItems != nil {
					var childNodes = it.Parser(childItems)
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
					var childNodes = it.Parser(childItems)
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
					var childNodes = it.Parser(childItems)
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

type iConvert interface {
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

func (it *oracle) Convert() string {
	return fmt.Sprint(" :val", it.Get(), " ")
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
func buildStmtConvert(driverType string) (iConvert, error) {
	switch driverType {
	case "mysql", "mymysql", "mssql", "sqlite3","sqlite","dm","gbase":
		return &mysql{}, nil
	case "postgres","kingbase":
		return &postgreSQL{
			sync.RWMutex{},
			0,
		}, nil
	case "oci8":
		return &oracle{sync.RWMutex{},
			0}, nil
	case "shentong":
		return &shenTong{
			sync.RWMutex{},
			0,
		}, nil
	default:
		panic(fmt.Sprint("[Error] un support dbName:", driverType, " only support: ", "mysql,", "mymysql,", "mssql,", "sqlite3,", "postgres,", "oci8"))
	}
}



