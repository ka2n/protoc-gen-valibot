package protocgenvalibot

import (
	pvr "github.com/bufbuild/protovalidate-go/resolver"
	"github.com/samber/lo"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type GenerateOptions struct {
	SchemaSuffix string
}

type GenContext struct {
	Opt GenerateOptions
}

func Generate(file protoreflect.FileDescriptor, code *File, opt GenerateOptions) error {
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

	messages := file.Messages()
	declarations := make([]Declaration, 0, messages.Len())

	for i := 0; i < messages.Len(); i++ {
		m := messages.Get(i)
		loc := m.ParentFile().SourceLocations().ByDescriptor(m)
		name := string(m.Name()) + genCtx.Opt.SchemaSuffix
		ast, err := astNodeFromMessage(genCtx, m)
		if err != nil {
			return err
		}
		decl := ExportVar{Name: name, Value: ast, Comment: loc.LeadingComments}
		declarations = append(declarations, decl)
	}

	code.Content = declarations
	return nil
}

func astNodeFromMessage(genCtx GenContext, m protoreflect.MessageDescriptor) (Node, error) {
	fieldsIter := m.Fields()

	normalFields := make([]protoreflect.FieldDescriptor, 0, fieldsIter.Len())
	for i := 0; i < fieldsIter.Len(); i++ {
		f := fieldsIter.Get(i)
		if f.ContainingOneof() == nil {
			normalFields = append(normalFields, fieldsIter.Get(i))
		}
	}

	var nameAndValues []any
	for _, f := range normalFields {
		nameAndValues = append(nameAndValues, string(f.JSONName()))
		nameAndValues = append(nameAndValues, astNodeFromField(genCtx, f))
	}
	baseObj := valibotObject(object(nameAndValues...))
	oneOfsIter := m.Oneofs()
	if oneOfsIter.Len() == 0 {
		return baseObj, nil
	}

	oneOfNodes := make([]Node, 0, oneOfsIter.Len())
	//oneOfs := make([]protoreflect.OneofDescriptor, 0, oneOfsIter.Len())
	for i := 0; i < oneOfsIter.Len(); i++ {
		o := oneOfsIter.Get(i)

		var nameAndValues []any
		for i := 0; i < o.Fields().Len(); i++ {
			ff := o.Fields().Get(i)
			nameAndValues = append(nameAndValues, string(ff.JSONName()))
			nameAndValues = append(nameAndValues, astNodeFromField(genCtx, ff))
		}

		oneOfNodes = append(oneOfNodes, valibotPartial(valibotObject(object(nameAndValues...))))
	}

	elements := make([]Node, 0, len(oneOfNodes)+1)
	elements = append(elements, baseObj)
	elements = append(elements, oneOfNodes...)
	member := lo.Map(elements, func(e Node, _ int) ObjectMember {
		return ObjectSpread{Paren{Member{e, Ident{"entries"}}}}
	})
	return valibotObject(Object{member}), nil
}

func astNodeFromField(genCtx GenContext, f protoreflect.FieldDescriptor) Node {
	var required bool
	pvresolver := pvr.DefaultResolver{}
	constraints := pvresolver.ResolveFieldConstraints(f)
	required = constraints.GetRequired()

	if f.IsList() {
		// https://buf.build/bufbuild/protovalidate/docs/main:buf.validate#buf.validate.FieldConstraints
		// if FieldConstraints.required is true, then it is a non-empty array
		// Further constraints can be added by using RepeatedRules, but it is not supported yet :(
		if required {
			return valibotPipe(valibotArray(astNodeFromFieldValue(genCtx, f, false), ""), vmethod("minLength", Number{Value: 1}))
		} else {
			return valibotArray(astNodeFromFieldValue(genCtx, f, false), "")
		}
	}

	if f.IsMap() {
		// Key is string,numeric, or boolean
		var keyType Node
		switch (f.MapKey()).Kind() {
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
			astNodeFromFieldDescriptor(genCtx, f.MapValue(), false),
		)
	}

	return astNodeFromFieldValue(genCtx, f, required)
}

func astNodeFromFieldValue(genCtx GenContext, f protoreflect.FieldDescriptor, required bool) Node {
	return astNodeFromFieldDescriptor(genCtx, f, required)
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
