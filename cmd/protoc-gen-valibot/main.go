package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bufbuild/protoplugin"
	"github.com/k0kubun/pp/v3"
	protocgenvalibot "github.com/ka2n/protoc-gen-valibot"
	"github.com/samber/lo"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func main() {
	protoplugin.Main(protoplugin.HandlerFunc(handle))
}

func handle(
	_ context.Context,
	_ protoplugin.PluginEnv,
	responseWriter protoplugin.ResponseWriter,
	request protoplugin.Request,
) error {
	responseWriter.SetFeatureProto3Optional()

	var flags flag.FlagSet
	optSchemaSuffix := flags.String("schema_suffix", "Schema", "suffix for schema name")
	request.Parameter()

	for _, param := range strings.Split(request.Parameter(), ",") {
		var value string
		if i := strings.Index(param, "="); i >= 0 {
			value = param[i+1:]
			param = param[0:i]
		}
		if err := flags.Set(param, value); err != nil {
			return err
		}
	}

	var opt protocgenvalibot.GenerateOptions
	opt.SchemaSuffix = *optSchemaSuffix

	fileDescs, err := request.FileDescriptorsToGenerate()
	if err != nil {
		return err
	}
	// preprocess
	var preprocessedFiles []Plan
	for _, file := range fileDescs {
		var plan Plan
		if err := generatePlan(file, &plan, opt); err != nil {
			return fmt.Errorf("generating file %s: %v", file.Path(), err)
		}
		preprocessedFiles = append(preprocessedFiles, plan)
	}

	// generate
	buf := new(bytes.Buffer)
	for _, plan := range preprocessedFiles {
		if err := render(buf, plan); err != nil {
			return fmt.Errorf("rendering file %s: %v", plan.File.Path(), err)
		}
		responseWriter.AddFile(
			plan.GenerateFileName,
			buf.String(),
		)
		buf.Reset()
	}

	return nil
}

type Plan struct {
	Code             protocgenvalibot.File
	ExportedNames    []string
	File             protoreflect.FileDescriptor
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

func generatePlan(file protoreflect.FileDescriptor, plan *Plan, opt protocgenvalibot.GenerateOptions) error {
	plan.File = file
	plan.GenerateFileName = pathToGeneratedFile(file.Path())

	if err := protocgenvalibot.Generate(file, &plan.Code, opt); err != nil {
		return fmt.Errorf("generating file %s: %v", file.Path(), err)
	}

	plan.ExportedNames = make([]string, 0)
	for _, decl := range plan.Code.Content {
		plan.ExportedNames = append(plan.ExportedNames, decl.GetName())
	}

	return nil
}

func render(newFile io.Writer, plan Plan) error {
	var b bytes.Buffer

	b.WriteString("// Code generated by protoc-gen-valibot. DO NOT EDIT.\n")
	b.WriteString("// source: ")
	b.WriteString(plan.File.Path())
	b.WriteString("\n\n")

	// Ignore formatters (Prettier, ESLint, Biome etc.)
	b.WriteString("// eslint-disable\n")
	b.WriteString("// biome-ignore format lint: \n")

	// Construct imports
	importMap := plan.Code.GetImportMap()
	if len(importMap) > 0 {
		//debug(&b, importMap)
		importKeys := sortedKeys(importMap)
		for _, pkg := range importKeys {
			if pkg == "" || pkg == protocgenvalibot.PkgLookup {
				// skip, this is local identifier or will be generated next step
				continue
			}
			values := lo.Keys(importMap[pkg])
			b.WriteString(protocgenvalibot.Import{Pkg: pkg, Names: values}.String())
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Construct import from other files
	if len(importMap[protocgenvalibot.PkgLookup]) > 0 {
		// Create map<import path, import names>
		imports := make(map[string][]string)

		for name, detail := range importMap[protocgenvalibot.PkgLookup] {
			rel, err := relativeImportPath(plan.File.Path(), detail.FullName)
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

		importKeys := sortedKeys(imports)
		for _, fname := range importKeys {
			names := imports[fname]
			sort.Strings(names)
			b.WriteString(protocgenvalibot.Import{Pkg: fname, Names: names}.String())
			b.WriteString("\n")
		}

		b.WriteString("\n")
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

func sortedKeys[T any](m map[string]T) []string {
	keys := lo.Keys(m)
	sort.Strings(keys)
	return keys
}

func debug(b *bytes.Buffer, v ...interface{}) {
	pp.Default.SetColoringEnabled(false)
	b.WriteString("/**\n")
	pp.Fprintln(b, v...)
	b.WriteString("*/\n")
	pp.Default.SetColoringEnabled(true)
}
