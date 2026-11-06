package ipc

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
)

func clientFrameLimit(op string, response bool) int {
	if isImport(op) || op == "company.brand-set" || response && op == "company.brand-get" {
		return MaxBrandFrame
	}
	return MaxFrame
}

func (c *Client) exchangeFrame(ctx context.Context, q Request) (Response, error) {
	bad := errors.New("invalid IPC response")
	if safeDir(c.dir, c.uid) != nil {
		return Response{}, bad
	}
	socket := filepath.Join(c.dir, "service.sock")
	if _, err := privateFile(socket, c.uid, os.ModeSocket); err != nil {
		return Response{}, bad
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", socket)
	if err != nil {
		return Response{}, bad
	}
	defer conn.Close()
	uc, ok := conn.(*net.UnixConn)
	if !ok {
		return Response{}, bad
	}
	uid, err := peerUID(uc)
	if err != nil || uid != c.uid {
		return Response{}, bad
	}
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	deadline, _ := ctx.Deadline()
	if conn.SetDeadline(deadline) != nil {
		return Response{}, bad
	}
	if writeFrameLimit(conn, q, clientFrameLimit(q.Op, false)) != nil {
		return Response{}, bad
	}
	body, err := readFrameLimit(conn, uint32(clientFrameLimit(q.Op, true)))
	if err != nil {
		return Response{}, bad
	}
	var response Response
	if strict(body, &response) != nil || response.Version != 1 || response.RequestID != q.RequestID {
		return Response{}, bad
	}
	if !validErrorDetail(response) {
		return Response{}, bad
	}
	return response, nil
}
