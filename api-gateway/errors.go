package main

import (
	"context"
	"net/http"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// quotaFields are the JSON (and proto) names of the int64 quota inputs.
var quotaFields = []string{
	"storageQuotaBytes", "storage_quota_bytes",
	"requestedQuotaBytes", "requested_quota_bytes",
}

// quotaErrorHandler answers like the default handler, except for a quota that
// cannot even be decoded: a number too large for int64 (or a fraction) never
// reaches users-service, and protojson's "proto: (line 1:22): invalid value for
// int64 field ..." means nothing to a person. They get the same range message
// users-service sends for any other quota out of bounds.
func quotaErrorHandler(rangeMessage string) runtime.ErrorHandlerFunc {
	return func(ctx context.Context, mux *runtime.ServeMux, m runtime.Marshaler, w http.ResponseWriter, r *http.Request, err error) {
		if isQuotaDecodeError(status.Convert(err)) {
			err = status.Error(codes.InvalidArgument, rangeMessage)
		}
		runtime.DefaultHTTPErrorHandler(ctx, mux, m, w, r, err)
	}
}

func isQuotaDecodeError(st *status.Status) bool {
	msg := st.Message()
	if st.Code() != codes.InvalidArgument || !strings.HasPrefix(msg, "proto:") || !strings.Contains(msg, "int64") {
		return false
	}
	for _, field := range quotaFields {
		if strings.Contains(msg, field) {
			return true
		}
	}
	return false
}
