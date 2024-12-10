# protoc-gen-valibot

protoc-gen-valibot is a plugin for Protocol Buffers that generates [valibot](https://valibot.dev/) schemas from Protobuf message definitions. The generated schemas are designed to validate protoJSON representations of your Protobuf messages.

🚧 This project is in an early alpha stage. Expect breaking changes and incomplete functionality. Contributions and feedback are welcome!

## Usage

1. Install `protoc-gen-valibot` binary from [Releases](https://github.com/ka2n/protoc-gen-valibot/releases)
2. Configure buf.gen.yaml for schema generation. Below is an example configuration:
```yaml:buf.gen.yaml
version: v2
managed:
  enabled: true
  override:
    - field_option: jstype
      value: JS_STRING
plugins:
  - remote: buf.build/bufbuild/es:v2.2.0 # renovate: depName=bufbuild/protobuf-es
    out: packages/schema/src
    opt:
      - target=ts
      - json_types=true
  - local: protoc-gen-valibot
    out: ./packages/schema/src
    opt:
      - schema_suffix=Valibot
inputs:
  - directory: proto
```

This configuration uses buf to generate TypeScript definitions alongside valibot schemas, ensuring compatibility with protoJSON (`buildbuild/protobuf-es`'s `fromJson`/`toJson`).
