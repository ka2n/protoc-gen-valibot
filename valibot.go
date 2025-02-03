package protocgenvalibot

import (
	pvr "github.com/bufbuild/protovalidate-go/resolver"
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

	intersections := make([]Node, 0)

	normalFields := make([]protoreflect.FieldDescriptor, 0, fieldsIter.Len())
	normalFieldsOpt := make([]protoreflect.FieldDescriptor, 0, fieldsIter.Len())
	for i := 0; i < fieldsIter.Len(); i++ {
		f := fieldsIter.Get(i)
		if f.ContainingOneof() == nil {
			if isFieldRequired(f) {
				normalFields = append(normalFields, f)
			} else {
				normalFieldsOpt = append(normalFieldsOpt, f)
			}
		}
	}
	baseObject := valibotObject(objectFromFields(genCtx, normalFields))
	intersections = append(intersections, baseObject)

	baseObjectOpt := valibotPartial(valibotObject(objectFromFields(genCtx, normalFieldsOpt)))
	intersections = append(intersections, baseObjectOpt)

	// Process oneofs
	oneOfsIter := m.Oneofs()
	for i := 0; i < oneOfsIter.Len(); i++ {
		oneOfNodes := []Node{}
		o := oneOfsIter.Get(i)
		required := isOneofRequired(o)

		if required {
			for i := 0; i < o.Fields().Len(); i++ {
				ff := o.Fields().Get(i)
				oneOfNodes = append(
					oneOfNodes,
					valibotObject(object(string(ff.JSONName()), astNodeFromField(genCtx, ff))),
				)
			}
		} else {
			nameAndValues := []any{}
			for i := 0; i < o.Fields().Len(); i++ {
				ff := o.Fields().Get(i)
				nameAndValues = append(nameAndValues, string(ff.JSONName()))
				nameAndValues = append(nameAndValues, astNodeFromField(genCtx, ff))
			}
			oneOfNodes = append(oneOfNodes, valibotPartial(valibotObject(object(nameAndValues...))))
		}

		if len(oneOfNodes) > 1 {
			oneOfUnion := valibotUnion(oneOfNodes...)
			intersections = append(intersections, oneOfUnion)
		} else if len(oneOfNodes) == 1 {
			intersections = append(intersections, oneOfNodes[0])
		}
	}

	return valibotIntersect(intersections...), nil
}

func objectFromFields(genCtx GenContext, fields []protoreflect.FieldDescriptor) Object {
	nameAndValues := make([]any, 0, len(fields)*2)
	for _, f := range fields {
		nameAndValues = append(nameAndValues, string(f.JSONName()))
		nameAndValues = append(nameAndValues, astNodeFromField(genCtx, f))
	}
	return object(nameAndValues...)
}

func astNodeFromField(genCtx GenContext, f protoreflect.FieldDescriptor) Node {
	required := isFieldRequired(f)

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

func isFieldRequired(f protoreflect.FieldDescriptor) bool {
	var required bool
	pr := pvr.DefaultResolver{}
	constraints := pr.ResolveFieldConstraints(f)
	required = constraints.GetRequired()
	return required
}

func isOneofRequired(f protoreflect.OneofDescriptor) bool {
	var required bool
	pr := pvr.DefaultResolver{}
	constraints := pr.ResolveOneofConstraints(f)
	required = constraints.GetRequired()
	return required
}
