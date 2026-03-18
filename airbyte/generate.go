package airbyte

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -package airbyte -generate client,models,embedded-spec -o gen/client.gen.go ../api/airbyte-api.yml
