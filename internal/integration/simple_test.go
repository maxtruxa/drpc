// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package integration

import (
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/zeebo/assert"

	"storj.io/drpc/drpcctx"
	"storj.io/drpc/drpcerr"
)

func TestSimple(t *testing.T) {
	ctx := drpcctx.NewTracker(context.Background())
	defer ctx.Wait()
	defer ctx.Cancel()

	cli, close := createConnection(standardImpl)
	defer close()

	{
		out, err := cli.Method1(ctx, &In{In: 1})
		assert.NoError(t, err)
		assert.True(t, Equal(out, &Out{Out: 1}))
	}

	{
		stream, err := cli.Method2(ctx)
		assert.NoError(t, err)
		assert.NoError(t, stream.Send(&In{In: 2}))
		assert.NoError(t, stream.Send(&In{In: 2}))
		out, err := stream.CloseAndRecv()
		assert.NoError(t, err)
		assert.True(t, Equal(out, &Out{Out: 2}))
	}

	{
		stream, err := cli.Method3(ctx, &In{In: 3})
		assert.NoError(t, err)
		for {
			out, err := stream.Recv()
			if err == io.EOF {
				break
			}
			assert.NoError(t, err)
			assert.True(t, Equal(out, &Out{Out: 3}))
		}
	}

	{
		stream, err := cli.Method4(ctx)
		assert.NoError(t, err)
		assert.NoError(t, stream.Send(&In{In: 4}))
		assert.NoError(t, stream.Send(&In{In: 4}))
		assert.NoError(t, stream.Send(&In{In: 4}))
		assert.NoError(t, stream.Send(&In{In: 4}))
		assert.NoError(t, stream.CloseSend())
		for {
			out, err := stream.Recv()
			if err == io.EOF {
				break
			}
			assert.NoError(t, err)
			assert.True(t, Equal(out, &Out{Out: 4}))
		}
	}

	{
		_, err := cli.Method1(ctx, &In{In: 5})
		assert.Error(t, err)
		assert.Equal(t, drpcerr.Code(err), 5)
	}
}

func TestConcurrent(t *testing.T) {
	ctx := drpcctx.NewTracker(context.Background())
	defer ctx.Wait()
	defer ctx.Cancel()

	cli, close := createConnection(standardImpl)
	defer close()

	const N = 1000
	errs := make(chan error)
	for i := 0; i < N; i++ {
		ctx.Run(func(ctx context.Context) {
			out, err := cli.Method1(ctx, &In{In: 1})
			if err != nil {
				errs <- err
			} else if out.Out != 1 {
				errs <- fmt.Errorf("wrong result %d", out.Out)
			} else {
				errs <- nil
			}
		})
	}
	for i := 0; i < N; i++ {
		assert.NoError(t, <-errs)
	}
}
