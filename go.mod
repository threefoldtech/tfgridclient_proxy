module github.com/waleedhammam/graphql-server

go 1.13

require (
	github.com/go-redis/redis/v8 v8.11.3
	github.com/gorilla/mux v1.8.0
	github.com/pkg/errors v0.9.1
	github.com/rs/zerolog v1.23.0
	github.com/threefoldtech/zos v0.4.10-0.20210814102443-3cf4d7b75604
)

replace github.com/waleedhammam/graphql-server v1.0.0 => ./
