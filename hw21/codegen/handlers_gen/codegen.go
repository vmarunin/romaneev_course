package main

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"log"
	"os"
	"reflect"
	"strconv"
	"strings"
	"text/template"
)

// код писать тут
type HandlerMethodInfo struct {
	Url        string `json:"url"`
	HTTPMethod string `json:"method"`
	Auth       bool   `json:"auth"`
	MethodName string
	ParamType  string
}
type ParamFieldInfo struct {
	Name                                        string
	ParamName                                   string
	ParamType                                   string
	Default                                     string
	Min, Max                                    int
	Enum                                        []string
	IsRequired, IsMin, IsMax, IsEnum, IsDefault bool
}
type ParamStructInfo struct {
	Fields []ParamFieldInfo
}

type CGDataType struct {
	Handlers    map[string][]HandlerMethodInfo
	Params      map[string]ParamStructInfo
	PackageName string
}

// var cgStructs = map[string][]HandlerMethodInfo{}
// var cgParamTypes = map[string]ParamStructInfo{}
// var packageName string

func main() {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, os.Args[1], nil, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}

	out, _ := os.Create(os.Args[2])
	data := &CGDataType{
		Handlers:    map[string][]HandlerMethodInfo{},
		Params:      map[string]ParamStructInfo{},
		PackageName: node.Name.Name,
	}

	ProcessFuncDecl(node, data)
	ProcessTypeDecl(node, data)

	Generate(out, data)
}

func Generate(out io.Writer, data *CGDataType) {
	tpl := template.New("package.go.tpl")
	tpl.Funcs(template.FuncMap{"StringsJoin": strings.Join})
	_, err := tpl.ParseFiles("package.go.tpl")
	if err != nil {
		panic(err)
	}

	var buf bytes.Buffer
	err = tpl.Execute(&buf, data)
	if err != nil {
		panic(err)
	}
	code, err := format.Source(buf.Bytes())
	if err != nil {
		panic(err)
	}
	out.Write(code)
}

func ProcessTypeDecl(node *ast.File, data *CGDataType) {
	for _, f := range node.Decls {
		g, ok := f.(*ast.GenDecl)
		if !ok {
			continue
		}
		// SPECS_LOOP:
		for _, spec := range g.Specs {
			currType, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			typeName := currType.Name.Name
			// typeInfo, ok := cgParamTypes[typeName]
			_, ok = data.Params[typeName]
			if !ok {
				continue
			}

			currStruct, ok := currType.Type.(*ast.StructType)
			if !ok {
				continue
			}

			structInfo := ParamStructInfo{Fields: []ParamFieldInfo{}}
			// FIELDS_LOOP:
			for _, field := range currStruct.Fields.List {
				if field.Tag == nil {
					continue
				}
				fieldTags := reflect.StructTag(field.Tag.Value[1 : len(field.Tag.Value)-1])
				fieldTag, ok := fieldTags.Lookup("apivalidator")
				if !ok {
					continue
				}
				paramInfo := ParamFieldInfo{}
				paramInfo.Name = field.Names[0].Name
				paramInfo.ParamName = strings.ToLower(paramInfo.Name)
				paramInfo.ParamType = field.Type.(*ast.Ident).Name
				for _, tagRec := range strings.Split(fieldTag, ",") {
					if tagRec == "required" {
						paramInfo.IsRequired = true
						continue
					}
					kv := strings.SplitN(tagRec, "=", 2)
					if kv[0] == "paramname" {
						paramInfo.ParamName = kv[1]
						continue
					}
					if kv[0] == "min" {
						paramInfo.Min, _ = strconv.Atoi(kv[1])
						paramInfo.IsMin = true
						continue
					}
					if kv[0] == "max" {
						paramInfo.Max, _ = strconv.Atoi(kv[1])
						paramInfo.IsMax = true
						continue
					}
					if kv[0] == "default" {
						paramInfo.Default = kv[1]
						paramInfo.IsDefault = true
						continue
					}
					if kv[0] == "enum" {
						paramInfo.Enum = strings.Split(kv[1], "|")
						paramInfo.IsEnum = true
						continue
					}
				}
				if paramInfo.IsRequired {
					paramInfo.IsDefault = false
				}
				structInfo.Fields = append(structInfo.Fields, paramInfo)
			}
			data.Params[typeName] = structInfo
		}
	}
}

func ProcessFuncDecl(node *ast.File, data *CGDataType) {
	for _, f := range node.Decls {
		funcDecl, ok := f.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if funcDecl.Doc == nil {
			continue
		}
		if funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 {
			continue
		}
		if len(funcDecl.Type.Params.List) < 2 {
			continue
		}
		jsonText := ""
		for _, comment := range funcDecl.Doc.List {
			if strings.HasPrefix(comment.Text, "// apigen:api") {
				jsonText = comment.Text[14:]
				break
			}
		}
		var hInfo HandlerMethodInfo
		if len(jsonText) == 0 || json.Unmarshal([]byte(jsonText), &hInfo) != nil {
			continue
		}
		hInfo.MethodName = funcDecl.Name.String()

		hInfo.ParamType = funcDecl.Type.Params.List[1].Type.(*ast.Ident).Name
		data.Params[hInfo.ParamType] = ParamStructInfo{Fields: []ParamFieldInfo{}}

		handlerType := ""
		sExp, ok := funcDecl.Recv.List[0].Type.(*ast.StarExpr)
		if ok {
			handlerType = sExp.X.(*ast.Ident).Name
		} else {
			handlerType = funcDecl.Recv.List[0].Type.(*ast.Ident).Name
		}

		hList, ok := data.Handlers[handlerType]
		if !ok {
			hList = []HandlerMethodInfo{}
		}
		hList = append(hList, hInfo)
		data.Handlers[handlerType] = hList
	}
}
