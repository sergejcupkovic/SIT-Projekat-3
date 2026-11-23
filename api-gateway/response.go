package main

import (
	"context"
	"net/http"
	"strconv"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/protobuf/proto"
)

// httpCodeKey is the gRPC response-header metadata key services use to ask
// for a specific HTTP success status (e.g. users-service Register and
// storage-service CreateFolder send "x-http-code: 201").
const httpCodeKey = "x-http-code"

// httpStatusFromMetadata runs for every successful gRPC reply, just before
// the gateway writes the JSON body.
//
// gRPC itself only knows "OK" for success, so by default grpc-gateway
// answers 200. Here we read the "x-http-code" header the service attached
// with grpc.SetHeader, remove it so it does not leak to the client
// (grpc-gateway would otherwise forward it as "Grpc-Metadata-X-Http-Code"),
// and write it as the HTTP status code.
func httpStatusFromMetadata(ctx context.Context, w http.ResponseWriter, _ proto.Message) error {
	md, ok := runtime.ServerMetadataFromContext(ctx)
	if !ok {
		return nil
	}

	vals := md.HeaderMD.Get(httpCodeKey)
	if len(vals) == 0 {
		return nil
	}

	code, err := strconv.Atoi(vals[0])
	if err != nil {
		return err
	}

	delete(md.HeaderMD, httpCodeKey)
	w.Header().Del("Grpc-Metadata-X-Http-Code")
	w.WriteHeader(code)
	return nil
}
