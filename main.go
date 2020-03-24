package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/go-openapi/loads"
	"github.com/go-openapi/spec"
	"github.com/voxelbrain/goptions"
	"golang.org/x/tools/go/packages"
	"gopkg.in/yaml.v2"
)

type options struct {
	In     string   `goptions:"-i, description='in'"`
	Out    string   `goptions:"-o, description='out'"`
	Models []string `goptions:"-m, description='models'"`
}

func main() {

	opt := options{
		Out:    "./swagger.yaml",
		Models: []string{"./..."},
	}
	goptions.ParseAndFail(&opt)

	pkgs, err := packages.Load(&packages.Config{
		Dir:   ".",
		Mode:  pkgLoadMode,
		Tests: false,
	}, opt.Models...)
	if err != nil {
		fmt.Println(err)
		return
	}

	scanner, err := scan(pkgs)
	if err != nil {
		fmt.Println(err)
		return
	}

	swag, err := build(scanner, load(opt.In))
	if err != nil {
		fmt.Println(err)
		return
	}

	save(swag, true, opt.Out)
}

func load(input string) *spec.Swagger {
	if fi, err := os.Stat(input); err == nil {
		if !fi.IsDir() {
			if sp, err := loads.Spec(input); err == nil {
				return sp.Spec()
			}
		}
	}
	return nil
}

func save(swspec *spec.Swagger, pretty bool, output string) error {
	var b []byte
	var err error

	if strings.HasSuffix(output, "yml") || strings.HasSuffix(output, "yaml") {
		b, err = json.Marshal(swspec)
		if err != nil {
			return err
		}

		var jsonObj interface{}
		if err := yaml.Unmarshal(b, &jsonObj); err != nil {
			return err
		}

		b, err = yaml.Marshal(jsonObj)
	} else {
		if pretty {
			b, err = json.MarshalIndent(swspec, "", "  ")
		}
		b, err = json.Marshal(swspec)
	}

	if err != nil {
		return err
	}

	if output == "" {
		fmt.Println(string(b))
		return nil
	}

	return ioutil.WriteFile(output, b, 0644)
}
