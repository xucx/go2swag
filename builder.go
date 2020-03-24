package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-openapi/spec"
	"golang.org/x/tools/go/ast/astutil"
)

type builder struct {
	input *spec.Swagger
	ctx   *scanner
}

func build(ctx *scanner, input *spec.Swagger) (*spec.Swagger, error) {
	if input == nil {
		input = new(spec.Swagger)
		input.Swagger = "2.0"
	}

	if input.Paths == nil {
		input.Paths = new(spec.Paths)
	}
	if input.Definitions == nil {
		input.Definitions = make(map[string]spec.Schema)
	}
	if input.Responses == nil {
		input.Responses = make(map[string]spec.Response)
	}
	if input.Extensions == nil {
		input.Extensions = make(spec.Extensions)
	}

	b := builder{
		input: input,
		ctx:   ctx,
	}

	b.buildMeta()
	b.buildRoute()
	b.buildReq()
	b.buildAns()

	return b.input, nil
}

func (self *builder) buildMeta() {
	metas := map[string]string{}
	for _, meta := range self.ctx.metas {
		var key, value string
		for _, lines := range meta.Comments.List {
			for _, line := range strings.Split(lines.Text, "\n") {
				line = strings.TrimLeft(line, "/")
				pos := strings.Index(line, ":")
				if pos != -1 {
					if key != "" {
						metas[key] = value
					}
					key = strings.TrimSpace(line[:pos])
					value = strings.TrimSpace(line[pos+1:])
				} else {
					if line != "" {
						value += "\n" + line
					}
				}
			}
			if key != "" {
				metas[key] = value
			}
		}
	}

	if self.input.Info == nil {
		self.input.Info = &spec.Info{}
	}
	for k, v := range metas {
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "title":
			self.input.Info.Title = v
		case "version":
			self.input.Info.Version = v
		case "schemes":
			vv := strings.Split(v, ",")
			for k, v := range vv {
				vv[k] = strings.TrimSpace(v)
			}
			self.input.Schemes = vv
		case "host":
			self.input.Host = v
		case "basepath":
			self.input.BasePath = v
		case "consumes":
			vv := strings.Split(v, ",")
			for k, v := range vv {
				vv[k] = strings.TrimSpace(v)
			}
			self.input.Consumes = vv
		case "produces":
			vv := strings.Split(v, ",")
			for k, v := range vv {
				vv[k] = strings.TrimSpace(v)
			}
			self.input.Produces = vv
		}
	}
}

func (self *builder) buildRoute() {
	for _, r := range self.ctx.routes {
		if self.input.Paths.Paths == nil {
			self.input.Paths.Paths = make(map[string]spec.PathItem)
		}

		pthObj := self.input.Paths.Paths[r.Path]

		op := new(spec.Operation)
		op.ID = r.ID

		switch strings.ToUpper(r.Method) {
		case "GET":
			if pthObj.Get != nil {
				if r.ID == pthObj.Get.ID {
					op = pthObj.Get
				} else {
					pthObj.Get = op
				}
			} else {
				pthObj.Get = op
			}

		case "POST":
			if pthObj.Post != nil {
				if r.ID == pthObj.Post.ID {
					op = pthObj.Post
				} else {
					pthObj.Post = op
				}
			} else {
				pthObj.Post = op
			}

		case "PUT":
			if pthObj.Put != nil {
				if r.ID == pthObj.Put.ID {
					op = pthObj.Put
				} else {
					pthObj.Put = op
				}
			} else {
				pthObj.Put = op
			}

		case "PATCH":
			if pthObj.Patch != nil {
				if r.ID == pthObj.Patch.ID {
					op = pthObj.Patch
				} else {
					pthObj.Patch = op
				}
			} else {
				pthObj.Patch = op
			}

		case "HEAD":
			if pthObj.Head != nil {
				if r.ID == pthObj.Head.ID {
					op = pthObj.Head
				} else {
					pthObj.Head = op
				}
			} else {
				pthObj.Head = op
			}

		case "DELETE":
			if pthObj.Delete != nil {
				if r.ID == pthObj.Delete.ID {
					op = pthObj.Delete
				} else {
					pthObj.Delete = op
				}
			} else {
				pthObj.Delete = op
			}

		case "OPTIONS":
			if pthObj.Options != nil {
				if r.ID == pthObj.Options.ID {
					op = pthObj.Options
				} else {
					pthObj.Options = op
				}
			} else {
				pthObj.Options = op
			}
		}

		for _, c := range r.Remaining.List {
			for _, line := range strings.Split(c.Text, "\n") {
				if op.Summary == "" {
					op.Summary = self.commentLineClear(line)
				} else {
					if op.Description != "" {
						op.Description += "\n"
					}
					op.Description += self.commentLineClear(line)
				}
			}
		}

		op.Tags = r.Tags
		self.input.Paths.Paths[r.Path] = pthObj
	}
}

