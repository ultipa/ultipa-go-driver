package gqldb

import (
	"context"
	"runtime/debug"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// ClientHeader is the request header the driver sends on every call: its
// language and version, such as "go/v6.2.144". It tells the server that this
// driver decides by the error detail (code, reason, executed, partly_stored)
// and never by words in a message, so the server sends every message as it
// was built. Without it the server takes the client for a driver older than
// this line and puts an invisible U+2060 WORD JOINER into the phrases such
// drivers sign in again or resend on ("session expired", "leader_changed",
// "use streaming API", ...).
const ClientHeader = "x-gqldb-client"

const modulePath = "github.com/ultipa/ultipa-go-driver/v6"

// driverVersion is the version of this module as the program was built with
// it ("v6.2.144"), "devel" inside its own repository, "unknown" without build
// information.
func driverVersion() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	version := ""
	if bi.Main.Path == modulePath {
		version = bi.Main.Version
	}
	for _, d := range bi.Deps {
		if d.Path == modulePath {
			version = d.Version
		}
	}
	version = strings.Trim(version, "()")
	if version == "" {
		return "unknown"
	}
	return version
}

var clientHeaderValue = "go/" + driverVersion()

// ClientHeaderValue is the value of ClientHeader this driver sends.
func ClientHeaderValue() string { return clientHeaderValue }

// withClientHeader adds ClientHeader to an outgoing call's metadata.
func withClientHeader(ctx context.Context) context.Context {
	return metadata.AppendToOutgoingContext(ctx, ClientHeader, clientHeaderValue)
}

// clientHeaderDialOptions are the interceptors that put ClientHeader on every
// unary and streaming call of a connection.
func clientHeaderDialOptions() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithChainUnaryInterceptor(func(ctx context.Context, method string, req, reply interface{},
			cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
			return invoker(withClientHeader(ctx), method, req, reply, cc, opts...)
		}),
		grpc.WithChainStreamInterceptor(func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn,
			method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
			return streamer(withClientHeader(ctx), desc, cc, method, opts...)
		}),
	}
}
