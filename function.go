package mbt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func (it *Engine)start(bean reflect.Value, name string) {
	xml, _ := ioutil.ReadFile(name)
	// 代码能够执行到这里 bean.Kind() 一定是 reflect.Ptr类型
	bt := bean.Type().Elem()
	be := bean.Elem()
	outPut := it.makeReturnTypeMap(be.Type())
	mapperTree := it.parseXml(name,xml)
	resultMaps := makeResultMaps(mapperTree)
	it.decodeTree(mapperTree, bt,name)
	methodXmlMap := it.makeMethodXmlMap(bt, mapperTree,name)
	it.proxyValue(be, func(funcField reflect.StructField, field reflect.Value) func(arg proxyArg) []reflect.Value {
		funcName := funcField.Name
		ret := outPut[funcName]
		m := methodXmlMap[funcName]
		var resultMap map[string]*resultProperty
		if funcName != sessionFunc {
			resultMapId := m.xml.SelectAttrValue(elementResultMap, "")
			if resultMapId != "" {
				resultMap = resultMaps[resultMapId]
			}
		}
		if funcName == sessionFunc {
			proxyFunc := func(arg proxyArg) []reflect.Value {
				var res *reflect.Value = nil
				returnV := reflect.New(*ret.Value)
				switch (*ret.Value).Kind() {
				case reflect.Map:
					returnV.Elem().Set(reflect.MakeMap(*ret.Value))
				case reflect.Slice:
					returnV.Elem().Set(reflect.MakeSlice(*ret.Value, 0, 0))
				}
				res = &returnV
				res.Elem().Set(reflect.ValueOf(it.s).Elem().Addr().Convert(*ret.Value))
				return buildReturnValue(ret, res)
			}
			return proxyFunc
		} else {
			proxyFunc := func(arg proxyArg) []reflect.Value {
				var res *reflect.Value = nil
				returnV := reflect.New(*ret.Value)
				switch (*ret.Value).Kind() {
				case reflect.Map:
					returnV.Elem().Set(reflect.MakeMap(*ret.Value))
				case reflect.Slice:
					returnV.Elem().Set(reflect.MakeSlice(*ret.Value, 0, 0))
				}
				res = &returnV
				it.exeMethodByXml(m.xml.Tag, arg, m.nodes, resultMap, res,bt.String()+"."+funcName+"() ")
				return buildReturnValue(ret, res)
			}
			return proxyFunc
		}
	})
}
func (it *Engine)makeReturnTypeMap(bean reflect.Type) (returnMap map[string]*returnValue) {
	returnMap = make(map[string]*returnValue)
	name := bean.String()
	for i := 0; i < bean.NumField(); i++ {
		fieldItem := bean.Field(i)
		funcType := fieldItem.Type
		funcName := fieldItem.Name
		funcKind := funcType.Kind()
		if funcKind != reflect.Func {
			if funcKind == reflect.Struct {
				childMap := it.makeReturnTypeMap(funcType)
				for k, v := range childMap {
					returnMap[k] = v
				}
			}
			continue
		}
		argsLen := funcType.NumIn()
		customLen := 0
		for j := 0; j < argsLen; j++ {
			inType := funcType.In(j)
			if inType.String() == mSessionPtr {
				it.log.SetPrefix("[Fatal] ")
				it.log.Fatalln(name+"."+funcName+"()"+" 的输入参数不能是 "+mSessionPtr+",只能是 "+mSession)
			}
			if isCustomStruct(inType) {
				customLen++
			}
		}
		if argsLen > 1 && customLen > 1 {
			it.log.SetPrefix("[Fatal] ")
			it.log.Fatalln(name +"."+ funcName + `() 这个函数结构体类型的输入参数有且只能有 1 个,现在它已经 > 1 个了! ([]Student这种输入参数可以有,但不能出现这种 func(s Student,u User)(int64,error)`)
		}
		numOut := funcType.NumOut()
		if numOut > 2 || numOut == 0 {
			it.log.SetPrefix("[Fatal] ")
			it.log.Fatalln(name + "." + funcName + "()' return num out must = 1 or = 2!")
		}
		for k := 0; k < numOut; k++ {
			outType := funcType.Out(k)
			outTypeK := outType.Kind()
			outTypeS := outType.String()
			if funcName != sessionFunc {
				if outTypeK == reflect.Ptr || (outTypeK == reflect.Interface && outTypeS != "error") {
					it.log.SetPrefix("[Fatal] ")
					it.log.Fatalln(name + "." + funcName + "()' return '" + outTypeS + "' can not be a 'ptr' or 'interface'!")
				}
			}
			ret := returnMap[funcName]
			if ret == nil {
				returnMap[funcName] = &returnValue{
					Index: -1,
					Num:      numOut,
				}
			}
			if outTypeS != "error" {
				returnMap[funcName].Index = k
				returnMap[funcName].Value = &outType
			} else {
				returnMap[funcName].Error = &outType
			}
		}
		if returnMap[funcName].Error == nil && funcName != sessionFunc{
			it.log.SetPrefix("[Fatal] ")
			it.log.Fatalln(name + "." + funcName + "()' must return an 'error'!")
		}
	}
	return returnMap
}
func isCustomStruct(value reflect.Type) bool {
	if value.Kind() == reflect.Struct && value.String() != mTime && value.String() != mTimePtr {
		return true
	} else {
		return false
	}
}
// ==============================================================================================================
func(it *Engine) makeMethodXmlMap(beanType reflect.Type, mapperTree map[string]*element,xmlName string) map[string]*mapper {
	methodXmlMap := make(map[string]*mapper)
	totalField := beanType.NumField()
	for i := 0; i < totalField; i++ {
		fieldItem := beanType.Field(i)
		if fieldItem.Type.Kind() == reflect.Func && fieldItem.Name != sessionFunc{
			mapperXml := it.findMapperXml(mapperTree, beanType.String(),fieldItem.Name,xmlName)
			methodXmlMap[fieldItem.Name] = &mapper{
				xml:   mapperXml,
				nodes: newNodeParser().Parser(mapperXml.Child),
			}
		}
	}
	return methodXmlMap
}
func (it *Engine)findMapperXml(mapperTree map[string]*element, beanName,methodName,xmlName string) *element {
	for _, mapperXml := range mapperTree {
		key := mapperXml.SelectAttrValue("id", "")
		if strings.EqualFold(key, methodName) {
			return mapperXml
		}
	}
	it.log.SetPrefix("[Fatal] ")
	it.log.Fatalln("在 "+ xmlName +" 文件中没有找到 "+ beanName + "." + methodName +"() 对应的 id 值 "+ methodName)
	return nil
}
func newNodeParser() express {
	return express{
		Proxy: &nodeExpress{},
	}
}
// ======================================= 将 []byte 类型的XML数据解析成结构体 ================================================
func expressSymbol(bytes *[]byte) {
	byteStr := string(*bytes)
	testRegex, _ := regexp.Compile(`test=".*"`)
	findList := testRegex.FindAllString(byteStr, -1)
	for _, findStr := range findList {
		newStr := findStr
		newStr = strings.Replace(newStr, "<", "&lt;", -1)
		newStr = strings.Replace(newStr, ">", "&gt;", -1)
		byteStr = strings.Replace(byteStr, findStr, newStr, -1)
	}
	*bytes = []byte(byteStr)
}
func (it *Engine)parseXml(xmlName string,bytes []byte) (items map[string]*element) {
	expressSymbol(&bytes)
	doc := newDocument()
	if err := doc.ReadFromBytes(bytes); err != nil {
		it.log.SetPrefix("[Fatal] ")
		it.log.Fatalln("解析 "+xmlName+" 文件错误,err=",err)
	}
	items = make(map[string]*element)
	root := doc.SelectElement(elementMapper)
	for _, s := range root.ChildElements() {
		if s.Tag == elementInsert ||
			s.Tag == elementDelete ||
			s.Tag == elementUpdate ||
			s.Tag == elementSelect ||
			s.Tag == elementResultMap ||
			s.Tag == elementSql ||
			s.Tag == elementInsertTemplate ||
			s.Tag == elementDeleteTemplate ||
			s.Tag == elementUpdateTemplate ||
			s.Tag == elementSelectTemplate {
			idValue := s.SelectAttrValue(iD, "")
			if idValue == "" {
				idValue = s.Tag
			}
			if idValue != "" {
				oldItem := items[idValue]
				if oldItem != nil {
					it.log.SetPrefix("[Fatal] ")
					it.log.Fatalln(xmlName+` 文件内的同一类 <` + s.Tag +`> 标签中，有且只能有一个 id = `+ idValue+ `! (即:id 的值在同一类标签中不能重复!)`)
				}
			}
			items[idValue] = s
		}
	}
	for _, mapperXml := range items {
		for _, v := range mapperXml.ChildElements() {
			it.includeElementReplace(v, &items,xmlName)
		}
	}
	return items
}
func (it *Engine)includeElementReplace(xml *element, xmlMap *map[string]*element,xmlName string) {
	if xml.Tag == elementInclude {
		ref := xml.SelectAttr("refid").Value
		if ref == "" {
			it.log.SetPrefix("[Fatal] ")
			it.log.Fatalln(xmlName+` 文件中标签 <include refid=""> 'refid' 不能为 ""`)
		}
		mapperXml := (*xmlMap)[ref]
		if mapperXml == nil {
			it.log.SetPrefix("[Fatal] ")
			it.log.Fatalln(xmlName+` 文件中标签 <includ refid="` + ref + `"> element can not find !`)
		}
		if xml != nil {
			(*xml).Child = mapperXml.Child
		}
	}
	if xml.Child != nil {
		for _, v := range xml.ChildElements() {
			it.includeElementReplace(v, xmlMap,xmlName)
		}
	}
}
// ========================================== 解析 xml文件中的 <Template></Template> 模版标签 ================================================
var equalOperator = []string{"/", "+", "-", "*", "**", "|", "^", "&", "%", "<", ">", ">=", "<=", " in ", " not in ", " or ", "||", " and ", "&&", "==", "!="}

