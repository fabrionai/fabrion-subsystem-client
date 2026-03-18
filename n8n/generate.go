package n8n

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -package n8n -generate client,models,embedded-spec -o gen/client.gen.go ../api/n8n-api.yml
