package mbt

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)
func (it *Session)Run(){
	for k,v := range it.data {
		it.start(k,v)
	}
}
func (it *Session)start(be reflect.Value,outPut map[string]*returnValue) {
	it.proxyValue(be, func(funcField reflect.StructField, field reflect.Value) func(arg proxyArg) []reflect.Value {
		funcName := funcField.Name
		ret := outPut[funcName]
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
			it.exeMethodByXml(ret.xml.Tag, arg, ret.nodes,res,ret.name)
			return buildReturnValue(ret, res)
		}
		return proxyFunc
	})
}
func (it *Session)makeReturnTypeMap(bean reflect.Type,mapperTree map[string]*element,xmlName string) map[string]*returnValue {
	returnMap := make(map[string]*returnValue)
	name := bean.String()
	for i := 0; i < bean.NumField(); i++ {
		fieldItem := bean.Field(i)
		funcType := fieldItem.Type
		funcName := fieldItem.Name
		funcKind := funcType.Kind()
		if funcKind != reflect.Func {
			if funcKind == reflect.Struct {
				childMap := it.makeReturnTypeMap(funcType,mapperTree,xmlName)
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
			if isCustomStruct(inType) {
				customLen++
			}
		}
		if argsLen > 1 && customLen > 1 {
			it.log.SetPrefix("[Fatal] ")
			it.log.Fatalln(name +"."+ funcName + `() 这个函数结构体类型的输入参数有且只能有 1 个,现在它已经 > 1 个了! ([]Student这种输入参数可以有,但不能出现这种 func(s Student,u User)(int64,error)`)
		}
		numOut := funcType.NumOut()
		if numOut != 1 {
			it.log.SetPrefix("[Fatal] ")
			it.log.Fatalln(name + "." + funcName + "() return num out must = 1!")
		}
		for k := 0; k < numOut; k++ {
			outType := funcType.Out(k)
			outTypeK := outType.Kind()
			outTypeS := outType.String()
			if outTypeK == reflect.Ptr || outTypeK == reflect.Interface || outTypeS == `error`{
				it.log.SetPrefix("[Fatal] ")
				it.log.Fatalln(name + "." + funcName + "()' return value can not be a 'pointer' or 'interface' or 'error'!")
			}
			ret := returnMap[funcName]
			if ret == nil {
				returnMap[funcName] = &returnValue{
					Index: -1,
					Num:      numOut,
				}
			}
			returnMap[funcName].Index = k
			returnMap[funcName].Value = &outType
			mapperXml := it.findMapperXml(mapperTree, name,funcName,xmlName)
			returnMap[funcName].xml = mapperXml
			returnMap[funcName].nodes = express{Proxy: &nodeExpress{}}.Parser(mapperXml.Child)
			returnMap[funcName].name = name+"."+funcName+"() "
		}
	}
	return returnMap
}
func isCustomStruct(value reflect.Type) bool {
	if value.Kind() == reflect.Struct && value.String() != `time.Time` && value.String() != `*time.Time` {
		return true
	} else {
		return false
	}
}
func (it *Session)findMapperXml(mapperTree map[string]*element, beanName,methodName,xmlName string) *element {
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
func (it *Session)includeElementReplace(xml *element, xmlMap *map[string]*element,xmlName string) {
	if xml.Tag == "include" {
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
func isMethodElement(tag string) bool {
	switch tag {
	case "insert", "delete", "update", "select":
		return true
	}
	return false
}
func (it *Session)decodeTree(tree map[string]*element, beanType reflect.Type,xmlName string){
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
			it.log.Println(s)
		}
	}
}
func printElement(ele *element, v *string) {
	*v += "<" + ele.Tag + " "
	for _, item := range ele.Attr {
		*v += item.Key + `="` + item.Value + `"`
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
func (it *Session)decode(method *reflect.StructField, mapper *element, tree map[string]*element,xmlName string){
	switch mapper.Tag {
	case "select":
		id := mapper.SelectAttrValue("id", "")
		if id == "" {
			mapper.CreateAttr("id", "select")
		}
		columns := mapper.SelectAttrValue("column", "")
		if columns == ""{
			break
		}
		resultMap := mapper.SelectAttrValue("resultMap", "")
		resultMapData := tree[resultMap]
		tables := mapper.SelectAttrValue("table", "")
		if resultMapData == nil && tables == ""{
			it.log.SetPrefix("[Fatal] ")
			it.log.Fatalln(xmlName+"TemplateDecoder", "resultMap not define! id = ", resultMap)
		}
		it.checkTablesValue(mapper, &tables, resultMapData,xmlName)
		wheres := mapper.SelectAttrValue("where", "")
		logic := it.decodeLogicDelete(resultMapData,xmlName)
		var sql bytes.Buffer
		sql.WriteString("select ")
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
	case "insert":
		id := mapper.SelectAttrValue("id", "")
		if id == "" {
			mapper.CreateAttr("id", "insert")
		}
		resultMap := mapper.SelectAttrValue("resultMap", "")
		if resultMap == "" {
			break
		}
		resultMapData := tree[resultMap]
		if resultMapData == nil {
			it.log.SetPrefix("[Fatal] ")
			it.log.Fatalln(xmlName+" TemplateDecoder", "resultMap not define! id = ", resultMap)
		}
		tables := mapper.SelectAttrValue("table", "")
		inserts := mapper.SelectAttrValue("insert", "")
		if inserts == "" {
			inserts = "*?*"
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
		trimColumn := element{
			Tag: "trim",
			Attr: []attr{
				{Key: "prefix", Value: "("},
				{Key: "suffix", Value: ")"},
				{Key: "suffixOverrides", Value: ","},
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
						Tag: "if",
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
			Tag:   "trim",
			Attr:  []attr{{Key: "prefix", Value: "values ("}, {Key: "suffix", Value: ")"}, {Key: "suffixOverrides", Value: ","}},
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
						Tag:  "if",
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
			tempElement.Tag = "foreach"
			tempElement.Attr = []attr{{Key: "open", Value: "values "}, {Key: "close", Value: ""}, {Key: "separator", Value: ","}, {Key: "collection", Value: collectionName}}
			tempElement.Child = []token{}
			for index, v := range resultMapData.ChildElements() {
				prefix := ""
				if index == 0 {
					prefix = "("
				}
				defProperty := v.SelectAttrValue("column", "")
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
				value := prefix + "#{" + "item." + defProperty + "}"
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
	case "update":
		id := mapper.SelectAttrValue("id", "")
		if id == "" {
			mapper.CreateAttr("id", "update")
		}
		resultMap := mapper.SelectAttrValue("resultMap", "")
		if resultMap == "" {
			break
		}
		resultMapData := tree[resultMap]
		if resultMapData == nil {
			it.log.SetPrefix("[Fatal] ")
			it.log.Fatalln(xmlName+" TemplateDecoder", "resultMap not define! id = ", resultMap)
		}
		tables := mapper.SelectAttrValue("table", "")
		columns := mapper.SelectAttrValue("set", "")
		wheres := mapper.SelectAttrValue("where", "")
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
	case "delete":
		id := mapper.SelectAttrValue("id", "")
		if id == "" {
			mapper.CreateAttr("id", "delete")
		}
		resultMap := mapper.SelectAttrValue("resultMap", "")
		if resultMap == "" {
			break
		}
		resultMapData := tree[resultMap]
		if resultMapData == nil {
			it.log.SetPrefix("[Fatal] ")
			it.log.Fatalln(xmlName+" TemplateDecoder", "resultMap not define! id = ", resultMap)
		}
		tables := mapper.SelectAttrValue("table", "")
		wheres := mapper.SelectAttrValue("where", "")
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
func (it *Session)checkTablesValue(mapper *element, tables *string, resultMapData *element,xmlName string) {
	if *tables == "" {
		*tables = resultMapData.SelectAttrValue("table", "")
		if *tables == "" {
			it.log.SetPrefix("[Fatal] ")
			it.log.Fatalln(xmlName+" 文件中的 标签 <"+mapper.Tag+" id = "+mapper.SelectAttrValue("id","")+` table 属性不能为空!如果为空,则 resultMap 属性指定的标签 <resultMap id="" table="" </resultMap> table 的值一定不能为空!`)
		}
	}
}
func decodeWheres(arg string, mapper *element, logic logicDeleteData, version *versionData) {
	whereRoot := &element{
		Tag:   "where",
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
				Tag:   "if",
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
				Tag:  "if",
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
func (it *Session)decodeLogicDelete(xml *element,xmlName string) logicDeleteData {
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
func (it *Session)decodeVersionData(xml *element,xmlName string) *versionData {
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
					collection = `arg` + strconv.Itoa(i)
				} else {
					if args[i] == "" {
						collection = `arg` + strconv.Itoa(i)
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
func (it *Session)exeMethodByXml(elementType string, proxyArg proxyArg, nodes []iiNode, returnValue *reflect.Value,name string){
	convert := it.stmtConvert()
	array := make([]interface{},0)
	sql := it.buildSql(proxyArg, nodes,&array, convert,name)
	if elementType == "select"{
		res := it.queryPrepare(name,sql, array...)
		it.decodeSqlResult(res, returnValue.Interface(),name)
	} else {
		res := it.execPrepare(name,sql, array...)
		returnValue.Elem().SetInt(res.RowsAffected)
	}
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
func (it *Session)buildSql(proxyArg proxyArg, nodes []iiNode, array *[]interface{}, stmtConvert Convert,name string) string{
	paramMap := make(map[string]interface{})
	tagArgsLen := proxyArg.TagArgsLen
	argsLen := proxyArg.ArgsLen
	customLen := 0
	customIndex := -1
	for argIndex, arg := range proxyArg.Args {
		argInterface := arg.Interface()
		argK := arg.Kind()
		argT := arg.Type()
		if argK == reflect.Ptr && arg.IsNil() == false && argInterface != nil && argT.String() == `*mbt.Session` {
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
			paramMap[`arg`+strconv.Itoa(argIndex)] = argInterface
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
func (it *Session)sqlBuild(args map[string]interface{}, node []iiNode, array *[]interface{}, stmtConvert Convert,name string)string{
	sql, err := doChildNodes(node, args, array, stmtConvert)
	if err != nil {
		it.log.SetPrefix("[Fatal] ")
		it.log.Fatalln(name+err.Error())
	}
	return string(sql)
}
func (it *Session)scanStructArgFields(v reflect.Value, tag *tagArg,name string) map[string]interface{} {
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
func (it *Session)proxyValue(v reflect.Value, buildFunc func(funcField reflect.StructField, field reflect.Value) func(arg proxyArg) []reflect.Value) {
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
				it.buildRemoteMethod(v.Type().String()+"."+sf.Name+"()",f, ft, sf, buildFunc(sf, f))
			}
		}
	}
	v.Set(v)
}
func (it *Session)buildRemoteMethod(name string,f reflect.Value, ft reflect.Type, sf reflect.StructField, proxyFunc func(arg proxyArg) []reflect.Value) {
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
			if ftk == reflect.Struct || ftk == reflect.Slice && fti.Elem().Kind() == reflect.Struct{
				continue
			}else {
				it.log.SetPrefix("[Fatal] ")
				it.log.Fatalln(name+` 上的 tag "arg:" 的值的个数 != `+name+` 的输入参数的个数!!`)
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
			it.log.Fatalln(name+` 上的 tag "arg:" 的值的个数 != `+name+` 的输入参数的个数!!`)
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
func (it *Session)decodeSqlResult(sqlResult []map[string][]byte, result interface{},name string){
	if sqlResult == nil || result == nil {
		return
	}
	resultV := reflect.ValueOf(result).Elem()
	value := make([]byte,0)
	sqlResultLen := len(sqlResult)
	if sqlResultLen == 0 {
		return
	}
	if isArray(resultV.Kind()) {
		resultVItemType := resultV.Type().Elem()
		structMap := makeStructMap(resultVItemType)
		done := len(sqlResult) - 1
		index := 0
		jsonData := strings.Builder{}
		jsonData.WriteString("[")
		for _, v := range sqlResult {
			jsonData.Write(makeJsonObjBytes(v, structMap))
			if index < done {
				jsonData.WriteString(",")
			}
			index += 1
		}
		jsonData.WriteString("]")
		value = []byte(jsonData.String())
	}else {
		if sqlResultLen > 1 {
			it.log.SetPrefix("[Fatal] ")
			it.log.Fatalln(name+"SqlResultDecoder Decode one result,but find database result size find > 1 !")
		}
		if isBasicType(resultV.Type()) {
			for _, s := range sqlResult[0] {
				var b = strings.Builder{}
				if resultV.Kind() == reflect.String || (resultV.Kind() == reflect.Struct) {
					b.WriteString(`"`)
					b.Write(s)
					b.WriteString(`"`)
				} else {
					b.Write(s)
				}
				value = []byte(b.String())
				break
			}
		} else {
			structMap := makeStructMap(resultV.Type())
			value = makeJsonObjBytes(sqlResult[0], structMap)
		}
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
func makeJsonObjBytes(sqlData map[string][]byte, structMap map[string]*reflect.Type) []byte {
	jsonData := strings.Builder{}
	jsonData.WriteString("{")
	done := len(sqlData) - 1
	index := 0
	for k, sqlV := range sqlData {
		jsonData.WriteString(`"`)
		jsonData.WriteString(k)
		jsonData.WriteString(`":`)
		isStringType := false
		fetched := true
		if structMap != nil {
			v := structMap[strings.ToLower(k)]
			if v != nil {
				if (*v).Kind() == reflect.String || (*v).String() == "time.Time" {
					isStringType = true
				}
			}else {
				fetched = false
			}
		}else {
			isStringType = true
		}
		if fetched {
			if isStringType {
				jsonData.WriteString(`"`)
				jsonData.WriteString(encodeStringValue(sqlV))
				jsonData.WriteString(`"`)
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
func isBasicType(arg reflect.Type) bool {
	if arg.Kind() == reflect.Bool ||
		arg.Kind() == reflect.Int ||
		arg.Kind() == reflect.Int8 ||
		arg.Kind() == reflect.Int16 ||
		arg.Kind() == reflect.Int32 ||
		arg.Kind() == reflect.Int64 ||
		arg.Kind() == reflect.Uint ||
		arg.Kind() == reflect.Uint8 ||
		arg.Kind() == reflect.Uint16 ||
		arg.Kind() == reflect.Uint32 ||
		arg.Kind() == reflect.Uint64 ||
		arg.Kind() == reflect.Float32 ||
		arg.Kind() == reflect.Float64 ||
		arg.Kind() == reflect.String {
		return true
	}
	if arg.Kind() == reflect.Struct && arg.String() == "time.Time" {
		return true
	}
	return false
}
var (
	xmlData = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE mapper PUBLIC "-//mybatis.org//DTD Mapper 3.0//EN"
       "https://github.com/kotlin2018/mbt/blob/master/mybatis.dtd">
<mapper namespace="#{namespace}">
    <!--logic_enable 逻辑删除字段-->
    <!--logic_deleted 逻辑删除已删除字段-->
    <!--logic_undelete 逻辑删除 未删除字段-->
    <!--version_enable 乐观锁版本字段,支持int,int8,int16,int32,int64-->
	<resultMap id="#{resultMap}" table="#{table}">
    #{resultMapBody}
    </resultMap>
	<insert id=""/>
	<update id=""/>
	<delete id=""/>
	<select id=""/>
</mapper>
`
	xmlDataS = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE mapper PUBLIC "-//mybatis.org//DTD Mapper 3.0//EN"
        "https://github.com/kotlin2018/mbt/blob/master/mybatis.dtd">
<mapper namespace="#{namespace}">
	<insert id=""/>
	<update id=""/>
	<delete id=""/>
	<select id=""/>
</mapper>
`
	xmlLogicEnable = `logic_enable="true" logic_undelete="1" logic_deleted="0"`
	xmlVersionEnable = `version_enable="true"`
	resultItem = `<result column="#{column}" property="#{property}" langType="#{langType}" #{version} #{logic}/>`
)
func (it *Session)createXml(name string,tv reflect.Type)[]byte{
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
		itemStr = strings.Replace(itemStr, "#{property}", item.Name, -1)
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
	res := strings.Replace(xmlData, "#{namespace}", it.namespace+"."+name, -1)
	res = strings.Replace(res, "#{resultMap}", tv.String(), -1)
	res = strings.Replace(res, "#{table}", snake(tv.Name()), -1)
	res = strings.Replace(res, "#{resultMapBody}", content, -1)
	return []byte(res)
}
// https://mp.weixin.qq.com/s/1nqpVzitGdVVvHIuk8iAeQ
func (it *Session)register(mapperPtr,modelPtr interface{}){
	var (
		obj = reflect.ValueOf(mapperPtr)
		bt = obj.Type().Elem()
		name = bt.Name()
		t reflect.Type
		w strings.Builder
		f *os.File
		err error
		s string
		fileName string
		flag bool
		body []byte
	)
	if bt.NumField() != 0 {
		if xml,ok := modelPtr.(string);ok && xml !="" {
			fileName = xml
			body = []byte(strings.Replace(xmlDataS, "#{namespace}", it.namespace+"."+name, -1))
		}else {
			t = reflect.TypeOf(modelPtr)
			if t.Kind() != reflect.Ptr {
				it.log.SetPrefix("[Fatal] ")
				it.log.Fatalln(t.String()+"{} 必须是指针类型!!!")
			}
			t = t.Elem()
			fileName = name+".xml"
			body = it.createXml(name,t)
		}
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
				}
			}
		}
		it.data = make(map[reflect.Value]map[string]*returnValue,0)
		tree := it.parseXml(s)
		it.decodeTree(tree,bt,s)
		it.data[obj.Elem()] = it.makeReturnTypeMap(bt, tree,s)
	}
}
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
func (it *Session)parseXml(xmlName string) (items map[string]*element) {
	bytes, _ := ioutil.ReadFile(xmlName)
	expressSymbol(&bytes)
	doc := newDocument()
	if err := doc.ReadFromBytes(bytes); err != nil {
		it.log.SetPrefix("[Fatal] ")
		it.log.Fatalln("解析 "+xmlName+" 文件错误,err=",err)
	}
	items = make(map[string]*element)
	root := doc.SelectElement("mapper")
	for _, s := range root.ChildElements() {
		if s.Tag == "insert" ||
			s.Tag == "delete" ||
			s.Tag == "update" ||
			s.Tag == "select" ||
			s.Tag == "resultMap" ||
			s.Tag == "sql"{
			idValue := s.SelectAttrValue("id", "")
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