type (
	logicDeleteData struct {
		Column        string
		Property      string
		LangType      string
		Enable        bool
		DeletedValue  string
		UndeleteValue string
	}
	versionData struct {
		Column   string
		Property string
		LangType string
	}
)
func upperFirst(fieldStr string) string {
	if fieldStr != "" {
		var fieldBytes = []byte(fieldStr)
		var fieldLength = len(fieldStr)
		fieldStr = strings.ToUpper(string(fieldBytes[:1])) + string(fieldBytes[1:fieldLength])
		fieldBytes = nil
	}
	return fieldStr
}
// 参数 tree肯定不为空,所以该方法内部不用对 tree判空!!!
func (it *Engine)decodeTree(tree map[string]*element, beanType reflect.Type,xmlName string){
	for _, v := range tree {
		var method *reflect.StructField
		if isMethodElement(v.Tag) {
			upperId := upperFirst(v.SelectAttrValue("id", ""))
			if upperId == "" {
				upperId = upperFirst(v.Tag)
			}
			m, _ := beanType.FieldByName(upperId)
			method = &m
		}
		oldChild := v.Child
		v.Child = []token{}
		it.decode(method, v, tree,xmlName)
		v.Child = append(v.Child, oldChild...)
		beanName := beanType.String()
		if it.printXml {
			s := "================输出 " + beanName + "." + v.SelectAttrValue("id", "") +"()"+" 对应的 xml 标签 ============\n"
			printElement(v, &s)
			println(s)//log.Println(s)这里可以将s输出到日志中
		}
	}
}
func printElement(ele *element, v *string) {
	*v += "<" + ele.Tag + " "
	for _, item := range ele.Attr {
		*v += item.Key + "=\"" + item.Value + "\""
	}
	*v += " >"
	if ele.Child != nil && len(ele.Child) != 0 {
		for _, item := range ele.Child {
			var typeString = reflect.TypeOf(item).String()
			if typeString == "*mbt.element"{
				str := ""
				printElement(item.(*element), &str)
				*v += str
			} else if typeString == "*mbt.charData"{
				*v += "" + item.(*charData).Data
			}
		}
	}
	*v += "</" + ele.Tag + ">\n"
}
func (it *Engine)decode(method *reflect.StructField, mapper *element, tree map[string]*element,xmlName string){
	switch mapper.Tag {
	case elementSelectTemplate:
		mapper.Tag = elementSelect
		id := mapper.SelectAttrValue("id", "")
		if id == "" {
			mapper.CreateAttr("id", elementSelectTemplate)
		}
		tables := mapper.SelectAttrValue("table", "")
		columns := mapper.SelectAttrValue("column", "")
		wheres := mapper.SelectAttrValue("where", "")
		resultMap := mapper.SelectAttrValue("resultMap", "")
		if resultMap == "" {
			resultMap = "BaseResultMap"
		}
		resultMapData := tree[resultMap]
		if resultMapData == nil {
			it.log.SetPrefix("[Fatal] ")
			it.log.Fatalln(xmlName+"TemplateDecoder", "resultMap not define! id = ", resultMap)
		}
		it.checkTablesValue(mapper, &tables, resultMapData,xmlName)
		logic := it.decodeLogicDelete(resultMapData,xmlName)
		var sql bytes.Buffer
		sql.WriteString("select ")
		if columns == "" {
			columns = "*"
		}
		sql.WriteString(columns)
		sql.WriteString(" from ")
		sql.WriteString(tables)
		if len(wheres) > 0 {
			mapper.Child = append(mapper.Child, &charData{
				Data: sql.String(),
			})
			decodeWheres(wheres, mapper, logic, nil)
		}
		break
	case elementInsertTemplate:
		mapper.Tag = elementInsert
		id := mapper.SelectAttrValue("id", "")
		if id == "" {
			mapper.CreateAttr("id", elementInsertTemplate)
		}
		tables := mapper.SelectAttrValue("table", "")
		inserts := mapper.SelectAttrValue("insert", "")
		resultMap := mapper.SelectAttrValue("resultMap", "")
		if resultMap == "" {
			resultMap = "BaseResultMap"
		}
		if inserts == "" {
			inserts = "*?*"
		}
		resultMapData := tree[resultMap]
		if resultMapData == nil {
			it.log.SetPrefix("[Fatal] ")
			it.log.Fatalln(xmlName+"TemplateDecoder", "resultMap not define! id = ", resultMap)
		}
		it.checkTablesValue(mapper, &tables, resultMapData,xmlName)
		logic := it.decodeLogicDelete(resultMapData,xmlName)
		collectionName := decodeCollectionName(method)
		var sql bytes.Buffer
		sql.WriteString("insert into ")
		sql.WriteString(tables)
		mapper.Child = append(mapper.Child, &charData{
			Data: sql.String(),
		})
		var trimColumn = element{
			Tag: elementTrim,
			Attr: []attr{
				{Key: "prefix", Value: "("},
				{Key: "suffix", Value: ")"},
				{Key: "trimSuffix", Value: ","},
			},
			Child: []token{},
		}
		if collectionName != "" {
			for _, v := range resultMapData.ChildElements() {
				if inserts == "*" || inserts == "*?*" {
					trimColumn.Child = append(trimColumn.Child, &charData{
						Data: v.SelectAttrValue("column", "") + ",",
					})
				}
			}
		} else {
			for _, v := range resultMapData.ChildElements() {
				if collectionName == "" && inserts == "*?*" {
					trimColumn.Child = append(trimColumn.Child, &element{
						Tag: elementIf,
						Attr: []attr{
							{Key: "test", Value: makeIfNotNull(v.SelectAttrValue("column", ""))},
						},
						Child: []token{
							&charData{
								Data: v.SelectAttrValue("column", "") + ",",
							},
						},
					})
				} else if inserts == "*" {
					trimColumn.Child = append(trimColumn.Child, &charData{
						Data: v.SelectAttrValue("column", "") + ",",
					})
				}
			}
		}
		mapper.Child = append(mapper.Child, &trimColumn)
		var tempElement = element{
			Tag:   elementTrim,
			Attr:  []attr{{Key: "prefix", Value: "values ("}, {Key: "suffix", Value: ")"}, {Key: "trimSuffix", Value: ","}},
			Child: []token{},
		}
		if collectionName == "" {
			for _, v := range resultMapData.ChildElements() {
				if logic.Enable && v.SelectAttrValue("column", "") == logic.Property {
					tempElement.Child = append(tempElement.Child, &charData{
						Data: logic.UndeleteValue + ",",
					})
					continue
				}
				if inserts == "*?*" {
					tempElement.Child = append(tempElement.Child, &element{
						Tag:  elementIf,
						Attr: []attr{{Key: "test", Value: makeIfNotNull(v.SelectAttrValue("column", ""))}},
						Child: []token{
							&charData{
								Data: "#{" + v.SelectAttrValue("column", "") + "},",
							},
						},
					})
				} else if inserts == "*" {
					tempElement.Child = append(tempElement.Child, &charData{
						Data: "#{" + v.SelectAttrValue("column", "") + "},",
					})
				}
			}
		} else {
			tempElement.Attr = []attr{}
			tempElement.Tag = elementForeach
			tempElement.Attr = []attr{{Key: "open", Value: "values "}, {Key: "close", Value: ""}, {Key: "separator", Value: ","}, {Key: "list", Value: collectionName}}
			tempElement.Child = []token{}
			for index, v := range resultMapData.ChildElements() {
				var prefix = ""
				if index == 0 {
					prefix = "("
				}
				var defProperty = v.SelectAttrValue("column", "")
				if method != nil {
					for i := 0; i < method.Type.NumIn(); i++ {
						var argItem = method.Type.In(i)
						if argItem.Kind() == reflect.Ptr {
							argItem = argItem.Elem()
						}
						if argItem.Kind() == reflect.Slice || argItem.Kind() == reflect.Array {
							argItem = argItem.Elem()
						}
						if argItem.Kind() == reflect.Struct {
							for k := 0; k < argItem.NumField(); k++ {
								var argStructField = argItem.Field(k)
								var js = argStructField.Tag.Get("json")
								if strings.Index(js, ",") != -1 {
									js = strings.Split(js, ",")[0]
								}
								if strings.ToLower(strings.Replace(defProperty, "_", "", -1)) ==
									strings.ToLower(strings.Replace(argStructField.Name, "_", "", -1)) ||
									js == defProperty {
									defProperty = argStructField.Name
								}
							}
						}
					}
				}
				var value = prefix + "#{" + "item." + defProperty + "}"
				if logic.Enable && v.SelectAttrValue("column", "") == logic.Property {
					value = `'` + logic.UndeleteValue + "'"
				}
				if index+1 == len(resultMapData.ChildElements()) {
					value += ")"
				} else {
					value += ","
				}
				tempElement.Child = append(tempElement.Child, &charData{
					Data: value,
				})
			}
		}
		mapper.Child = append(mapper.Child, &tempElement)
		break
	case elementUpdateTemplate:
		mapper.Tag = elementUpdate
		id := mapper.SelectAttrValue("id", "")
		if id == "" {
			mapper.CreateAttr("id", elementUpdateTemplate)
		}
		tables := mapper.SelectAttrValue("table", "")
		columns := mapper.SelectAttrValue("set", "")
		wheres := mapper.SelectAttrValue("where", "")
		resultMap := mapper.SelectAttrValue("resultMap", "")
		if resultMap == "" {
			resultMap = "BaseResultMap"
		}
		resultMapData := tree[resultMap]
		if resultMapData == nil {
			it.log.SetPrefix("[Fatal] ")
			it.log.Fatalln(xmlName+"TemplateDecoder", "resultMap not define! id = ", resultMap)
		}
		it.checkTablesValue(mapper, &tables, resultMapData,xmlName)
		logic := it.decodeLogicDelete(resultMapData,xmlName)
		version := it.decodeVersionData(resultMapData,xmlName)
		var sql bytes.Buffer
		sql.WriteString("update ")
		sql.WriteString(tables)
		sql.WriteString(" set ")
		if columns == "" {
			mapper.Child = append(mapper.Child, &charData{
				Data: sql.String(),
			})
			sql.Reset()
			for _, v := range resultMapData.ChildElements() {
				if v.Tag == "id" {

				} else {
					if v.SelectAttrValue("version_enable", "") == "true" {
						continue
					}
					columns += v.SelectAttrValue("column", "") + "?" + v.SelectAttrValue("column", "") + " = #{" + v.SelectAttrValue("column", "") + "},"
				}
			}
			columns = strings.Trim(columns, ",")
			decodeSets(columns, mapper, logicDeleteData{}, version)
		} else {
			mapper.Child = append(mapper.Child, &charData{
				Data: sql.String(),
			})
			sql.Reset()
			decodeSets(columns, mapper, logicDeleteData{}, version)
		}
		if len(wheres) > 0 || logic.Enable {
			mapper.Child = append(mapper.Child, &charData{
				Data: sql.String(),
			})
			decodeWheres(wheres, mapper, logic, version)
		}
		break
	case elementDeleteTemplate:
		mapper.Tag = elementDelete
		id := mapper.SelectAttrValue("id", "")
		if id == "" {
			mapper.CreateAttr("id", elementDeleteTemplate)
		}
		tables := mapper.SelectAttrValue("table", "")
		wheres := mapper.SelectAttrValue("where", "")
		resultMap := mapper.SelectAttrValue("resultMap", "")
		if resultMap == "" {
			resultMap = "BaseResultMap"
		}
		resultMapData := tree[resultMap]
		if resultMapData == nil {
			it.log.SetPrefix("[Fatal] ")
			it.log.Fatalln(xmlName+"TemplateDecoder", "resultMap not define! id = ", resultMap)
		}
		it.checkTablesValue(mapper, &tables, resultMapData,xmlName)
		logic := it.decodeLogicDelete(resultMapData,xmlName)
		if logic.Enable {
			var sql bytes.Buffer
			sql.WriteString("update ")
			sql.WriteString(tables)
			sql.WriteString(" set ")
			mapper.Child = append(mapper.Child, &charData{
				Data: sql.String(),
			})
			sql.Reset()
			decodeSets("", mapper, logic, nil)
			if len(wheres) > 0 {
				mapper.Child = append(mapper.Child, &charData{
					Data: sql.String(),
				})
				decodeWheres(wheres, mapper, logic, nil)
			}
			break
		} else {
			var sql bytes.Buffer
			sql.WriteString("delete from ")
			sql.WriteString(tables)
			if len(wheres) > 0 {
				mapper.Child = append(mapper.Child, &charData{
					Data: sql.String(),
				})
				decodeWheres(wheres, mapper, logicDeleteData{}, nil)
			}
		}
	}
}
func (it *Engine)checkTablesValue(mapper *element, tables *string, resultMapData *element,xmlName string) {
	if *tables == "" {
		*tables = resultMapData.SelectAttrValue("table", "")
		if *tables == "" {
			it.log.SetPrefix("[Fatal] ")
			it.log.Fatalln(xmlName+"[TemplateDecoder] 属性 'table' 不能为空! 需要定义在 <resultMap> 或者 <" + mapper.Tag + "Template>中,mapper id=" + mapper.SelectAttrValue("id", ""))
		}
	}
}
func decodeWheres(arg string, mapper *element, logic logicDeleteData, version *versionData) {
	whereRoot := &element{
		Tag:   elementWhere,
		Attr:  []attr{},
		Child: []token{},
	}
	if logic.Enable == true {
			appendAdd := ""
			item := &charData{
			Data: appendAdd + logic.Column + " = " + logic.UndeleteValue,
		}
		whereRoot.Child = append(whereRoot.Child, item)
	}
	if version != nil {
		appendAdd := ""
		if len(whereRoot.Child) >= 1 {
			appendAdd = " and "
		}
			item := &charData{
			Data: appendAdd + version.Column + " = #{" + version.Property + "}",
		}
		whereRoot.Child = append(whereRoot.Child, item)
	}
	wheres := strings.Split(arg, ",")
	for index, v := range wheres {
		if v == "" || strings.Trim(v, " ") == "" {
			continue
		}
		expressions := strings.Split(v, "?")
		appendAdd := ""
		if index >= 1 || len(whereRoot.Child) > 0 {
			appendAdd = " and "
		}
		var item token
		if len(expressions) > 1 {
			var newWheres bytes.Buffer
			newWheres.WriteString(expressions[1])
			item = &element{
				Tag:   elementIf,
				Attr:  []attr{{Key: "test", Value: makeIfNotNull(expressions[0])}},
				Child: []token{&charData{Data: appendAdd + newWheres.String()}},
			}
		} else {
			var newWheres bytes.Buffer
			newWheres.WriteString(appendAdd)
			newWheres.WriteString(v)
			item = &charData{
				Data: newWheres.String(),
			}
		}
		whereRoot.Child = append(whereRoot.Child, item)
	}
	mapper.Child = append(mapper.Child, whereRoot)
}
func decodeSets(arg string, mapper *element, logic logicDeleteData, version *versionData) {
	sets := strings.Split(arg, ",")
	for index, v := range sets {
		if v == "" {
			continue
		}
		expressions := strings.Split(v, "?")
		if len(expressions) > 1 {
			var newWheres bytes.Buffer
			if index > 0 {
				newWheres.WriteString(",")
			}
			newWheres.WriteString(expressions[1])
				item := &element{
				Tag:  elementIf,
				Attr: []attr{{Key: "test", Value: makeIfNotNull(expressions[0])}},
			}
			item.SetText(newWheres.String())
			mapper.Child = append(mapper.Child, item)
		} else {
			var newWheres bytes.Buffer
			if index > 0 {
				newWheres.WriteString(",")
			}
			newWheres.WriteString(v)
				item := &charData{
				Data: newWheres.String(),
			}
			mapper.Child = append(mapper.Child, item)
		}
	}
	if logic.Enable == true {
		appendAdd := ""
		if len(sets) >= 1 && arg != "" {
			appendAdd = ","
		}
		item := &charData{
			Data: appendAdd + logic.Column + " = " + logic.DeletedValue,
		}
		mapper.Child = append(mapper.Child, item)
	}
	if version != nil {
		appendAdd := ""
		if len(sets) >= 1 && arg != "" {
			appendAdd = ","
		}
		item := &charData{
			Data: appendAdd + version.Column + " = #{" + version.Property + "+1}",
		}
		mapper.Child = append(mapper.Child, item)
	}
}
func makeIfNotNull(arg string) string {
	for _, v := range equalOperator {
		if v == "" {
			continue
		}
		if strings.Contains(arg, v) {
			return arg
		}
	}
	return arg + ` != nil`
}
func (it *Engine)decodeLogicDelete(xml *element,xmlName string) logicDeleteData {
	if xml == nil || len(xml.Child) == 0 {
		return logicDeleteData{}
	}
	logicData := logicDeleteData{}
	for _, v := range xml.ChildElements() {
		if v.SelectAttrValue("logic_enable", "") == "true" {
			logicData.Enable = true
			logicData.DeletedValue = v.SelectAttrValue("logic_deleted", "")
			logicData.UndeleteValue = v.SelectAttrValue("logic_undelete", "")
			logicData.Column = v.SelectAttrValue("column", "")
			logicData.Property = v.SelectAttrValue("column", "")
			logicData.LangType = v.SelectAttrValue("langType", "")
			if logicData.DeletedValue == "" {
				it.log.SetPrefix("[Fatal] ")
				it.log.Fatalln(xmlName+"TemplateDecoder", `<resultMap> logic_deleted="" can't be empty !`)
			}
			if logicData.UndeleteValue == "" {
				it.log.SetPrefix("[Fatal] ")
				it.log.Fatalln(xmlName+"TemplateDecoder", `<resultMap> logic_undelete="" can't be empty !`)
			}
			if logicData.UndeleteValue == logicData.DeletedValue {
				it.log.SetPrefix("[Fatal] ")
				it.log.Fatalln(xmlName+"TemplateDecoder", `<resultMap> logic_deleted value can't be logic_undelete value!`)
			}
			break
		}
	}
	return logicData
}
func (it *Engine)decodeVersionData(xml *element,xmlName string) *versionData {
	if xml == nil || len(xml.Child) == 0 {
		return nil
	}
	for _, v := range xml.ChildElements() {
		if v.SelectAttrValue("version_enable", "") == "true" {
			version := versionData{}
			version.Column = v.SelectAttrValue("column", "")
			version.Property = v.SelectAttrValue("column", "")
			version.LangType = v.SelectAttrValue("langType", "")
			if !(strings.Contains(version.LangType, "int") || strings.Contains(version.LangType, "time.Time")) {
				it.log.SetPrefix("[Fatal] ")
				it.log.Fatalln(xmlName+"TemplateDecoder", `version_enable only support int...,time.Time... number type!`)
			}
			return &version
		}
	}
	return nil
}
func decodeCollectionName(method *reflect.StructField) string {
	var collection string
	if method != nil {
		numIn := method.Type.NumIn()
		for i := 0; i < numIn; i++ {
			var itemType = method.Type.In(i)
			if itemType.Kind() == reflect.Slice || itemType.Kind() == reflect.Array {
				var params = method.Tag.Get("arg")
				var args = strings.Split(params, ",")
				if params == "" || args == nil || len(args) == 0 {
					collection = defaultOneArg + strconv.Itoa(i)
				} else {
					if args[i] == "" {
						collection = defaultOneArg + strconv.Itoa(i)
					} else {
						collection = args[i]
					}
				}
				if collection != "" {
					return collection
				}
			}
		}
	}
	return collection
}
// =======================================
func makeResultMaps(xml map[string]*element) map[string]map[string]*resultProperty {
	resultMaps := make(map[string]map[string]*resultProperty)
	for _, item := range xml {
		xmlItem := item
		if xmlItem.Tag == elementResultMap {
			resultPropertyMap := make(map[string]*resultProperty)
			for _, elementItem :=range xmlItem.ChildElements() {
				property := resultProperty{
					XMLName:  elementItem.Tag,
					Column:   elementItem.SelectAttrValue("column", ""),
					LangType: elementItem.SelectAttrValue("langType", ""),
				}
				resultPropertyMap[property.Column] = &property
			}
			resultMaps[xmlItem.SelectAttrValue("id", "")] = resultPropertyMap
		}
	}
	return resultMaps
}
func buildReturnValue(ptr *returnValue, value *reflect.Value) []reflect.Value {
	list := make([]reflect.Value, ptr.Num)
	for k, _ := range list {
		if k == ptr.Index {
			if value != nil {
				list[k] = (*value).Elem()
			}
		} else {
			list[k] = reflect.Zero(*ptr.Error)
		}
	}
	return list
}
func printArray(array []interface{}) string {
	return strings.Replace(fmt.Sprint(array), " ", ",", -1)
}
func (it *Engine)exeMethodByXml(elementType elementType, proxyArg proxyArg, nodes []iiNode, resultMap map[string]*resultProperty, returnValue *reflect.Value,name string){
	var s Session
	 s = findArgSession(proxyArg)
	 if s == nil {
		 s = it.get(goroutineID())
		 if s == nil {
			 s = it.s
		 }
	 }
	convert := s.stmtConvert()
	array := make([]interface{},0)
	sql := it.buildSql(proxyArg, nodes,&array, convert,name)
	if elementType == elementSelect {
		res, err := s.queryPrepare(sql, array...)
		if err != nil {
			it.log.SetPrefix("[Fatal] ")
			it.log.Fatalln(fmt.Sprintf(name+" [%s] error == %s",s.id(),err.Error()))
		}
		if it.printSql {
			it.log.Println(name+"[",s.id(),"] Query ==> "+sql)
			it.log.Println(name+"[",s.id(),"] Args  ==> "+printArray(array))
		}
		defer func() {
			if it.printSql {
				RowsAffected := "0"
				if res != nil {
					RowsAffected = strconv.Itoa(len(res))
				}
				it.log.Println(name+"[", s.id(), "] ReturnRows <== "+RowsAffected)
			}
		}()
		it.decodeSqlResult(resultMap, res, returnValue.Interface(),name)
	} else {
		res, err := s.execPrepare(sql, array...)
		if err != nil {
			it.log.SetPrefix("[Fatal] ")
			it.log.Fatalln(fmt.Sprintf(name+" [%s] error == %s",s.id(),err.Error()))
		}
		if it.printSql {
			it.log.Println(name+"[", s.id(), "] Exec ==> "+sql)
			it.log.Println(name+"[", s.id(), "] Args ==> "+printArray(array))
		}
		defer func() {
			if it.printSql {
				RowsAffected := "0"
				if res != nil {
					RowsAffected = strconv.FormatInt(res.RowsAffected, 10)
				}
				it.log.Println(name+"[", s.id(), "] RowsAffected <== "+RowsAffected)
			}
		}()
		returnValue.Elem().SetInt(res.RowsAffected)
	}
}
func findArgSession(proxyArg proxyArg)Session{
	for _, arg := range proxyArg.Args {
		argInterface := arg.Interface()
		argK := arg.Kind()
		argS := arg.Type().String()
		if argK == reflect.Interface && argInterface != nil && argS == mSession {
			return argInterface.(Session)
		}
	}
	return nil
}
func lowerFirst(fieldStr string) string {
	if fieldStr != "" {
		fieldBytes := []byte(fieldStr)
		fieldLength := len(fieldStr)
		fieldStr = strings.ToLower(string(fieldBytes[:1])) + string(fieldBytes[1:fieldLength])
		fieldBytes = nil
	}
	return fieldStr
}
func (it *Engine)buildSql(proxyArg proxyArg, nodes []iiNode, array *[]interface{}, stmtConvert iConvert,name string) string{
	paramMap := make(map[string]interface{})
	tagArgsLen := proxyArg.TagArgsLen
	argsLen := proxyArg.ArgsLen
	customLen := 0
	customIndex := -1
	for argIndex, arg := range proxyArg.Args {
		argInterface := arg.Interface()
		argK := arg.Kind()
		argT := arg.Type()
		if argK == reflect.Ptr && arg.IsNil() == false && argInterface != nil && argT.String() == mSessionPtr {
			if argsLen > 0 {
				argsLen--
			}
			if tagArgsLen > 0 {
				tagArgsLen--
			}
			continue
		} else if argInterface != nil && argK == reflect.Interface {
			continue
		}
		if isCustomStruct(argT) {
			customLen++
			customIndex = argIndex
		}
		if tagArgsLen > 0 && argIndex < tagArgsLen && proxyArg.TagArgs[argIndex].Name != "" {
			//插入2份参数，兼容大小写不敏感的参数
			lowerKey := lowerFirst(proxyArg.TagArgs[argIndex].Name)
			upperKey := upperFirst(proxyArg.TagArgs[argIndex].Name)
			paramMap[lowerKey] = argInterface
			paramMap[upperKey] = argInterface
		} else {
			paramMap[defaultOneArg+strconv.Itoa(argIndex)] = argInterface
		}
	}
	if customLen == 1 && customIndex != -1 {
		var tag *tagArg
		if proxyArg.TagArgsLen == 1 {
			tag = &proxyArg.TagArgs[0]
		}
		paramMap = it.scanStructArgFields(proxyArg.Args[customIndex], tag,name)
	}
	return it.sqlBuild(paramMap, nodes, array, stmtConvert,name)
}
func (it *Engine)sqlBuild(args map[string]interface{}, node []iiNode, array *[]interface{}, stmtConvert iConvert,name string)string{
	sql, err := doChildNodes(node, args, array, stmtConvert)
	if err != nil {
		it.log.SetPrefix("[Fatal] ")
		it.log.Fatalln(name+err.Error())
	}
	return string(sql)
}
func (it *Engine)scanStructArgFields(v reflect.Value, tag *tagArg,name string) map[string]interface{} {
	t := v.Type()
	parameters := make(map[string]interface{})
	if v.Kind() == reflect.Ptr {
		if v.IsNil() == true {
			return parameters
		}
		v = v.Elem()
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		it.log.SetPrefix("[Fatal] ")
		it.log.Fatalln(name +`the scanParameterBean() arg is not a struct type!,type =` + t.String())
	}
	structArg := make(map[string]interface{})
	for i := 0; i < t.NumField(); i++ {
		typeValue := t.Field(i)
		field := v.Field(i)
		var obj interface{}
		if field.CanInterface() {
			obj = field.Interface()
		}
		jsonKey := typeValue.Tag.Get(`json`)
		if strings.Index(jsonKey, ",") != -1 {
			jsonKey = strings.Split(jsonKey, ",")[0]
		}
		if jsonKey != "" {
			parameters[jsonKey] = obj
			structArg[jsonKey] = obj
			parameters[typeValue.Name] = obj
			structArg[typeValue.Name] = obj
		} else {
			parameters[typeValue.Name] = obj
			structArg[typeValue.Name] = obj
		}
	}
	if tag != nil && parameters[tag.Name] == nil {
		parameters[tag.Name] = structArg
	}
	return parameters
}
func (it *Engine)proxyValue(v reflect.Value, buildFunc func(funcField reflect.StructField, field reflect.Value) func(arg proxyArg) []reflect.Value) {
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		ft := f.Type()
		ftk := ft.Kind()
		sf := v.Type().Field(i)
		if ftk == reflect.Ptr{
			ft = ft.Elem()
		}
		if f.CanSet() {
			switch ftk {
			case reflect.Struct:
				it.proxyValue(f, buildFunc)
			case reflect.Func:
				it.buildRemoteMethod(v.Type().String(),f, ft, sf, buildFunc(sf, f))
			}
		}
	}
	v.Set(v)
}
func (it *Engine)buildRemoteMethod(name string,f reflect.Value, ft reflect.Type, sf reflect.StructField, proxyFunc func(arg proxyArg) []reflect.Value) {
	var (
		tagParams []string
		num = ft.NumIn()
		tagArgs = make([]tagArg, 0)
	)
	args := sf.Tag.Get(`arg`)
	if args == ""{
		for i := 0;i<num;i++ {
			fti := ft.In(i)
			ftk := fti.Kind()
			if fti.String() == mSession || ftk == reflect.Struct || ftk == reflect.Slice && fti.Elem().Kind() == reflect.Struct{
				continue
			}else {
				it.log.SetPrefix("[Fatal] ")
				it.log.Fatalln(name+"."+sf.Name+"()"+` 上的 tag "arg:" 的值的个数 != `+name+"."+sf.Name+"()"+` 的输入参数的个数!!`)
			}
		}
	}else {
		tagParams = strings.Split(args, `,`)
		tagParamsLen := len(tagParams)
		if tagParamsLen != 0 {
			for index, v := range tagParams {
				tag := tagArg{
					Index: index,
					Name:  v,
				}
				tagArgs = append(tagArgs, tag)
			}
		}
		tagArgsLen := len(tagArgs)
		if tagArgsLen > 0 && num != tagArgsLen{
			it.log.SetPrefix("[Fatal] ")
			it.log.Fatalln(name+"."+sf.Name+"()"+` 上的 tag "arg:" 的值的个数 != `+name+"."+sf.Name+"()"+` 的输入参数的个数!!`)
		}
	}
	fn := func(args []reflect.Value) (results []reflect.Value) {
		proxyResults := proxyFunc(newArg(tagArgs, args))
		for _, returnV := range proxyResults {
			results = append(results, returnV)
		}
		return results
	}
	f.Set(reflect.MakeFunc(ft, fn))
	tagParams = nil
}
func (it *Engine)decodeSqlResult(resultMap map[string]*resultProperty, sqlResult []map[string][]byte, result interface{},name string){
	if sqlResult == nil || result == nil {
		return
	}
	resultV := reflect.ValueOf(result).Elem()
	value := make([]byte,0)
	sqlResultLen := len(sqlResult)
	if sqlResultLen == 0 {
		return
	}
	if !isArray(resultV.Kind()) {
		if sqlResultLen > 1 {
			it.log.SetPrefix("[Fatal] ")
			it.log.Fatalln(name+"SqlResultDecoder Decode one result,but find database result size find > 1 !")
		}
		if isBasicType(resultV.Type()) {
			for _, s := range sqlResult[0] {
				var b = strings.Builder{}
				if resultV.Kind() == reflect.String || (resultV.Kind() == reflect.Struct) {
					b.WriteString("\"")
					b.Write(s)
					b.WriteString("\"")
				} else {
					b.Write(s)
				}
				value = []byte(b.String())
				break
			}
		} else {
			structMap := makeStructMap(resultV.Type())
			value = makeJsonObjBytes(resultMap, sqlResult[0], structMap)
		}
	} else {
		if resultV.Type().Kind() != reflect.Array && resultV.Type().Kind() != reflect.Slice {
			it.log.SetPrefix("[Fatal] ")
			it.log.Fatalln(name+"SqlResultDecoder decode type not an struct array or slice!")
		}
		resultVItemType := resultV.Type().Elem()
		structMap := makeStructMap(resultVItemType)
		done := len(sqlResult) - 1
		index := 0
		jsonData := strings.Builder{}
		jsonData.WriteString("[")
		for _, v := range sqlResult {
			jsonData.Write(makeJsonObjBytes(resultMap, v, structMap))
			if index < done {
				jsonData.WriteString(",")
			}
			index += 1
		}
		jsonData.WriteString("]")
		value = []byte(jsonData.String())
	}
	err := json.Unmarshal(value, result)
	if err != nil {
		it.log.SetPrefix("[Fatal] ")
		it.log.Fatalln(name+err.Error())
	}
}
func makeStructMap(itemType reflect.Type)map[string]*reflect.Type{
	if itemType.Kind() != reflect.Struct {
		return nil
	}
	structMap := map[string]*reflect.Type{}
	for i := 0; i < itemType.NumField(); i++ {
		item := itemType.Field(i)
		structMap[strings.ToLower(item.Tag.Get("json"))] = &item.Type
	}
	return structMap
}
func makeJsonObjBytes(resultMap map[string]*resultProperty, sqlData map[string][]byte, structMap map[string]*reflect.Type) []byte {
	jsonData := strings.Builder{}
	jsonData.WriteString("{")
	done := len(sqlData) - 1
	index := 0
	for k, sqlV := range sqlData {
		jsonData.WriteString("\"")
		jsonData.WriteString(k)
		jsonData.WriteString("\":")
		isStringType := false
		fetched := true
		if resultMap != nil {
			resultMapItem := resultMap[k]
			if resultMapItem != nil && (resultMapItem.LangType == "string" || resultMapItem.LangType == "time.Time") {
				isStringType = true
			}
			if resultMapItem == nil {
				fetched = false
			}
		} else if structMap != nil {
			v := structMap[strings.ToLower(k)]
			if v != nil {
				if (*v).Kind() == reflect.String || (*v).String() == "time.Time" {
					isStringType = true
				}
			}
			if v == nil {
				fetched = false
			}
		} else {
			isStringType = true
		}
		if fetched {
			if isStringType {
				jsonData.WriteString("\"")
				jsonData.WriteString(encodeStringValue(sqlV))
				jsonData.WriteString("\"")
			} else {
				if sqlV == nil || len(sqlV) == 0 {
					sqlV = []byte("null")
				}
				jsonData.Write(sqlV)
			}
		} else {
			sqlV = []byte("null")
			jsonData.Write(sqlV)
		}
		if index < done {
			jsonData.WriteString(",")
		}
		index += 1
	}
	jsonData.WriteString("}")
	return []byte(jsonData.String())
}
func encodeStringValue(v []byte) string {
	if v == nil {
		return "null"
	}
	if len(v) == 0 {
		return ""
	}
	s := string(v)
	b, e := json.Marshal(s)
	if e != nil || len(b) == 0 {
		return "null"
	}
	s = string(b[1 : len(b)-1])
	return s
}
func isArray(kind reflect.Kind) bool {
	if kind == reflect.Slice || kind == reflect.Array {
		return true
	}
	return false
}
func isBasicType(tItemTypeFieldType reflect.Type) bool {
	if tItemTypeFieldType.Kind() == reflect.Bool ||
		tItemTypeFieldType.Kind() == reflect.Int ||
		tItemTypeFieldType.Kind() == reflect.Int8 ||
		tItemTypeFieldType.Kind() == reflect.Int16 ||
		tItemTypeFieldType.Kind() == reflect.Int32 ||
		tItemTypeFieldType.Kind() == reflect.Int64 ||
		tItemTypeFieldType.Kind() == reflect.Uint ||
		tItemTypeFieldType.Kind() == reflect.Uint8 ||
		tItemTypeFieldType.Kind() == reflect.Uint16 ||
		tItemTypeFieldType.Kind() == reflect.Uint32 ||
		tItemTypeFieldType.Kind() == reflect.Uint64 ||
		tItemTypeFieldType.Kind() == reflect.Float32 ||
		tItemTypeFieldType.Kind() == reflect.Float64 ||
		tItemTypeFieldType.Kind() == reflect.String {
		return true
	}
	if tItemTypeFieldType.Kind() == reflect.Struct && tItemTypeFieldType.String() == "time.Time" {
		return true
	}
	return false
}
// =========================================== 根据表的model实体,生成该表的 xml 文件 =====================================================
var xmlData = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE mapper PUBLIC "-//mybatis.org//DTD Mapper 3.0//EN"
        "https://github.com/kotlin2018/mbt/blob/master/mybatis.dtd">
