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
	num := len(mapperPtr)
	if len(it.data)!= num {
		for i:=0;i<num;i++ {
			it.genXml(mapperPtr[i])
		}
	}
	for i:=0;i<num;i++ {
		it.start(mapperPtr[i])
	}
}
func (it *Session)start(mapperPtr interface{}) {
	outPut := it.data[mapperPtr]
	be := reflect.ValueOf(mapperPtr).Elem()
	it.proxyValue(be, func(funcField reflect.StructField, field reflect.Value) func(arg proxyArg) []reflect.Value {
		funcName := funcField.Name
		ret := outPut[funcName]
		proxyFunc := func(arg proxyArg) []reflect.Value {
			var res *reflect.Value = nil
			returnV := reflect.New(*ret.value)
			returnVE := returnV.Elem()
			rvk := (*ret.value).Kind()
			switch rvk {
			case reflect.Map:
				returnVE.Set(reflect.MakeMap(*ret.value))
			case reflect.Slice:
				returnVE.Set(reflect.MakeSlice(*ret.value, 0, 0))
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

func (it *Session)decodeTree(tree map[string]*element, beanType reflect.Type,xmlName string){
	for _, v := range tree {
		var method *reflect.StructField
		switch v.Tag {
		case "insert", "delete", "update", "select":
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
	num := len(proxyArg.Args)
	for i:=0;i<num;i++ {
		argInterface := proxyArg.Args[i].Interface()
		argT := proxyArg.Args[i].Type()
		if argT.Kind() == reflect.Struct && argT.String() != `time.Time` && argT.String() != `*time.Time` {
			customIndex = i
		}
		if tagArgsLen > 0 && proxyArg.TagArgs[i].Name != ""{
			paramMap[proxyArg.TagArgs[i].Name] = argInterface
		} else {
			paramMap[`arg`+strconv.Itoa(i)] = argInterface
		}
	}
	if customIndex != -1 {
		v := proxyArg.Args[customIndex]
		t := v.Type()
		cou := t.NumField()
		for i := 0; i <cou; i++ {
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
	num := v.NumField()
	for i := 0; i < num;i++ {
		f := v.Field(i)
		ft := f.Type()
		ftk := ft.Kind()
		sf := v.Type().Field(i)
		if ftk == reflect.Func {
			it.buildRemoteMethod(f, ft, sf, buildFunc(sf, f))
		}
	}
	v.Set(v)
}
func (it *Session)buildRemoteMethod(f reflect.Value, ft reflect.Type,sf reflect.StructField, proxyFunc func(arg proxyArg) []reflect.Value) {
	tagArgs := make([]tagArg, 0)
	args := sf.Tag.Get(`arg`)
	tagParams := strings.Split(args, `,`)
	num := len(tagParams)
	for i:=0;i<num;i++ {
		tag := tagArg{
			Index: i,
			Name:  tagParams[i],
		}
		tagArgs = append(tagArgs, tag)
	}
	tagNum := len(tagArgs)
	argNum := len(args)
	fn := func(args []reflect.Value) (results []reflect.Value) {
		proxyResults := proxyFunc(proxyArg{
			TagArgs:    tagArgs,
			Args:       args,
			TagArgsLen: tagNum,
			ArgsLen:    argNum,
		})
		count := len(proxyResults)
		for i:=0;i<count;i++ {
			results = append(results, proxyResults[i])
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
		itemType := resultV.Type().Elem()
		structMap := map[string]*reflect.Type{}
		num := itemType.NumField()
		for i := 0; i <num; i++ {
			item := itemType.Field(i)
			itemT := item.Type
			if itemT.Kind() == reflect.Struct{
				cou:=itemT.NumField()
				for j :=0;j<cou;j++{
					field := itemT.Field(j)
					structMap[strings.ToLower(field.Name)] = &field.Type
				}
			}
			structMap[strings.ToLower(item.Name)] = &item.Type
		}
		done := len(sqlResult) - 1
		index := 0
		jsonData := strings.Builder{}
		jsonData.WriteString(`[`)
		for i:=0;i<sqlResultLen;i++{
			build := strings.Builder{}
			build.WriteString(`{`)
			b := len(sqlResult[i]) - 1
			a := 0
			for k, sqlV := range sqlResult[i] {
				build.WriteString(`"`)
				build.WriteString(k)
				build.WriteString(`":`)
				v := structMap[k]
				if v != nil {
					if (*v).Kind() == reflect.String || (*v).String() ==`time.Time`{
						build.WriteString(`"`)
						build.WriteString(sqlV)
						build.WriteString(`"`)
					}else {
						build.WriteString(sqlV)
					}
				}else {
					it.log.SetPrefix("[Fatal] ")
					it.log.Fatalln(name+"方法的返回值缺少一个结构体字段",k)
				}
				if a < b {
					build.WriteString(`,`)
				}
				a += 1
			}
			build.WriteString(`}`)
			jsonData.WriteString(build.String())
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
		rtk := resultV.Type().Kind()
		if isBasicType(rtk) {
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
			structMap := map[string]*reflect.Type{}
			itemType := resultV.Type()
			num := itemType.NumField()
			for i := 0; i <num; i++ {
				item := itemType.Field(i)
				itemT := item.Type
				if itemT.Kind() == reflect.Struct{
					cou:=itemT.NumField()
					for j :=0;j<cou;j++{
						field := itemT.Field(j)
						structMap[strings.ToLower(field.Name)] = &field.Type
					}
				}
				structMap[strings.ToLower(item.Name)] = &item.Type
			}
			jsonData := strings.Builder{}
			jsonData.WriteString(`{`)
			done := len(sqlResult[0]) - 1
			index := 0
			for k, sqlV := range sqlResult[0] {
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
			value = jsonData.String()
		}
	}
	err := json.Unmarshal([]byte(value), result)
	if err != nil {
		it.log.SetPrefix("[Fatal] ")
		it.log.Fatalln(name+err.Error())
	}
}
func isBasicType(arg reflect.Kind) bool {
	if 	arg == reflect.Bool ||
		arg == reflect.Int ||
		arg == reflect.Int8 ||
		arg == reflect.Int16 ||
		arg == reflect.Int32 ||
		arg == reflect.Int64 ||
		arg == reflect.Uint ||
		arg == reflect.Uint8 ||
		arg == reflect.Uint16 ||
		arg == reflect.Uint32 ||
		arg == reflect.Uint64 ||
		arg == reflect.Float32 ||
		arg == reflect.Float64 ||
		arg == reflect.String {
		return true
	}
	if arg == reflect.Struct && arg.String() == `time.Time` {
		return true
	}
	return false
}
func (it *Session)genXml(mapperPtr interface{}){
	obj := reflect.ValueOf(mapperPtr)
	bt := obj.Type().Elem()
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
				body = []byte(strings.Replace(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE mapper PUBLIC "-//mybatis.org//DTD Mapper 3.0//EN"
       "https://github.com/kotlin2018/mbt/blob/master/mybatis.dtd">
<mapper>
	<insert id=""></insert>
	<delete id=""></delete>
	<update id=""></update>
	<select id=""></select>
</mapper>
`, "#{table}", snake(name), -1))
			}else {
				fieldType := bt.Field(0).Type
				if fieldType.Kind() != reflect.Struct || fieldType.String() == `time.Time` {
					it.log.SetPrefix("[Fatal] ")
					it.log.Fatalln(name + " 结构体的第一个字段必须是非 time.Time 结构体!")
				}
				content := ""
				cou := fieldType.NumField()
				for i := 0; i < cou; i++ {
					item := fieldType.Field(i)
					itemStr := strings.Replace(`<result column="#{column}" property="#{property}" langType="#{langType}" #{version} #{logic}/>`, "#{column}", snake(item.Name), -1)
					if item.Type.Name() == "Time" {
						itemStr = strings.Replace(itemStr, "#{langType}", "time."+item.Type.Name(), -1)
					} else {
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
					if i+1 < cou {
						content += "\n"
					}
				}
				body = []byte(strings.Replace(strings.Replace(`<?xml version="1.0" encoding="UTF-8"?>
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
`, "#{table}", snake(fieldType.Name()), -1), "#{resultMapBody}", content, -1))
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
	bytes, _ := ioutil.ReadFile(s)
	expressSymbol(&bytes)
	doc := newDocument()
	if err = doc.ReadFromBytes(bytes); err != nil {
		it.log.SetPrefix("[Fatal] ")
		it.log.Fatalln("解析 "+s+" 文件错误,err=",err)
	}
	mapperTree := make(map[string]*element)
	root := doc.SelectElement("mapper")
	e := root.ChildElements()
	num := len(e)
	for i:=0;i<num;i++ {
		if  e[i].Tag == "insert" ||
			e[i].Tag == "delete" ||
			e[i].Tag == "update" ||
			e[i].Tag == "select" ||
			e[i].Tag == "resultMap" ||
			e[i].Tag == "sql"{
			idValue := e[i].SelectAttrValue("id", "")
			if idValue == "" {
				idValue = e[i].Tag
			}
			if idValue != "" {
				oldItem := mapperTree[idValue]
				if oldItem != nil {
					it.log.SetPrefix("[Fatal] ")
					it.log.Fatalln(s+` 文件内的同一类 <` + e[i].Tag +`> 标签中，有且只能有一个 id = `+ idValue+ `! (即:id 的值在同一类标签中不能重复!)`)
				}
			}
			mapperTree[idValue] = e[i]
		}
	}
	for _, mapperXml := range mapperTree {
		ele := mapperXml.ChildElements()
		count := len(ele)
		for i:=0;i<count;i++ {
			it.includeElementReplace(ele[i], &mapperTree,s)
		}
	}
	it.decodeTree(mapperTree,bt,s)
	returnMap := make(map[string]*returnValue,0)
	names := bt.String()
	count := bt.NumField()
	for i := 0; i <count; i++ {
		fieldItem := bt.Field(i)
		funcType := fieldItem.Type
		funcName := fieldItem.Name
		funcKind := funcType.Kind()
		if !fieldItem.IsExported(){
			it.log.SetPrefix("[Fatal] ")
			it.log.Fatalln(names +"."+ funcName + `() 该方法首字母必须大写!`)
		}
		if funcKind == reflect.Ptr {
			it.log.SetPrefix("[Fatal] ")
			it.log.Fatalln(names +"."+ funcName + `() 该方法不能是指针类型!`)
		}
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
							it.log.Fatalln(names +"."+ funcName + `() 上的 tag "arg:" 的值的个数 != `+names +"."+ funcName + `() 的输入参数的个数!`)
						}
					}else if args == "" || tagLen != argsLen{
						it.log.SetPrefix("[Fatal] ")
						it.log.Fatalln(names +"."+ funcName + `() 上的 tag "arg:" 的值的个数 != `+names +"."+ funcName + `() 的输入参数的个数!`)
					}
				}
				if ftk == reflect.Struct && inType.String() != `time.Time` && inType.String() != `*time.Time`{
					customLen++
				}
			}
			if argsLen > 1 && customLen > 1 {
				it.log.SetPrefix("[Fatal] ")
				it.log.Fatalln(names +"."+ funcName + `() 这个函数结构体类型的输入参数有且只能有 1 个,现在它已经 > 1 个了! ([]Student这种输入参数可以有,但不能出现这种 func(s Student,u User)int64`)
			}
			if funcType.NumOut() != 1 {
				it.log.SetPrefix("[Fatal] ")
				it.log.Fatalln(names + "." + funcName + "() return num out must = 1!")
			}
			outType := funcType.Out(0)
			outTypeK := outType.Kind()
			outTypeS := outType.String()
			if outTypeK == reflect.Ptr || outTypeK == reflect.Interface || outTypeK == reflect.Map || outTypeK == reflect.Slice && outType.Elem().Kind() != reflect.Struct || outTypeS == `error`{
				it.log.SetPrefix("[Fatal] ")
				it.log.Fatalln(names + "." + funcName + "()' return value can not be a 'pointer' or 'interface' or 'error' or 'map' or '[]map' !")
			}
			returnMap[funcName] = &returnValue{}
			returnMap[funcName].value = &outType
			for _, v := range mapperTree {
				key := v.SelectAttrValue("id", "")
				if strings.EqualFold(key,funcName) {
					returnMap[funcName].xml = v
					returnMap[funcName].nodes = express{Proxy: &nodeExpress{}}.Parser(v.Child)
				}else {
					it.log.SetPrefix("[Fatal] ")
					it.log.Fatalln("在 "+ s +" 文件中没有找到 "+ names + "." + funcName +"() 对应的 id 值 "+ funcName)
				}
			}
			returnMap[funcName].name = names+"."+funcName+"() "
		}
	}
	it.data[mapperPtr]=returnMap
}
func expressSymbol(bytes *[]byte) {
	byteStr := string(*bytes)
	testRegex, _ := regexp.Compile(`test=".*"`)
	findList := testRegex.FindAllString(byteStr, -1)
	num := len(findList)
	for i:=0;i<num;i++ {
		newStr := findList[i]
		newStr = strings.Replace(newStr, "<", "&lt;", -1)
		newStr = strings.Replace(newStr, ">", "&gt;", -1)
		byteStr = strings.Replace(byteStr, findList[i], newStr, -1)
	}
	*bytes = []byte(byteStr)
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