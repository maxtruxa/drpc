// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package grpccompat

import (
	"context"
	"io"
	"testing"

	"github.com/zeebo/assert"
	"google.golang.org/protobuf/proto"
)

func asOut(in *In) *Out {
	return &Out{Out: in.In, Buf: in.Buf}
}

var benchmarkImpl = &serviceImpl{
	Method1Fn: func(ctx context.Context, in *In) (*Out, error) {
		return asOut(in), nil
	},

	Method2Fn: func(stream ServerMethod2Stream) error {
		var in *In
		for {
			cin, err := stream.Recv()
			if err == io.EOF {
				break
			} else if err != nil {
				return err
			}
			in = cin
		}
		return stream.SendAndClose(asOut(in))
	},

	Method3Fn: func(in *In, stream ServerMethod3Stream) error {
		for i := int64(0); i < in.In; i++ {
			err := stream.Send(asOut(in))
			if err != nil {
				return err
			}
		}
		return nil
	},

	Method4Fn: func(stream ServerMethod4Stream) error {
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				return nil
			} else if err != nil {
				return err
			}
			err = stream.Send(asOut(in))
			if err != nil {
				return err
			}
		}
	},
}

func benchmarkBoth(b *testing.B, fn func(b *testing.B, in *In, client Client)) {
	for _, size := range []struct {
		Name  string
		Value *In
	}{
		{"Small", &In{In: 5}},
		{"Med", &In{In: 1, Buf: make([]byte, 2<<10)}},
		{"Large", &In{In: 1, Buf: make([]byte, 1<<20)}},
	} {
		size := size

		b.Run(size.Name, func(b *testing.B) {
			b.Run("GRPC", func(b *testing.B) {
				conn, close := createGRPCConnection(benchmarkImpl.GRPC())
				defer close()
				fn(b, size.Value, grpcWrapper{conn})
			})
			b.Run("DRPC", func(b *testing.B) {
				conn, close := createDRPCConnection(benchmarkImpl.DRPC())
				defer close()
				fn(b, size.Value, drpcWrapper{conn})
			})
		})
	}
}

func BenchmarkUnitary(b *testing.B) {
	benchmarkBoth(b, func(b *testing.B, in *In, client Client) {
		ctx := context.Background()

		b.SetBytes(int64(proto.Size(in)))
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, err := client.Method1(ctx, in)
			assert.NoError(b, err)
		}
	})
}

func BenchmarkInputStream(b *testing.B) {
	benchmarkBoth(b, func(b *testing.B, in *In, client Client) {
		ctx := context.Background()

		stream, err := client.Method2(ctx)
		assert.NoError(b, err)

		b.SetBytes(int64(proto.Size(in)))
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			err = stream.Send(in)
			assert.NoError(b, err)
		}

		_, err = stream.CloseAndRecv()
		assert.NoError(b, err)
	})
}

func BenchmarkOutputStream(b *testing.B) {
	benchmarkBoth(b, func(b *testing.B, in *In, client Client) {
		ctx := context.Background()

		in.In = int64(b.N)
		stream, err := client.Method3(ctx, in)
		assert.NoError(b, err)

		b.SetBytes(int64(proto.Size(in)))
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, err = stream.Recv()
			assert.NoError(b, err)
		}
	})
}

func BenchmarkBidirectionalStream(b *testing.B) {
	benchmarkBoth(b, func(b *testing.B, in *In, client Client) {
		ctx := context.Background()

		stream, err := client.Method4(ctx)
		assert.NoError(b, err)

		b.SetBytes(int64(proto.Size(in)))
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			err = stream.Send(in)
			assert.NoError(b, err)

			_, err = stream.Recv()
			assert.NoError(b, err)
		}
	})
}
