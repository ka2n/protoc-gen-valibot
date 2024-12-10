package protocgenvalibot

import (
	"sort"

	pvr "github.com/bufbuild/protovalidate-go/resolver"
	"github.com/samber/lo"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type GenerateOptions struct {
	SchemaSuffix string
}

type GenContext struct {
	Opt GenerateOptions
}

func Generate(file *protogen.File, code *File, opt GenerateOptions) error {
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

	genCtx := GenContext{Opt: opt}

	messages := file.Messages
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Desc.Name() < messages[j].Desc.Name()
	})

	declarations := make([]Declaration, 0, len(messages))
	for _, m := range messages {
		name := string(m.Desc.Name()) + genCtx.Opt.SchemaSuffix
		ast, err := astNodeFromMessage(genCtx, m)
		if err != nil {
			return err
		}
		decl := ExportVar{Name: name, Value: ast, Comment: m.Comments.Leading.String()}

		declarations = append(declarations, decl)
	}

	code.Content = declarations
	return nil
}

func astNodeFromMessage(genCtx GenContext, m *protogen.Message) (Node, error) {
	fields := m.Fields
	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Desc.Number() < fields[j].Desc.Number()
	})

	normalFields := lo.Filter(fields, func(f *protogen.Field, _ int) bool {
		return f.Oneof == nil
	})

	var nameAndValues []any
	for _, f := range normalFields {
		nameAndValues = append(nameAndValues, string(f.Desc.JSONName()))
		nameAndValues = append(nameAndValues, astNodeFromField(genCtx, f))
	}
	baseObj := valibotObject(object(nameAndValues...))
	if len(m.Oneofs) == 0 {
		return baseObj, nil
	}

	oneOfs := lo.Map(m.Oneofs, func(f *protogen.Oneof, _ int) Node {
		var nameAndValues []any
		for _, ff := range f.Fields {
			nameAndValues = append(nameAndValues, string(ff.Desc.JSONName()))
			nameAndValues = append(nameAndValues, astNodeFromField(genCtx, ff))
		}
		// ...(object({ <fields> }).entries)
		return valibotPartial(valibotObject(object(nameAndValues...)))
	})

	elements := make([]Node, 0, len(oneOfs)+1)
	elements = append(elements, baseObj)
	elements = append(elements, oneOfs...)
	member := lo.Map(elements, func(e Node, _ int) ObjectMember {
		return ObjectSpread{Paren{Member{e, Ident{"entries"}}}}
	})
	return valibotObject(Object{member}), nil
}

func astNodeFromField(genCtx GenContext, f *protogen.Field) Node {
	var required bool
	pvresolver := pvr.DefaultResolver{}
	constraints := pvresolver.ResolveFieldConstraints(f.Desc)
	required = constraints.GetRequired()

	if f.Desc.IsList() {
		// https://buf.build/bufbuild/protovalidate/docs/main:buf.validate#buf.validate.FieldConstraints
		// if FieldConstraints.required is true, then it is a non-empty array
		// Further constraints can be added by using RepeatedRules, but it is not supported yet :(
		if required {
			return valibotPipe(valibotArray(astNodeFromFieldValue(genCtx, f, false), ""), vmethod("minLength", Number{Value: 1}))
		} else {
			return valibotArray(astNodeFromFieldValue(genCtx, f, false), "")
		}
	}

	if f.Desc.IsMap() {
		// Key is string,numeric, or boolean
		var keyType Node
		switch (f.Desc.MapKey()).Kind() {
		case protoreflect.StringKind:
			keyType = valibotString("")
		case protoreflect.BoolKind:
			keyType = valibotBoolean("")
		case protoreflect.Int32Kind, protoreflect.Int64Kind, protoreflect.Uint32Kind, protoreflect.Uint64Kind, protoreflect.FloatKind, protoreflect.DoubleKind:
			keyType = valibotNumber()
		default:
			panic("unsupported map key type")
		}

		return valibotRecord(
			keyType,
			astNodeFromFieldDescriptor(genCtx, f.Desc.MapValue(), false),
		)
	}

	return astNodeFromFieldValue(genCtx, f, required)
}

func astNodeFromFieldValue(genCtx GenContext, f *protogen.Field, required bool) Node {
	return astNodeFromFieldDescriptor(genCtx, f.Desc, required)
}

func astNodeFromFieldDescriptor(genCtx GenContext, f protoreflect.FieldDescriptor, required bool) Node {
	switch f.Kind() {
	// Scalars
	case protoreflect.StringKind:
		if required {
			return valibotPipe(valibotString(""), vmethod("minLength", Number{1}))
		} else {
			return valibotString("")
		}
	case protoreflect.BoolKind:
		return valibotBoolean("")
	case protoreflect.Int32Kind, protoreflect.Int64Kind, protoreflect.Uint32Kind, protoreflect.Uint64Kind, protoreflect.FloatKind, protoreflect.DoubleKind:
		return valibotNumber()

	// Message
	case protoreflect.MessageKind:
		// Ignore map for now
		if f.Message().IsMapEntry() {
			return valibotAny()
		}

		// Well-known types
		pkgName := string(f.Message().FullName().Parent())
		if pkgName == "google.protobuf" {
			return valibotAny()
		}

		return Callable{Name: string(f.Message().Name()) + genCtx.Opt.SchemaSuffix, Pkg: PkgLookup, PkgFile: string(f.Message().ParentFile().Path()), Args: []Node{}}
	default:
		return valibotAny()
	}
}
