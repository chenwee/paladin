package paladin

import (
	"assalyn/paladin/cmn"
	"assalyn/paladin/frm/plog"
	"bytes"
	"fmt"
	"github.com/dave/jennifer/jen"
	"os"
	"reflect"
	"runtime"
	"sort"
)

type GoCodeBuilder struct {
	jfile      *jen.File
	codeDir    string
	dataDir    string
	fileName   string
	structName string // 给定的外层结构名, 为解决创建的匿名结构问题
}

func NewGoCodeBuilder(codeDir string, dataDir string, fileName string) *GoCodeBuilder {
	c := new(GoCodeBuilder)
	c.codeDir = codeDir
	c.dataDir = dataDir
	c.fileName = fileName
	c.jfile = jen.NewFile("dbc")
	c.jfile.HeaderComment("// Code generated by paladin. DO NOT EDIT.")
	return c
}

func (p *GoCodeBuilder) GenStructWithName(obj interface{}, structName string) string {
	p.structName = cmn.CamelName(structName)
	t := reflect.TypeOf(obj)
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	p.genType(t, p.structName, "")
	p.genValue()
	p.genGet()
	p.genGetAll()
	loadFuncName := p.genLoadFile()
	return loadFuncName
}

func (p *GoCodeBuilder) GenType(t reflect.Type, structName string) {
	p.structName = structName
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	p.genType(t, p.structName, "")
	p.genValue()
	p.genGet()
	p.genGetAll()
	p.genLoadFile()
}

// 这个代码问题还挺多的
func (p *GoCodeBuilder) genType(t reflect.Type, structName string, printPrefix string) {
	//fmt.Printf("%s[gen type %s]\n", printPrefix, t.Name())
	fields := make([]jen.Code, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		subField := t.Field(i)
		//fmt.Printf("%sfield %d %s %s\n", printPrefix+"  ", i, t.Field(i).Name, t.Field(i).Type.Kind().String())
		switch subField.Type.Kind() {
		case reflect.Struct:
			subStruct := structName + subField.Name
			p.genType(subField.Type, subStruct, printPrefix+"  ")
			fields[i] = jen.Id(subField.Name).Id(subStruct)

		case reflect.Map:
			elem := subField.Type.Elem()
			if elem.Kind() == reflect.Struct {
				mapSubStruct := structName + subField.Name
				p.genType(elem, mapSubStruct, printPrefix+"  ")
				fields[i] = jen.Id(subField.Name).Map(p.TypeToJenStatement(subField.Type.Key())).Id(mapSubStruct)
			} else {
				// 常规类型 int, float, string等
				fields[i] = p.AppendKeyword(jen.Id(subField.Name).Map(p.TypeToJenStatement(subField.Type.Key())), elem)
			}

		case reflect.Slice:
			elem := subField.Type.Elem()
			if elem.Kind() == reflect.Struct {
				sliceSubStruct := structName + subField.Name
				p.genType(elem, sliceSubStruct, printPrefix+"  ")
				fields[i] = jen.Id(subField.Name).Index().Id(sliceSubStruct)
			} else {
				// 常规类型 int, float, string等
				fields[i] = p.AppendKeyword(jen.Id(subField.Name).Index(), elem) //  Id(elem.Kind().String())
			}

		default:
			// 基础类型
			fields[i] = p.AppendKeyword(jen.Id(subField.Name), subField.Type)
		}
	}
	if structName == "" {
		structName = t.Name()
	}
	p.jfile.Type().Id(structName).Struct(fields...)
	if t.Kind() == reflect.Struct {
		p.jfile.Line()
	}
}

// 生成变量声明代码
func (p *GoCodeBuilder) genValue() {
	p.jfile.Var().Id("tbl" + p.structName).Map(jen.Int64()).Id("*" + p.structName)
}

// 生成GetXXX(id) *XXX
func (p *GoCodeBuilder) genGet() {
	p.jfile.Func().Id("Get" + p.structName).Params(jen.Id("id").Int64()).Id("*" + p.structName).Block(
		jen.Return().Id("tbl" + p.structName).Index(jen.Id("id")),
	).Line()
}

// 生成GetAllXXX() []*XXX
func (p *GoCodeBuilder) genGetAll() {
	p.jfile.Func().Id("GetAll" + p.structName).Params().Map(jen.Int64()).Id("*" + p.structName).Block(
		jen.Return().Id("tbl" + p.structName),
	).Line()
}

