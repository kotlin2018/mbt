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
func (it *Session)Run(mapperPtr ...interface{}){
	for _,v := range mapperPtr {
		it.register(v)
	}
}
func (it *Session)start(be reflect.Value,outPut map[string]*returnValue) {
	it.proxyValue(be, func(funcField reflect.StructField, field reflect.Value) func(arg proxyArg) []reflect.Value {
		funcName := funcField.Name
		ret := outPut[funcName]
		proxyFunc := func(arg proxyArg) []reflect.Value {
			var res *reflect.Value = nil
			returnV := reflect.New(*ret.value)
			switch (*ret.value).Kind() {
			case reflect.Map:
				returnV.Elem().Set(reflect.MakeMap(*ret.value))
			case reflect.Slice:
				returnV.Elem().Set(reflect.MakeSlice(*ret.value, 0, 0))
			}
			res = &returnV
			it.exeMethodByXml(ret,res,arg)
			list := make([]reflect.Value, 1)
			list[0] = (*res).Elem()
			return list
		}
		return proxyFunc
	})
}
func (it *Session)makeReturnTypeMap(bean reflect.Type,xmlName string) map[string]*returnValue {
	mapperTree := it.parseXml(xmlName)
	it.decodeTree(mapperTree,bean,xmlName)
	returnMap := make(map[string]*returnValue,0)
	name := bean.String()
	for i := 0; i < bean.NumField(); i++ {
		fieldItem := bean.Field(i)
		funcType := fieldItem.Type
		funcName := fieldItem.Name
		funcKind := funcType.Kind()
		if funcKind == reflect.Func {
			args,ok := fieldItem.Tag.Lookup(`arg`)
			tagLen := len(strings.Split(args, `,`))
			argsLen := funcType.NumIn()
			customLen := 0
			for j := 0; j < argsLen; j++ {
				inType := funcType.In(j)
				ftk := inType.Kind()
				if ftk != reflect.Struct{
					if ftk == reflect.Slice || ftk == reflect.Map{
						if inType.Elem().Kind()!= reflect.Struct && !ok || tagLen != argsLen{
							it.log.SetPrefix("[Fatal] ")
							it.log.Fatalln(name +"."+ funcName + `() 上的 tag "arg:" 的值的个数 != `+name +"."+ funcName + `() 的输入参数的个数!`)
						}
					}else if args == "" || tagLen != argsLen{
						it.log.SetPrefix("[Fatal] ")
						it.log.Fatalln(name +"."+ funcName + `() 上的 tag "arg:" 的值的个数 != `+name +"."+ funcName + `() 的输入参数的个数!`)
					}
				}
				if ftk == reflect.Struct && inType.String() != `time.Time` && inType.String() != `*time.Time`{
					customLen++
				}
			}
			if argsLen > 1 && customLen > 1 {
				it.log.SetPrefix("[Fatal] ")
				it.log.Fatalln(name +"."+ funcName + `() 这个函数结构体类型的输入参数有且只能有 1 个,现在它已经 > 1 个了! ([]Student这种输入参数可以有,但不能出现这种 func(s Student,u User)int64`)
			}
			if funcType.NumOut() != 1 {
				it.log.SetPrefix("[Fatal] ")
				it.log.Fatalln(name + "." + funcName + "() return num out must = 1!")
			}
			outType := funcType.Out(0)
			outTypeK := outType.Kind()
			outTypeS := outType.String()
			if outTypeK == reflect.Ptr || outTypeK == reflect.Interface || outTypeK == reflect.Map || outTypeK == reflect.Slice && outType.Elem().Kind() != reflect.Struct || outTypeS == `error`{
				it.log.SetPrefix("[Fatal] ")
				it.log.Fatalln(name + "." + funcName + "()' return value can not be a 'pointer' or 'interface' or 'error' or 'map' or '[]map' !")
			}
			returnMap[funcName] = &returnValue{}
			returnMap[funcName].value = &outType
			mapperXml := it.findMapperXml(mapperTree, name,funcName,xmlName)
			returnMap[funcName].xml = mapperXml
			returnMap[funcName].nodes = express{Proxy: &nodeExpress{}}.Parser(mapperXml.Child)
			returnMap[funcName].name = name+"."+funcName+"() "
		}
	}
	return returnMap
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
						argItem := method.Type.In(i)
						if argItem.Kind() == reflect.Slice || argItem.Kind() == reflect.Array {
							argItem = argItem.Elem()
						}
						if argItem.Kind() == reflect.Struct {
							for k := 0; k < argItem.NumField(); k++ {
								arg := argItem.Field(k)
								if strings.ToLower(strings.ReplaceAll(defProperty, "_", "")) == strings.ToLower(arg.Name){
									defProperty = arg.Name
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
func (it *Session)exeMethodByXml(ret *returnValue,returnValue *reflect.Value,proxyArg proxyArg){
	convert := it.stmtConvert()
	array := make([]interface{},0)
	sql := it.buildSql(proxyArg,ret,&array,convert)
	if ret.xml.Tag == "select"{
		var res []map[string]string
		if it.slave != nil {
			list := make([]interface{},0)
			res = it.slaveQuery(ret.name,it.buildSql(proxyArg,ret,&list, it.slaveConvert()), array...)
		}else {
			res = it.queryPrepare(ret.name,sql, array...)
		}
		it.decodeSqlResult(res,returnValue.Interface(),ret.name)
	} else {
		returnValue.Elem().SetInt(it.execPrepare(ret.name,sql, array...).RowsAffected)
	}
}
func (it *Session)buildSql(proxyArg proxyArg,ret *returnValue,array *[]interface{},stmtConvert Convert) string{
	paramMap := make(map[string]interface{})
	tagArgsLen := proxyArg.TagArgsLen
	customIndex := -1
	for argIndex, arg := range proxyArg.Args {
		argInterface := arg.Interface()
		argT := arg.Type()
		if argT.Kind() == reflect.Struct && argT.String() != `time.Time` && argT.String() != `*time.Time` {
			customIndex = argIndex
		}
		if tagArgsLen > 0 && proxyArg.TagArgs[argIndex].Name != ""{
			paramMap[proxyArg.TagArgs[argIndex].Name] = argInterface
		} else {
			paramMap[`arg`+strconv.Itoa(argIndex)] = argInterface
		}
	}
	if customIndex != -1 {
		v := proxyArg.Args[customIndex]
		t := v.Type()
		for i := 0; i < t.NumField(); i++ {
			typeValue := t.Field(i)
			obj := v.Field(i).Interface()
			tagValue := typeValue.Tag.Get(`arg`)
			if tagValue != "" {
				paramMap[tagValue]=obj
			}else {
				paramMap[typeValue.Name]=obj
			}
		}
	}
	return it.sqlBuild(paramMap,ret,array,stmtConvert)
}
func (it *Session)sqlBuild(args map[string]interface{},ret *returnValue,array *[]interface{},stmtConvert Convert)string{
	sql, err := doChildNodes(ret.nodes, args, array, stmtConvert)
	if err != nil {
		it.log.SetPrefix("[Fatal] ")
		it.log.Fatalln(ret.name+" "+err.Error())
	}
	if sql == nil {
		it.log.SetPrefix("[Fatal] ")
		it.log.Fatalln(ret.name+" Not Find SQL Statements")
	}
	return string(sql)
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
				it.buildRemoteMethod(f, ft, sf, buildFunc(sf, f))
			}
		}
	}
	v.Set(v)
}
func (it *Session)buildRemoteMethod(f reflect.Value, ft reflect.Type,sf reflect.StructField, proxyFunc func(arg proxyArg) []reflect.Value) {
	tagArgs := make([]tagArg, 0)
	args := sf.Tag.Get(`arg`)
	tagParams := strings.Split(args, `,`)
	for index, v := range tagParams {
		tag := tagArg{
			Index: index,
			Name:  v,
		}
		tagArgs = append(tagArgs, tag)
	}
	fn := func(args []reflect.Value) (results []reflect.Value) {
		proxyResults := proxyFunc(newArg(tagArgs, args))
		for _, returnV := range proxyResults {
			results = append(results, returnV)
		}
		return results
	}
	f.Set(reflect.MakeFunc(ft, fn))
}
func (it *Session)decodeSqlResult(sqlResult []map[string]string, result interface{},name string){
	sqlResultLen := len(sqlResult)
	if sqlResultLen == 0 {
		return
	}
	resultV := reflect.ValueOf(result).Elem()
	value := ""
	resultK := resultV.Kind()
	if resultK == reflect.Slice || resultK == reflect.Array {
		resultVItemType := resultV.Type().Elem()
		structMap := makeStructMap(resultVItemType)
		done := len(sqlResult) - 1
		index := 0
		jsonData := strings.Builder{}
		jsonData.WriteString(`[`)
		for _, v := range sqlResult {
			jsonData.WriteString(it.makeJsonObjByte(v, structMap,name))
			if index < done {
				jsonData.WriteString(`,`)
			}
			index += 1
		}
		jsonData.WriteString(`]`)
		value = jsonData.String()
	}else {
		if sqlResultLen > 1 {
			it.log.SetPrefix("[Fatal] ")
			it.log.Fatalln(name+" SqlResultDecoder Decode one result,but find database result size find > 1 !")
		}
		if isBasicType(resultV.Type()) {
			for _, s := range sqlResult[0] {
				b := strings.Builder{}
				if resultV.Kind() == reflect.String || resultV.Kind() == reflect.Struct {
					b.WriteString(`"`)
					b.WriteString(s)
					b.WriteString(`"`)
				} else {
					b.WriteString(s)
				}
				value = b.String()
				break
			}
		} else {
			structMap := makeStructMap(resultV.Type())
			value = it.makeJsonObjByte(sqlResult[0], structMap,name)
		}
	}
	err := json.Unmarshal([]byte(value), result)
	if err != nil {
		it.log.SetPrefix("[Fatal] ")
		it.log.Fatalln(name+err.Error())
	}
}
func makeStructMap(itemType reflect.Type)map[string]*reflect.Type{
	structMap := map[string]*reflect.Type{}
	for i := 0; i < itemType.NumField(); i++ {
		item := itemType.Field(i)
		itemT := item.Type
		if itemT.Kind() == reflect.Struct{
			for j :=0; j < itemT.NumField();j++{
				field := itemT.Field(j)
				structMap[strings.ToLower(field.Name)] = &field.Type
			}
		}
		structMap[strings.ToLower(item.Name)] = &item.Type
	}
	return structMap
}
func (it *Session)makeJsonObjByte(sqlData map[string]string, structMap map[string]*reflect.Type,name string) string {
	jsonData := strings.Builder{}
	jsonData.WriteString(`{`)
	done := len(sqlData) - 1
	index := 0
	for k, sqlV := range sqlData {
		jsonData.WriteString(`"`)
		jsonData.WriteString(k)
		jsonData.WriteString(`":`)
		v := structMap[k]
		if v != nil {
			if (*v).Kind() == reflect.String || (*v).String() ==`time.Time`{
				jsonData.WriteString(`"`)
				jsonData.WriteString(sqlV)
				jsonData.WriteString(`"`)
			}else {
				jsonData.WriteString(sqlV)
			}
		}else {
			it.log.SetPrefix("[Fatal] ")
			it.log.Fatalln(name+"方法的返回值缺少一个结构体字段",k)
		}
		if index < done {
			jsonData.WriteString(`,`)
		}
		index += 1
	}
	jsonData.WriteString(`}`)
	return jsonData.String()
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
	if arg.Kind() == reflect.Struct && arg.String() == `time.Time` {
		return true
	}
	return false
}

func (it *Session)createXml(tv reflect.Type)[]byte{
	content := ""
	num := tv.NumField()
	for i := 0; i < num; i++ {
		item := tv.Field(i)
		itemStr := strings.Replace(`<result column="#{column}" property="#{property}" langType="#{langType}" #{version} #{logic}/>`, "#{column}", snake(item.Name), -1)
		if item.Type.Name() == "Time" {
			itemStr = strings.Replace(itemStr, "#{langType}", "time." + item.Type.Name(), -1)
		}else {
			itemStr = strings.Replace(itemStr, "#{langType}", item.Type.Name(), -1)
		}
		itemStr = strings.Replace(itemStr, "#{property}", item.Name, -1)
		gm := item.Tag.Get("gm")
		if gm == "version" {
			itemStr = strings.Replace(itemStr, "#{version}", `version_enable="true"`, -1)
		}
		if gm == "logic" {
			itemStr = strings.Replace(itemStr, "#{logic}", `logic_enable="true" logic_undelete="1" logic_deleted="0"`, -1)
		}
		itemStr = strings.Replace(itemStr, "#{version}", "", -1)
		itemStr = strings.Replace(itemStr, "#{logic}", "", -1)
		content += "\t" + itemStr
		if i+1 < num {
			content += "\n"
		}
	}
	res := strings.Replace(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE mapper PUBLIC "-//mybatis.org//DTD Mapper 3.0//EN"
       "https://github.com/kotlin2018/mbt/blob/master/mybatis.dtd">
<mapper>
    <!--logic_enable 逻辑删除字段-->
    <!--logic_deleted 逻辑删除已删除字段-->
    <!--logic_undelete 逻辑删除 未删除字段-->
    <!--version_enable 乐观锁版本字段,支持int,int8,int16,int32,int64-->
	<resultMap id="base" table="#{table}">
    #{resultMapBody}
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
`, "#{table}", snake(tv.Name()), -1)
	res = strings.Replace(res, "#{resultMapBody}", content, -1)
	return []byte(res)
}
func (it *Session)genXml(bt reflect.Type)string {
	var (
		w strings.Builder
		s string
		err error
		flag bool
		name = bt.Name()
		fileName = name+".xml"
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
			var (
				f *os.File
				body []byte
				num = bt.NumField()
			)
			if !flag {
				err = os.MkdirAll(it.pkg, os.ModePerm)
				if err != nil {
					it.log.SetPrefix("[Fatal] ")
					it.log.Fatalln("create package "+it.pkg+" error:"+ err.Error())
				}
			}
			if num == 0 {
				res := strings.Replace(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE mapper PUBLIC "-//mybatis.org//DTD Mapper 3.0//EN"
       "https://github.com/kotlin2018/mbt/blob/master/mybatis.dtd">
<mapper>
	<insert id=""></insert>
	<delete id=""></delete>
	<update id=""></update>
	<select id=""></select>
</mapper>
`, "#{table}", snake(name), -1)
				body = []byte(res)
			}else {
				fieldType := bt.Field(0).Type
				if fieldType.Kind() != reflect.Struct || fieldType.String() == `time.Time`{
					it.log.SetPrefix("[Fatal] ")
					it.log.Fatalln(name + " 结构体的第一个字段必须是非 time.Time 结构体!")
				}
				body = it.createXml(fieldType)
			}
			f, err = os.Create(s)
			if err != nil {
				it.log.SetPrefix("[Fatal] ")
				it.log.Fatalln("create file"+s+" error:"+ err.Error())
			}
			_, err = f.Write(body)
			f.Close()
			if err != nil {
				it.log.SetPrefix("[Fatal] ")
				it.log.Fatalln("写入文件失败："+s+"error:"+ err.Error())
			} else {
				it.log.Println("写入文件成功："+s)
			}
		}
	}
	return s
}
func (it *Session)register(mapperPtr interface{})*Session{
	obj := reflect.ValueOf(mapperPtr)
	bt := obj.Type().Elem()
	s := it.genXml(bt)
	it.start(obj.Elem(),it.makeReturnTypeMap(bt,s))
	return it
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
func (it *Session)parseXml(xmlName string) map[string]*element {
	bytes, _ := ioutil.ReadFile(xmlName)
	expressSymbol(&bytes)
	doc := newDocument()
	if err := doc.ReadFromBytes(bytes); err != nil {
		it.log.SetPrefix("[Fatal] ")
		it.log.Fatalln("解析 "+xmlName+" 文件错误,err=",err)
	}
	items := make(map[string]*element)
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
	num := len(s)
	data := make([]byte, 0, 2*num)
	j := false
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