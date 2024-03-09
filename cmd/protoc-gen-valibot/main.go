package main

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"

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

		var registry = make(map[string]string)
		for _, plan := range preprocessedFiles {
			for _, name := range plan.ExportedNames {
				registry[name] = plan.GenerateFileName
			}
		}

		// generate
		for _, plan := range preprocessedFiles {
			newFile := plugin.NewGeneratedFile(plan.GenerateFileName, ".")
			if err := render(newFile, plan, registry); err != nil {
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

func generatePlan(file *protogen.File, plan *Plan) error {
	plan.File = file
	plan.GenerateFileName = file.GeneratedFilenamePrefix + ".valibot.ts"

	if err := protocgenvalibot.Generate(file, &plan.Code); err != nil {
		return fmt.Errorf("generating file %s: %v", file.Desc.Path(), err)
	}

	plan.ExportedNames = make([]string, 0)
	for _, decl := range plan.Code.Content {
		plan.ExportedNames = append(plan.ExportedNames, decl.GetName())
	}

	return nil
}

func render(newFile *protogen.GeneratedFile, plan Plan, registry map[string]string) error {
	var b bytes.Buffer

	// Construct imports
	importMap := plan.Code.GetImportMap()
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
		values := lo.Keys(importMap[protocgenvalibot.PkgLookup])
		imports := make(map[string][]string)

		for _, name := range values {
			fname := registry[name]
			if fname == "" {
				return fmt.Errorf("cannot find file name for %s", name)
			}
			if imports[fname] == nil {
				imports[fname] = make([]string, 0)
			}
			imports[fname] = append(imports[fname], name)
		}

		for fname, names := range imports {
			if fname == plan.GenerateFileName {
				// skip, this is local identifier
				continue
			}

			pkg := "./" + strings.TrimSuffix(fname, ".ts")
			sort.Strings(names)
			b.WriteString(protocgenvalibot.Import{Pkg: pkg, Names: names}.String())
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