/*
	file, err := os.Open("bin/output/location.json")
	if err != nil {
		plog.Error(err)
		return
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	if err = decoder.Decode(&tblLocation); err != nil {
		plog.Error(err)
	}
*/
func (p *GoCodeBuilder) genLoadFile() string {
	funcName := "LoadFile" + p.structName
	p.jfile.Func().Id(funcName).Params().Block(
		jen.List(jen.Id("file"), jen.Id("err")).Op(":=").Qual("os", "Open").Call(jen.Lit(p.dataDir+"/"+p.fileName+".json")),
		jen.If(jen.Id("err").Op("!=").Id("nil")).Block(
			jen.Qual("fmt", "Println").Call(jen.List(jen.Lit("fail to open!!"), jen.Id("err"))),
			jen.Return(),
		),
		jen.Id("defer").Op(" ").Id("file").Dot("Close").Call(),
		jen.Id("decoder").Op(":=").Qual("encoding/json", "NewDecoder").Call(jen.Id("file")),
		jen.Id("err").Op("=").Id("decoder").Dot("Decode").Call(jen.Id("&tbl"+p.structName)),
		jen.If(jen.Id("err").Op("!=").Id("nil")).Block(
			jen.Qual("fmt", "Println").Call(jen.List(jen.Lit("fail to decode!!"), jen.Id("err"))),
			jen.Return(),
		),
	).Line()
	return funcName
}

// 生成init() { LoadFile(); }
func (p *GoCodeBuilder) GenInit(loadFuncName string) {
	p.jfile.Func().Id("init").Params().Block(
		jen.Qual("", loadFuncName).Call(),
	)
}

func (p *GoCodeBuilder) GenReloadAllFile(funcNames []string) string {
	stmts := make([]jen.Code, 0, len(funcNames))
	sort.Slice(funcNames, func(i, j int) bool {
		if funcNames[i] > funcNames[j] {
			funcNames[i], funcNames[j] = funcNames[j], funcNames[i]
		}
		return true
	})
	for _, name := range funcNames {
		stmt := jen.Qual("", name).Call()
		if stmt == nil {
			continue
		}
		stmts = append(stmts, stmt)
	}
	funcName := "ReloadAllFile"
	p.jfile.Func().Id(funcName).Params().Block(stmts...).Line()
	return funcName
}

// 输出
func (p *GoCodeBuilder) Output() {
	filename := p.codeDir + "/" + p.fileName + ".dbc.go"
	buf := &bytes.Buffer{}
	if err := p.jfile.Render(buf); err != nil {
		plog.Error("fail to p.jfile.Render!! ", err)
		return
	}
	var bs []byte
	if runtime.GOOS == "windows" {
		bs = bytes.Replace(buf.Bytes(), []byte("\n"), []byte("\r\n"), -1)
	} else if runtime.GOOS == "darwin" {
		bs = bytes.Replace(buf.Bytes(), []byte("\n"), []byte("\r"), -1)
	} else {
		bs = buf.Bytes()
	}
	if err := os.WriteFile(filename, bs, 0644); err != nil {
		plog.Error("fail to ioutil.WriteFile!! ", err)
		return
	}
}

func (p *GoCodeBuilder) DebugType(t reflect.Type, structName string) {
	p.structName = structName
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	p.genType(t, p.structName, "")
	fmt.Printf("%#v\n", p.jfile)
}

// 反射类型转换为jennifer语句
func (p *GoCodeBuilder) TypeToJenStatement(t reflect.Type) *jen.Statement {
	switch t.Kind() {
	case reflect.Bool:
		return jen.Bool()

	case reflect.Int,
		reflect.Int64:
		return jen.Int64()

	case reflect.Int32:
		return jen.Int32()

	case reflect.String:
		return jen.String()

	case reflect.Float32:
		return jen.Float32()

	case reflect.Float64:
		return jen.Float64()

	default:
		return jen.String()
	}
}

func (p *GoCodeBuilder) AppendKeyword(code *jen.Statement, t reflect.Type) *jen.Statement {
	switch t.Kind() {
	case reflect.Bool:
		return code.Bool()

	case reflect.Float32:
		return code.Float32()

	case reflect.Float64:
		return code.Float64()

	case reflect.Int,
		reflect.Int64:
		return code.Int64()

	case reflect.Int32:
		return code.Int32()

	case reflect.String:
		return code.String()

	default:
		plog.Panic("not support type", t)
		return nil
	}
}

func (p *GoCodeBuilder) structNameToValueName(structName string) (valueName string) {
	if structName == "" {
		return ""
	}
	bs := make([]byte, 0, len(structName))
	bs = append(bs, structName[0]+32)
	bs = append(bs, structName[1:]...)
	return string(bs)
}