<mapper>
    <!--logic_enable 逻辑删除字段-->
    <!--logic_deleted 逻辑删除已删除字段-->
    <!--logic_undelete 逻辑删除 未删除字段-->
    <!--version_enable 乐观锁版本字段,支持int,int8,int16,int32,int64-->
    <resultMap id="BaseResultMap" table="#{table}">
    #{resultMapBody}
    </resultMap>
	<insertTemplate/>
	<updateTemplate/>
	<deleteTemplate/>
	<selectTemplate/>
</mapper>
`
var (
	xmlLogicEnable = `logic_enable="true" logic_undelete="1" logic_deleted="0"`
	xmlVersionEnable = `version_enable="true"`
	resultItem = `<result column="#{column}" langType="#{langType}" #{version} #{logic}/>`
)
func createXml(tableName string, tv reflect.Type) []byte {
	content := ""
	for i := 0; i < tv.NumField(); i++ {
		item := tv.Field(i)
		tagValue := item.Tag.Get("json")
		itemStr := strings.Replace(resultItem, "#{column}", tagValue, -1) // tagValue将去替换 ResultItem这个字符串中的 #{column}。
		if item.Type.Name() == "Time" {
			itemStr = strings.Replace(itemStr, "#{langType}", "time." + item.Type.Name(), -1)
		}else {
			itemStr = strings.Replace(itemStr, "#{langType}", item.Type.Name(), -1)
		}
		gm := item.Tag.Get("gm")
		if gm == "version" {
			itemStr = strings.Replace(itemStr, "#{version}", xmlVersionEnable, -1)
		}
		if gm == "logic" {
			itemStr = strings.Replace(itemStr, "#{logic}", xmlLogicEnable, -1)
		}
		itemStr = strings.Replace(itemStr, "#{version}", "", -1)
		itemStr = strings.Replace(itemStr, "#{logic}", "", -1)
		content += "\t" + itemStr
		if i+1 < tv.NumField() {
			content += "\n"
		}
	}
	res := strings.Replace(xmlData, "#{resultMapBody}", content, -1)
	res = strings.Replace(res, "#{table}", tableName, -1)
	return []byte(res)
}
// 参数 <pointer>是数据库表的实体的指针,这里不能传结构体对象的原因是(即使传的结构体对象,最终该对象也会逃逸到堆内存上去)
func (it *Engine)xmlPath(pointer interface{})string{
	t := reflect.TypeOf(pointer)
	if t.Kind() != reflect.Ptr {
		it.log.SetPrefix("[Fatal] ")
		it.log.Fatalln(t.String()+"{} 必须是指针类型!!!")
	}
	t = t.Elem()
	table := snake(t.Name())
	fileName := table+".xml"
	var (
		w strings.Builder
		f *os.File
		err error
		s string
		flag bool
	)
	if it.pkg == "" || it.pkg == "./" {
		w.WriteString("./")
		w.WriteString(fileName)
		s = w.String()
		flag = true
	}else {
		w.WriteString(it.pkg)
		w.WriteString("/")
		w.WriteString(fileName)
		s = w.String()
	}
	_,err = os.Stat(s)
	if err != nil {
		if os.IsNotExist(err) {
			body := createXml(table,t)
			if !flag {
				err = os.MkdirAll(it.pkg, os.ModePerm)
				if err != nil {
					it.log.SetPrefix("[Fatal] ")
					it.log.Fatalln("create package "+it.pkg+" error:"+ err.Error())
				}
			}
			f, err = os.Create(s)
			if err != nil {
				it.log.SetPrefix("[Fatal] ")
				it.log.Fatalln("create file"+s+" error:"+ err.Error())
			}
			defer f.Close()
			_, err = f.Write(body)
			if err != nil {
				it.log.SetPrefix("[Fatal] ")
				it.log.Fatalln("写入文件失败："+s+"error:"+ err.Error())
			} else {
				it.log.Println("写入文件成功："+s)
				return s
			}
		}
	}
	return s
}
func snake(s string) string {
	data := make([]byte, 0, len(s)*2)
	j := false
	num := len(s)
	for i := 0; i < num; i++ {
		d := s[i]
		if i > 0 && d >= 'A' && d <= 'Z' && j {
			data = append(data, '_')
		}
		if d != '_' {
			j = true
		}
		data = append(data, d)
	}
	return strings.ToLower(string(data[:]))
}
// 以下代码注释掉,并不影响程序的运行,还是先保留吧
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
// ===================== 以下几个函数只给嵌套事务使用 =================================
func (it *Engine)Tx(mapperPtr interface{}) {
	service := reflect.ValueOf(mapperPtr)
	if service.Kind() != reflect.Ptr {
		it.log.SetPrefix("[Fatal] ")
		it.log.Fatalln(service.Type().String()+"{} 必须是指针类型!!!")
	}
	it.txStruct(service, func(funcField reflect.StructField, field reflect.Value) func(arg proxyArg) []reflect.Value {
		nativeImplFunc := reflect.ValueOf(field.Interface())
		txTag, ok  := funcField.Tag.Lookup("tx")
		name := service.Type().Elem().String()+"."
		funcName := funcField.Name
		fn := func(arg proxyArg) []reflect.Value {
			s := it.s
			var err error
			it.put(goroutineID(),s)
			if !ok{
				err = s.begin("")
			}else {
				err = s.begin(txTag)
			}
			if err != nil {
				it.log.SetPrefix("[Fatal] ")
				it.log.Fatalln(name+funcName+"() "+"Begin() err = " +err.Error())
			}
			nativeImplResult := it.doNativeMethod(name,funcName, arg, nativeImplFunc, s)
			if !haveRollBackType(nativeImplResult) {
				err = s.Commit()
				if err != nil {
					it.log.SetPrefix("[Fatal] ")
					it.log.Fatalln(name+funcName+"() "+"Commit() err = "+ err.Error())
				}
			} else {
				err = s.Rollback()
				if err != nil {
					it.log.SetPrefix("[Fatal] ")
					it.log.Fatalln(name+funcName+"() "+"Rollback() err = "+ err.Error())
				}
			}
			return nativeImplResult
		}
		return fn
	})
}
func (it *Engine)txStruct(v reflect.Value, buildFunc func(funcField reflect.StructField, field reflect.Value) func(arg proxyArg) []reflect.Value) {
	v = v.Elem()
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		ft := f.Type()
		ftk := ft.Kind()
		sf := t.Field(i)
		if ftk == reflect.Ptr{
			ft = ft.Elem()
		}
		if f.CanSet() {
			switch ftk {
			case reflect.Struct:
				it.txStruct(f, buildFunc)
			case reflect.Func:
				it.txMethod(f, ft, buildFunc(sf, f))
			}
		}
	}
	v.Set(v)
}
func (it *Engine)txMethod(f reflect.Value, ft reflect.Type, proxyFunc func(arg proxyArg) []reflect.Value) {
	tagArgs := make([]tagArg, 0)
	fn := func(args []reflect.Value) (results []reflect.Value) {
		proxyResults := proxyFunc(newArg(tagArgs, args))
		for _, returnV := range proxyResults {
			results = append(results, returnV)
		}
		return results
	}
	f.Set(reflect.MakeFunc(ft, fn))
}
func (it *Engine)doNativeMethod(name ,funcName string, arg proxyArg, nativeImplFunc reflect.Value, s Session) []reflect.Value {
	defer func() {
		err := recover()
		if err != nil {
			rollbackErr := s.Rollback()
			if rollbackErr != nil {
				it.log.SetPrefix("[Fatal] ")
				it.log.Fatalln(fmt.Sprint(err) + rollbackErr.Error())
			}
			it.log.Println([]byte(fmt.Sprint(err) + " Throw out error will Rollback! from >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>> " + name+funcName+"()"))
		}
	}()
	return nativeImplFunc.Call(arg.Args)
}
func haveRollBackType(v []reflect.Value) bool {
	if v == nil || len(v) == 0 {
		return false
	}
	for _, item := range v {
		if item.Kind() == reflect.Interface {
			if strings.Contains(item.String(), "error") {
				if !item.IsNil() {
					return true
				}
			}
		}
	}
	return false
}




