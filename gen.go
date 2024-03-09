package protocgenvalibot

import (
	"fmt"
	"sort"
	"strings"
)

type Node interface {
	String() string
}

type Callable struct {
	Name string
	Pkg  string
	Args []Node
}

const PkgLookup = ":lookup:"

type Object struct {
	Fields map[string]Node
}

type Array struct {
	Elements []Node
}

type String struct {
	Value string
}

type Number struct {
	Value int
}

func (m Callable) String() string {
	args := make([]string, 0, len(m.Args))
	for _, arg := range m.Args {
		args = append(args, arg.String())
	}
	return fmt.Sprintf("%s(%s)", m.Name, strings.Join(args, ", "))
}

func (o Object) String() string {
	fields := make([]string, 0, len(o.Fields))
	for key, value := range o.Fields {
		fields = append(fields, fmt.Sprintf("\t%s: %s", key, value.String()))
	}
	return fmt.Sprintf("{\n%s\n}", strings.Join(fields, ",\n "))
}

func (a Array) String() string {
	elements := make([]string, 0, len(a.Elements))
	for _, element := range a.Elements {
		elements = append(elements, element.String())
	}
	return fmt.Sprintf("[%s]", strings.Join(elements, ", "))
}

func (s String) String() string {
	return fmt.Sprintf("'%s'", s.Value)
}

func (n Number) String() string {
	return fmt.Sprintf("%d", n.Value)
}

func Walk(n Node, f func(Node)) {
	f(n)

	switch node := n.(type) {
	case Callable:
		for _, arg := range node.Args {
			Walk(arg, f)
		}
	case Object:
		for _, field := range node.Fields {
			Walk(field, f)
		}
	case Array:
		for _, element := range node.Elements {
			Walk(element, f)
		}
	case String, Number:
	}
}

type ImportMap map[string]map[string]any

func MergeImportMap(a, b ImportMap) ImportMap {
	for pkg, names := range b {
		if a[pkg] == nil {
			a[pkg] = map[string]any{}
		}
		for name := range names {
			a[pkg][name] = struct{}{}
		}
	}
	return a
}

type Code struct {
	Content []Declaration
}

func (c Code) GetImportMap() ImportMap {
	imports := make(ImportMap)
	for _, decl := range c.Content {
		imports = MergeImportMap(imports, decl.GetImportMap())
	}
	return imports
}

type Declaration interface {
	GetName() string
	String() string
	GetImportMap() ImportMap
}

type ExportVar struct {
	Name  string
	Value Node
}

// String implements Declaration.
func (v ExportVar) String() string {
	return fmt.Sprintf("export const %s = () => %s", v.Name, v.Value.String())
}

// GetName implements Declaration.
func (v ExportVar) GetName() string {
	return v.Name
}

// GetImportMap implements Declaration.
func (v ExportVar) GetImportMap() ImportMap {
	imports := make(ImportMap)
	Walk(v.Value, func(n Node) {
		switch node := n.(type) {
		case Callable:
			pkg := node.Pkg

			switch pkg {
			case "": // local
				break
			default:
				if imports[node.Pkg] == nil {
					imports[node.Pkg] = map[string]any{}
				}
				imports[node.Pkg][node.Name] = struct{}{}
			}
		}
	})
	return imports
}

var _ Declaration = ExportVar{}

type Import struct {
	Pkg   string
	Names []string
}

func (i Import) String() string {
	names := i.Names
	sort.Strings(names)
	return fmt.Sprintf("import { %s } from '%s'", strings.Join(names, ", "), i.Pkg)
}