func (self *builder) buildReq() {
	for _, req := range self.ctx.reqs {
		route, ok := self.ctx.routes[req.ID]
		if !ok {
			continue
		}

		op := self.routerOperator(route.Path, route.Method)
		if op != nil {

			pathParms := strings.Split(route.Path, "/")
			for _, p := range pathParms {
				if strings.HasPrefix(p, "{") && strings.HasSuffix(p, "}") {
					op.AddParam(spec.PathParam(p[1:len(p)-1]).Typed("string", ""))
				} else if strings.HasPrefix(p, ":") {
					op.AddParam(spec.PathParam(p[1:]).Typed("string", ""))
				}
			}

			if route.Method == "GET" {
				switch tpe := req.Type.Obj().Type().(type) {
				case *types.Named:
					o := tpe.Obj()
					switch stpe := o.Type().Underlying().(type) {
					case *types.Struct:

						for i := 0; i < stpe.NumFields(); i++ {
							fld := stpe.Field(i)
							// tg := stpe.Tag(i)

							if fld.Embedded() {
								continue
							}

							if !fld.Exported() {
								continue
							}

							var afld *ast.Field
							ans, _ := astutil.PathEnclosingInterval(req.File, fld.Pos(), fld.Pos())
							for _, an := range ans {
								at, valid := an.(*ast.Field)
								if !valid {
									continue
								}

								afld = at
								break
							}

							if afld == nil {
								continue
							}

							name, ignore := parseJsonTags(afld)
							if ignore {
								continue
							}

							var queryParam *spec.Parameter
							switch titpe := fld.Type().(type) {
							case *types.Basic:
								switch titpe.String() {
								case "bool":
									queryParam = spec.QueryParam(name).Typed("boolean", "")
								case "byte":
									queryParam = spec.QueryParam(name).Typed("integer", "uint8")
								case "complex128", "complex64":
								case "error":
									queryParam = spec.QueryParam(name).Typed("string", "")
								case "float32":
									queryParam = spec.QueryParam(name).Typed("number", "float")
								case "float64":
									queryParam = spec.QueryParam(name).Typed("number", "double")
								case "int":
									queryParam = spec.QueryParam(name).Typed("integer", "int64")
								case "int16":
									queryParam = spec.QueryParam(name).Typed("integer", "int16")
								case "int32":
									queryParam = spec.QueryParam(name).Typed("integer", "int32")
								case "int64":
									queryParam = spec.QueryParam(name).Typed("integer", "int64")
								case "int8":
									queryParam = spec.QueryParam(name).Typed("integer", "int8")
								case "rune":
									queryParam = spec.QueryParam(name).Typed("integer", "int32")
								case "string":
									queryParam = spec.QueryParam(name).Typed("string", "")
								case "uint":
									queryParam = spec.QueryParam(name).Typed("integer", "uint64")
								case "uint16":
									queryParam = spec.QueryParam(name).Typed("integer", "uint16")
								case "uint32":
									queryParam = spec.QueryParam(name).Typed("integer", "uint32")
								case "uint64":
									queryParam = spec.QueryParam(name).Typed("integer", "uint64")
								case "uint8":
									queryParam = spec.QueryParam(name).Typed("integer", "uint8")
								case "uintptr":
									queryParam = spec.QueryParam(name).Typed("integer", "uint64")
								default:
								}
							}

							if queryParam != nil {
								op.AddParam(queryParam.WithDescription(afld.Comment.Text()))
							}
						}
					}
				}

			} else {
				schema := self.input.Definitions[req.Name]
				if self.buildSchemaFromDecl(req.Name, req, &schema) != nil {
					continue
				}
				self.input.Definitions[req.Name] = schema

				commentlines := []string{}
				for _, c := range req.Comments.List {
					for _, line := range strings.Split(c.Text, "\n") {
						commentlines = append(commentlines, self.commentLineClear(line))
					}
				}

				desc := " "
				if len(commentlines) > 1 {
					desc = strings.Join(commentlines[1:], "\n")
				}

				body := op.AddParam(spec.BodyParam("Body", spec.RefSchema("#/definitions/"+req.Name)))
				body.Description = desc
			}
		}
	}
}

