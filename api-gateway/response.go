package main

import (
	"context"
	"net/http"
	"strconv"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/protobuf/proto"
)

const httpCodeKey = "x-http-code"

// Lets a service answer 201 by sending the "x-http-code" gRPC header.
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
