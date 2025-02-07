package protocgenvalibot

// string(ErrorMessage|undefined, Pipe|undefined) | string(Pipe)
func valibotString(message string, pipe ...Callable) Callable {
	args := []Node{}

	if message != "" {
		args = append(args, String{Value: message})
	}

	if len(pipe) > 0 {
		args = append(args, nodesToArray(pipe...))
	}

	return Callable{
		Name: "string",
		Pkg:  "valibot",
		Args: args,
	}
}

func valibotNumber() Callable {
	return Callable{Name: "number", Pkg: "valibot", Args: []Node{}}
}

func valibotAny() Callable {
	return Callable{Name: "any", Pkg: "valibot", Args: []Node{}}
}

func valibotPipe(pipes ...Node) Callable {
	return Callable{Name: "pipe", Pkg: "valibot", Args: pipes}
}

func valibotPartial(item Node) Callable {
	return Callable{Name: "partial", Pkg: "valibot", Args: []Node{item}}
}

func valibotObject(obj Node) Callable {
	return Callable{Name: "object", Pkg: "valibot", Args: []Node{obj}}
}

func valibotIntersect(types ...Node) Callable {
	return Callable{Name: "intersect", Pkg: "valibot", Args: []Node{nodesToArray(types...)}}
}

func valibotUnion(types ...Node) Callable {
	return Callable{Name: "union", Pkg: "valibot", Args: []Node{nodesToArray(types...)}}
}

// array(Item, ErrorMessage|undefined, Pipe|undefined) | array(item, Pipe) | array(item)
func valibotArray(item Node, message string, pipe ...Callable) Callable {
	args := []Node{item}

	if message != "" {
		args = append(args, String{Value: message})
	}

	if len(pipe) > 0 {
		args = append(args, nodesToArray(pipe...))
	}

	return Callable{
		Name: "array",
		Pkg:  "valibot",
		Args: args,
	}
}

func valibotRecord(key Node, value Node) Callable {
	return Callable{Name: "record", Pkg: "valibot", Args: []Node{key, value}}
}

// boolean(ErrorMessage|undefined, Pipe|undefined) | boolean(Pipe)
func valibotBoolean(message string, pipe ...Callable) Callable {
	args := []Node{}

	if message != "" {
		args = append(args, String{Value: message})
	}

	if len(pipe) > 0 {
		args = append(args, nodesToArray(pipe...))
	}

	return Callable{
		Name: "boolean",
		Pkg:  "valibot",
		Args: args,
	}
}

func nodesToArray[T Node](elements ...T) Array {
	arr := Array{
		Elements: []Node{},
	}

	for _, p := range elements {
		arr.Elements = append(arr.Elements, p)
	}

	return arr
}

func object(nameAndValues ...any) Object {
	fields := make([]ObjectMember, 0, len(nameAndValues)/2)
	for i := 0; i < len(nameAndValues); i += 2 {
		name := nameAndValues[i].(string)
		value := nameAndValues[i+1].(Node)
		fields = append(fields, ObjectField{Key: name, Value: value})
	}
	return Object{Fields: fields}
}

func vmethod(name string, args ...Node) Callable {
	return Callable{Name: name, Pkg: "valibot", Args: args}
}