func (self *builder) buildAns() {
	for _, ans := range self.ctx.anses {

		route, ok := self.ctx.routes[ans.ID]
		if !ok {
			continue
		}

		op := self.routerOperator(route.Path, route.Method)
		if op != nil {

			schema := self.input.Definitions[ans.Name]
			if self.buildSchemaFromDecl(ans.Name, ans, &schema) != nil {
				continue
			}
			self.input.Definitions[ans.Name] = schema

			commentlines := []string{}
			for _, c := range ans.Comments.List {
				for _, line := range strings.Split(c.Text, "\n") {
					commentlines = append(commentlines, self.commentLineClear(line))
				}
			}

			desc := " "
			if len(commentlines) > 1 {
				desc = strings.Join(commentlines[1:], "\n")
			}

			response := spec.NewResponse()
			response.WithSchema(spec.RefSchema("#/definitions/" + ans.Name))
			response.WithDescription(desc)
			op.RespondsWith(ans.Code, response)
		}
	}
}

func (self *builder) buildSchemaFromDecl(name string, decl *declParser, schema *spec.Schema) error {
	switch tpe := decl.Type.Obj().Type().(type) {
	case *types.Basic:
	case *types.Named:
		o := tpe.Obj()
		if o != nil {
			if o.Pkg().Name() == "time" && o.Name() == "Time" {
				schema.Typed("string", "date-time")
				return nil
			}

			for {
				ti := decl.Pkg.TypesInfo.Types[decl.Spec.Type]

				if ti.IsBuiltin() {
					break
				}
				if ti.IsType() {
					if err := self.buildSchemaFromType(decl, ti.Type, schema); err != nil {
						return err
					}
					break
				}
			}
		}
	}
	return nil
}

