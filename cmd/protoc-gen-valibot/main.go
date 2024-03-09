package main

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/k0kubun/pp/v3"
	protocgenvalibot "github.com/ka2n/protoc-gen-valibot"
	"github.com/samber/lo"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/pluginpb"
)

func main() {
	protogen.Options{}.Run(func(plugin *protogen.Plugin) error {
		plugin.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)

		// preprocess
		var preprocessedFiles []Plan
		for _, file := range plugin.Files {
			if file.Generate {
				var plan Plan
				if err := generatePlan(file, &plan); err != nil {
					return fmt.Errorf("generating file %s: %v", file.Desc.Path(), err)
				}
				preprocessedFiles = append(preprocessedFiles, plan)
			}
		}

		// generate
		for _, plan := range preprocessedFiles {
			newFile := plugin.NewGeneratedFile(plan.GenerateFileName, ".")
			if err := render(newFile, plan); err != nil {
				return fmt.Errorf("rendering file %s: %v", plan.File.Desc.Path(), err)
			}
		}

		return nil
	})
}

type Plan struct {
	Code             protocgenvalibot.Code
	ExportedNames    []string
	File             *protogen.File
	GenerateFileName string
}

func pathToGeneratedFile(protoPath string) string {
	return strings.TrimSuffix(protoPath, ".proto") + ".valibot.ts"
}

func pathToImportPath(protoPath string) string {
	return strings.TrimSuffix(pathToGeneratedFile(protoPath), ".ts")
}

func relativeImportPath(baseProto string, targetProto string) (string, error) {
	if baseProto == targetProto {
		return ".", nil
	}

	targetProto = strings.TrimSuffix(targetProto, ".proto") + ".ts"

	rel, err := filepath.Rel(filepath.Dir(baseProto), targetProto)
	if err != nil {
		return "", err
	}

	if !strings.HasPrefix(rel, ".") {
		rel = "./" + rel
	}
	return rel[:len(rel)-3], nil
}

func generatePlan(file *protogen.File, plan *Plan) error {
	plan.File = file
	plan.GenerateFileName = pathToGeneratedFile(file.Desc.Path())

	if err := protocgenvalibot.Generate(file, &plan.Code); err != nil {
		return fmt.Errorf("generating file %s: %v", file.Desc.Path(), err)
	}

	plan.ExportedNames = make([]string, 0)
	for _, decl := range plan.Code.Content {
		plan.ExportedNames = append(plan.ExportedNames, decl.GetName())
	}

	return nil
}

func render(newFile *protogen.GeneratedFile, plan Plan) error {
	var b bytes.Buffer

	// Construct imports
	importMap := plan.Code.GetImportMap()
	//debug(&b, importMap)
	importsKeys := lo.Keys(importMap)
	sort.Strings(importsKeys)
	for _, pkg := range importsKeys {
		if pkg == "" || pkg == protocgenvalibot.PkgLookup {
			// skip, this is local identifier or will be generated next step
			continue
		}
		values := lo.Keys(importMap[pkg])
		b.WriteString(protocgenvalibot.Import{Pkg: pkg, Names: values}.String())
		b.WriteString("\n\n")
	}

	// Construct import from other files
	if len(importMap[protocgenvalibot.PkgLookup]) > 0 {
		// Create map<import path, import names>
		imports := make(map[string][]string)

		for name, detail := range importMap[protocgenvalibot.PkgLookup] {
			rel, err := relativeImportPath(plan.File.Desc.Path(),  detail.FullName)
			if err != nil {
				return err
			}
			if rel == "." {
				// skip, this is local identifier
				continue
			}
			fname := pathToImportPath(rel)
			imports[fname] = append(imports[fname], name)
		}

		for fname, names := range imports {
			sort.Strings(names)
			b.WriteString(protocgenvalibot.Import{Pkg: fname, Names: names}.String())
			b.WriteString("\n\n")
		}
	}

	// Construct body
	for _, decl := range plan.Code.Content {
		b.WriteString(decl.String())
		b.WriteString("\n\n")
	}

	_, err := io.Copy(newFile, &b)
	if err != nil {
		return err
	}

	return nil
}

func debug(b *bytes.Buffer, v ...interface{}) {
	pp.Default.SetColoringEnabled(false)
	b.WriteString("/**\n")
	pp.Fprintln(b, v...)
	b.WriteString("*/\n")
	pp.Default.SetColoringEnabled(true)
}
