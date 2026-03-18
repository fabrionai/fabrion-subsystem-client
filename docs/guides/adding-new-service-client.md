# Adding a New Service Client

This guide explains how to add a new code-generated service client to fabrion-subsystem-client.

## Prerequisites

- OpenAPI specification (v3.0+) for the service
- Go 1.24+ installed
- `oapi-codegen` v2.5.1 (pulled automatically via `tools.go`)

## OpenAPI Clients (oapi-codegen)

### 1. Add the OpenAPI Specification

Place the spec in `api/`:

```
api/<service>-api.yaml
```

If using an upstream spec, prefer downloading or extracting from the official source rather than hand-writing. Curate to only the endpoints both repos need.

### 2. Create the Client Package

Create a package directory and a `generate.go` file with the `go:generate` directive:

```
<service>/
├── generate.go       # go:generate directive
└── gen/
    └── client.gen.go # Generated (never edit)
```

The `generate.go` file:

```go
package <service>

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -package <service> -generate client,models,embedded-spec -o gen/client.gen.go ../api/<service>-api.yaml
```

**Flags explained:**
- `-package <service>` — Go package name for generated code
- `-generate client,models,embedded-spec` — Generate client methods, model types, and embedded spec
- `-o gen/client.gen.go` — Output to `gen/` subdirectory (convention from DataFabric ADR-2025-12-09)

### 3. Generate the Client Code

```bash
go generate ./<service>/...
```

Or regenerate all clients:

```bash
make generate
```

### 4. Add Custom Types (if needed)

If the API returns non-standard formats (e.g., Airbyte's flexible timestamps), add a `types.go` in the package root:

```
<service>/
├── generate.go
├── types.go          # Custom types (e.g., FlexibleTime)
└── gen/
    └── client.gen.go
```

### 5. Verify

```bash
go build ./...   # Compiles
go test ./...    # Tests pass
make generate    # Regeneration is idempotent
```

## GraphQL Clients (genqlient)

For GraphQL APIs like Dagster:

### 1. Add Schema and Queries

```
<service>/
├── schema.graphql    # GraphQL schema
├── queries.graphql   # Operations (queries/mutations)
├── genqlient.yaml    # genqlient configuration
├── tools.go          # //go:build tools — imports genqlient
├── generate.go       # go:generate directive
└── gen/
    └── generated.go  # Generated (never edit)
```

### 2. Configure genqlient.yaml

```yaml
schema:
  - schema.graphql
operations:
  - queries.graphql
generated: gen/generated.go
package: gen
context_type: context.Context
use_struct_references: true
bindings:
  Float:
    type: float64
```

### 3. Generate

```bash
go generate ./<service>/...
```

## Hand-Written Clients (no spec)

For APIs without an OpenAPI/GraphQL spec (e.g., Cube.js), add `client.go` and `types.go` directly in the package.

## Consumer Usage

```go
import (
    om "github.com/fabrionai/fabrion-subsystem-client/openmetadata/gen"
    kc "github.com/fabrionai/fabrion-subsystem-client/keycloak/gen"
)

// OpenMetadata
omClient, _ := om.NewClientWithResponses(baseURL, om.WithRequestEditorFn(addAuth))
resp, _ := omClient.CreateOrUpdateDomainWithResponse(ctx, body)

// Keycloak
kcClient, _ := kc.NewClientWithResponses(baseURL, kc.WithRequestEditorFn(addAuth))
roles, _ := kcClient.GetAdminRealmsRealmRolesWithResponse(ctx, realm, nil)
```

## Checklist

- [ ] OpenAPI spec added to `api/`
- [ ] `generate.go` created with `go:generate` directive
- [ ] `go generate` runs successfully
- [ ] `go build ./...` compiles
- [ ] Generated code committed (per ADR — enables offline builds, code review)

## Conventions

These conventions are aligned with DataFabric's ADR-2025-12-09 (OpenAPI Spec-Driven Code Generation):

| Convention | Value |
|---|---|
| Spec location | `api/<service>-api.yaml` |
| Generated output | `<service>/gen/client.gen.go` |
| Generate flags | `client,models,embedded-spec` |
| Tool version | oapi-codegen v2.5.1 |
| GraphQL tool | genqlient v0.8.1 |
| Generated code | Committed to repo |
