package protocgenvalibot

import (
	"fmt"
	"sort"
	"strings"

	"github.com/samber/lo"
)

type Node interface {
	String() string
}

type Ident struct {
	Name string
}

var _ Node = Ident{}

type Callable struct {
	Name    string
	Pkg     string
	PkgFile string
	Args    []Node
}

var _ Node = Callable{}

const PkgLookup = ":lookup:"

type Object struct {
	Fields []ObjectMember
}

var _ Node = Object{}

type ObjectMember interface {
	ObjectKey() string
	ObjectValue() Node
}

type ObjectField struct {
	Key   string
	Value Node
}

var _ ObjectMember = ObjectField{}

type ObjectSpread struct {
	Node Node
}

var _ ObjectMember = ObjectSpread{}

type Array struct {
	Elements []Node
}

var _ Node = Array{}

type String struct {
	Value string
}

var _ Node = String{}

type Number struct {
	Value int
}

var _ Node = Number{}

type Paren struct {
	Node Node
}

var _ Node = Paren{}

type Member struct {
	Node  Node
	Field Node
}

var _ Node = Member{}

func (i Ident) String() string {
	return i.Name
}

func (p Paren) String() string {
	return fmt.Sprintf("(%s)", p.Node.String())
}

func (m Callable) String() string {
	if (m.Args) == nil {
		return fmt.Sprintf("%s", m.Name)
	}

	args := make([]string, 0, len(m.Args))
	for _, arg := range m.Args {
		args = append(args, arg.String())
	}
	return fmt.Sprintf("%s(%s)", m.Name, strings.Join(args, ", "))
}

func (o Object) String() string {
	fields := make([]string, 0, len(o.Fields))
	for _, f := range o.Fields {
		key := f.ObjectKey()
		value := f.ObjectValue()
		sep := lo.Ternary(key != "...", ": ", "")
		fields = append(fields, fmt.Sprintf("\t%s%s%s", key, sep, value.String()))
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

func (m Member) String() string {
	return m.Node.String() + "." + m.Field.String()
}

// ObjectKey implements ObjectMember.
func (o ObjectField) ObjectKey() string {
	return o.Key
}

// ObjectValue implements ObjectMember.
func (o ObjectField) ObjectValue() Node {
	return o.Value
}

// ObjectKey implements ObjectMember.
func (o ObjectSpread) ObjectKey() string {
	return "..."
}

// ObjectValue implements ObjectMember.
func (o ObjectSpread) ObjectValue() Node {
	return o.Node
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
			Walk(field.ObjectValue(), f)
		}
	case Array:
		for _, element := range node.Elements {
			Walk(element, f)
		}
	case Paren:
		Walk(node.Node, f)
	case Member:
		Walk(node.Node, f)
	case String, Number, Ident:
	}
}

type ImportMap map[string]map[string]ImportMapDetail

type ImportMapDetail struct {
	FullName string
}

func MergeImportMap(a, b ImportMap) ImportMap {
	for pkg, names := range b {
		if a[pkg] == nil {
			a[pkg] = map[string]ImportMapDetail{}
		}
		for name, v := range names {
			a[pkg][name] = v
		}
	}
	return a
}

type File struct {
	Content []Declaration
}

func (c File) GetImportMap() ImportMap {
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
	Name    string
	Value   Node
	Comment string
}

// String implements Declaration.
func (v ExportVar) String() string {
	comment := v.Comment
	if strings.HasPrefix(comment, "// ") {
		comment = strings.TrimPrefix(comment, "//")
		comment = strings.TrimSpace(comment)
		comment = "/** " + comment + " */\n"
	}
	return fmt.Sprintf("%sexport const %s = () => %s", comment, v.Name, v.Value.String())
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
					imports[node.Pkg] = map[string]ImportMapDetail{}
				}
				imports[node.Pkg][node.Name] = ImportMapDetail{FullName: node.PkgFile}
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
