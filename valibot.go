package protocgenvalibot

import (
	"sort"

	pvr "github.com/bufbuild/protovalidate-go/resolver"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func Generate(file *protogen.File, code *File) error {
	/** AST
	source:
	```typescript
		object({
			name: string(),
			id: any(),
			email: array(required(string())),
		})
	```

	AST:

	```go
	ast := method("object",
		object(
			"name", valibotString(""),
			"id", valibotString(""),
			"email", valibotArray(
				valibotString(""),
				"",
				method("minLength", Number{Value: 3}),
			),
		),
	)
	```
	*/

	messages := file.Messages
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Desc.Name() < messages[j].Desc.Name()
	})

	declarations := make([]Declaration, 0, len(messages))
	for _, m := range messages {
		name := string(m.Desc.Name()) + "Schema"
		ast := astNodeFromMessage(m)
		decl := ExportVar{Name: name, Value: ast, Comment: m.Comments.Leading.String()}

		declarations = append(declarations, decl)
	}

	code.Content = declarations
	return nil
}

func astNodeFromMessage(m *protogen.Message) Node {
	var nameAndValues []any
	fields := m.Fields
	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Desc.Number() < fields[j].Desc.Number()
	})
	for _, f := range fields {
		nameAndValues = append(nameAndValues, string(f.Desc.JSONName()))
		nameAndValues = append(nameAndValues, astNodeFromField(f))
	}
	return vmethod("object", object(nameAndValues...))
}

func astNodeFromField(f *protogen.Field) Node {
	var required bool
	pvresolver := pvr.DefaultResolver{}
	constraints := pvresolver.ResolveFieldConstraints(f.Desc)
	required = constraints.GetRequired()

	if f.Desc.IsList() {
		// https://buf.build/bufbuild/protovalidate/docs/main:buf.validate#buf.validate.FieldConstraints
		// if FieldConstraints.required is true, then it is a non-empty array
		// Further constraints can be added by using RepeatedRules, but it is not supported yet :(
		if required {
			return valibotArray(
				astNodeFromFieldValue(f, false),
				"",
				vmethod("minLength", Number{Value: 1}),
			)
		} else {
			return valibotArray(astNodeFromFieldValue(f, false), "")
		}
	}
	return astNodeFromFieldValue(f, required)
}

func astNodeFromFieldValue(f *protogen.Field, required bool) Node {
	switch f.Desc.Kind() {
	case protoreflect.StringKind:
		if required {
			return valibotString("", vmethod("minLength", Number{1}))
		} else {
			return valibotString("")
		}
	case protoreflect.BoolKind:
		return valibotBoolean("")
	case protoreflect.Int32Kind, protoreflect.Int64Kind, protoreflect.Uint32Kind, protoreflect.Uint64Kind, protoreflect.FloatKind, protoreflect.DoubleKind:
		return Callable{Name: "number", Pkg: "valibot"}
	case protoreflect.MessageKind:
		// Ignore map for now
		if f.Desc.Message().IsMapEntry() {
			return valibotAny()
		}

		// Well-known types
		pkgName := string(f.Desc.Message().FullName().Parent())
		if pkgName == "google.protobuf" {
			return valibotAny()
		}

		return Callable{Name: string(f.Desc.Message().Name()) + "Schema", Pkg: PkgLookup, PkgFile: string(f.Desc.Message().ParentFile().Path())}
	default:
		return valibotAny()
	}
}
