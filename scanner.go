package main

import (
	"go/ast"
	"go/types"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

const pkgLoadMode = packages.NeedName | packages.NeedFiles | packages.NeedImports | packages.NeedDeps | packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo

type node uint32

const (
	metaNode node = 1 << iota
	routeNode
	reqNode
	ansNode
)

type scanner struct {
	pkgs   map[string]*packages.Package
	metas  []*meteParser
	routes map[string]*routeParser
	reqs   map[string]*declParser
	anses  map[string]*declParser
}

func scan(pkgs []*packages.Package) (*scanner, error) {
	s := scanner{
		pkgs:   map[string]*packages.Package{},
		metas:  []*meteParser{},
		routes: map[string]*routeParser{},
		reqs:   map[string]*declParser{},
		anses:  map[string]*declParser{},
	}

	for _, pkg := range pkgs {
		if _, known := s.pkgs[pkg.PkgPath]; known {
			continue
		}
		s.pkgs[pkg.PkgPath] = pkg

		if err := s.processPackage(pkg); err != nil {
			return nil, err
		}
		if err := s.walkImports(pkg); err != nil {
			return nil, err
		}
	}

	return &s, nil

}

func (self *scanner) processPackage(pkg *packages.Package) error {
	for _, file := range pkg.Syntax {
		n, err := self.detectNodes(file)
		if err != nil {
			return err
		}

		if n&metaNode != 0 {
			self.metas = append(self.metas, &meteParser{Comments: file.Doc})
		}

		if n&routeNode != 0 {
			for _, cmts := range file.Comments {
				route := parseRoute(cmts.List)
				if route.Method != "" {
					self.routes[route.ID] = route
				}
			}
		}

		if n&reqNode != 0 || n&ansNode != 0 {
			for _, dt := range file.Decls {
				switch fd := dt.(type) {
				case *ast.BadDecl:
					continue
				case *ast.FuncDecl:
					continue
				case *ast.GenDecl:
					decls := parseDecl(pkg, file, n, fd)
					for _, decl := range decls {
						if decl.HasReqAnno() {
							self.reqs[decl.Name] = decl
						}
						if decl.HasAnsAnno() {
							self.anses[decl.Name] = decl
						}
					}
				}
			}
		}
	}

	return nil
}

func (self *scanner) walkImports(pkg *packages.Package) error {
	for _, v := range pkg.Imports {
		if _, known := self.pkgs[v.PkgPath]; known {
			continue
		}

		self.pkgs[v.PkgPath] = v
		if err := self.processPackage(v); err != nil {
			return err
		}
		if err := self.walkImports(v); err != nil {
			return err
		}
	}
	return nil
}

func (self *scanner) detectNodes(file *ast.File) (node, error) {
	var n node
	for _, comments := range file.Comments {
		for _, cline := range comments.List {
			if cline == nil {
				continue
			}

			matches := rxSwag.FindStringSubmatch(cline.Text)
			if len(matches) < 2 {
				continue
			}

			switch matches[1] {
			case "meta":
				n |= metaNode
			case "route":
				n |= routeNode
			case "req":
				n |= reqNode
			case "ans":
				n |= ansNode
			}
		}
	}
	return n, nil
}

type meteParser struct {
	Comments *ast.CommentGroup
}

type routeParser struct {
	ID, Method, Path string
	Tags             []string
	Remaining        *ast.CommentGroup
}

func parseRoute(lines []*ast.Comment) *routeParser {
	route := routeParser{}

	justMatched := false
	for _, cmt := range lines {
		txt := cmt.Text
		for _, line := range strings.Split(txt, "\n") {
			matches := rxRoute.FindStringSubmatch(line)
			if len(matches) > 4 {
				route.ID, route.Method, route.Path = matches[1], strings.ToUpper(matches[2]), matches[3]
				route.Tags = rxSpace.Split(matches[4], -1)
				if len(matches[4]) == 0 {
					route.Tags = nil
				}
				justMatched = true
			} else if route.Method != "" {
				if route.Remaining == nil {
					route.Remaining = new(ast.CommentGroup)
				}
				if !justMatched || strings.TrimSpace(rxStripComments.ReplaceAllString(line, "")) != "" {
					cc := new(ast.Comment)
					cc.Slash = cmt.Slash
					cc.Text = line
					route.Remaining.List = append(route.Remaining.List, cc)
					justMatched = false
				}
			}
		}
	}

	return &route
}

type declParser struct {
	ID   string
	Name string
	Code int

	Comments *ast.CommentGroup
	Type     *types.Named
	Ident    *ast.Ident
	Spec     *ast.TypeSpec
	File     *ast.File
	Pkg      *packages.Package
	HasReq   bool
	HasAns   bool
}

func parseDecl(pkg *packages.Package, file *ast.File, n node, gd *ast.GenDecl) []*declParser {
	decls := []*declParser{}

	for _, sp := range gd.Specs {
		switch ts := sp.(type) {
		case *ast.ValueSpec:
			return nil
		case *ast.ImportSpec:
			return nil
		case *ast.TypeSpec:

			def, ok := pkg.TypesInfo.Defs[ts.Name]
			if !ok {
				continue
			}

			nt, isNamed := def.Type().(*types.Named)
			if !isNamed {
				continue
			}

			decls = append(decls, &declParser{
				Comments: gd.Doc,
				Type:     nt,
				Ident:    ts.Name,
				Spec:     ts,
				File:     file,
				Pkg:      pkg,
			})
		}
	}

	return decls
}

func (self *declParser) HasReqAnno() bool {
	if self.HasReq {
		return true
	}
	if self.Comments == nil {
		return false
	}
	for _, cmt := range self.Comments.List {
		for _, ln := range strings.Split(cmt.Text, "\n") {
			matches := rxReq.FindStringSubmatch(ln)
			if len(matches) > 0 {
				self.ID = matches[1]
				self.Name = matches[1]
				self.HasReq = true
				return true
			}
		}
	}
	return false
}

func (self *declParser) HasAnsAnno() bool {
	if self.HasAns {
		return true
	}
	if self.Comments == nil {
		return false
	}
	for _, cmt := range self.Comments.List {
		for _, ln := range strings.Split(cmt.Text, "\n") {
			matches := rxAns.FindStringSubmatch(ln)
			if len(matches) > 0 {
				self.ID = matches[1]
				self.Code, _ = strconv.Atoi(matches[2])
				self.Name = matches[1] + "-" + matches[2]
				self.HasAns = true
				return true
			}
		}
	}
	return false
}