func (self *builder) buildSchemaFromType(decl *declParser, tpe types.Type, schema *spec.Schema) error {

	switch titpe := tpe.(type) {
	case *types.Basic:
		switch titpe.String() {
		case "bool":
			schema.Typed("boolean", "")
		case "byte":
			schema.Typed("integer", "uint8")
		case "complex128", "complex64":
		case "error":
			// TODO: error is often marshalled into a string but not always (e.g. errors package creates
			// errors that are marshalled into an empty object), this could be handled the same way
			// custom JSON marshallers are handled (in future)
			schema.Typed("string", "")
		case "float32":
			schema.Typed("number", "float")
		case "float64":
			schema.Typed("number", "double")
		case "int":
			schema.Typed("integer", "int64")
		case "int16":
			schema.Typed("integer", "int16")
		case "int32":
			schema.Typed("integer", "int32")
		case "int64":
			schema.Typed("integer", "int64")
		case "int8":
			schema.Typed("integer", "int8")
		case "rune":
			schema.Typed("integer", "int32")
		case "string":
			schema.Typed("string", "")
		case "uint":
			schema.Typed("integer", "uint64")
		case "uint16":
			schema.Typed("integer", "uint16")
		case "uint32":
			schema.Typed("integer", "uint32")
		case "uint64":
			schema.Typed("integer", "uint64")
		case "uint8":
			schema.Typed("integer", "uint8")
		case "uintptr":
			schema.Typed("integer", "uint64")
		default:
		}
	case *types.Pointer:
		return self.buildSchemaFromType(decl, titpe.Elem(), schema)
	case *types.Struct:
		self.buildSchemaFromStruct(decl, titpe, schema)
	case *types.Slice:
		if schema.Items == nil {
			schema.Items = new(spec.SchemaOrArray)
		}
		if schema.Items.Schema == nil {
			schema.Items.Schema = new(spec.Schema)
		}
		schema.Typed("array", "")
		return self.buildSchemaFromType(decl, titpe.Elem(), schema.Items.Schema)
	case *types.Array:
		if schema.Items == nil {
			schema.Items = new(spec.SchemaOrArray)
		}
		if schema.Items.Schema == nil {
			schema.Items.Schema = new(spec.Schema)
		}
		schema.Typed("array", "")
		return self.buildSchemaFromType(decl, titpe.Elem(), schema.Items.Schema)
	case *types.Named:
		switch utitpe := tpe.Underlying().(type) {
		case *types.Struct:
			return self.buildSchemaFromStruct(decl, utitpe, schema)
		}
	default:
		fmt.Println("buildSchemaFromType not found")
	}
	return nil
}

func (self *builder) buildSchemaFromStruct(decl *declParser, st *types.Struct, schema *spec.Schema) error {
	if schema.Properties == nil {
		schema.Properties = make(map[string]spec.Schema)
	}
	schema.Typed("object", "")

	for i := 0; i < st.NumFields(); i++ {
		fld := st.Field(i)
		// tg := st.Tag(i)

		if fld.Embedded() {
			continue
		}

		if !fld.Exported() {
			continue
		}

		var afld *ast.Field
		ans, _ := astutil.PathEnclosingInterval(decl.File, fld.Pos(), fld.Pos())
		for _, an := range ans {
			at, valid := an.(*ast.Field)
			if !valid {
				continue
			}

			afld = at
			break
		}

		if afld == nil {
			continue
		}

		name, ignore := parseJsonTags(afld)
		if ignore {
			continue
		}

		ps := schema.Properties[name]
		self.buildSchemaFromType(decl, fld.Type(), &ps)
		ps.Description = afld.Comment.Text()
		schema.Properties[name] = ps

	}

	return nil
}

func (self builder) commentLineClear(line string) string {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "//") {
		line = line[2:]
	}
	line = strings.TrimSpace(line)
	return line
}

func (self builder) routerOperator(path, method string) *spec.Operation {
	var op *spec.Operation
	if pthObj, ok := self.input.Paths.Paths[path]; ok {
		switch strings.ToUpper(method) {
		case "GET":
			op = pthObj.Get
		case "POST":
			op = pthObj.Post
		case "PUT":
			op = pthObj.Put
		case "PATCH":
			op = pthObj.Patch
		case "HEAD":
			op = pthObj.Head
		case "DELETE":
			op = pthObj.Delete
		case "OPTIONS":
			op = pthObj.Options
		}
	}
	return op
}

func parseJsonTags(field *ast.Field) (name string, ignore bool) {
	if len(field.Names) > 0 {
		name = field.Names[0].Name
	}
	if field.Tag == nil || len(strings.TrimSpace(field.Tag.Value)) == 0 {
		return name, false
	}

	tv, err := strconv.Unquote(field.Tag.Value)
	if err != nil {
		return name, false
	}

	if strings.TrimSpace(tv) != "" {
		st := reflect.StructTag(tv)
		jsonParts := strings.Split(st.Get("json"), ",")
		switch jsonParts[0] {
		case "-":
			return name, true
		case "":
			return name, false
		default:
			return jsonParts[0], false
		}
	}
	return name, false
}
