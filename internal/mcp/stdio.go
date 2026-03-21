package mcp

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
)

func (s *Server) ServeStdio(ctx context.Context, in io.Reader, out io.Writer, errOut io.Writer) error {
	reader := bufio.NewReader(in)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		payload, err := readFramedMessage(reader)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		resp, notify, err := s.HandlePayload(payload, errOut)
		if err != nil {
			return err
		}
		if notify {
			continue
		}
		if err := writeFramedMessage(out, resp); err != nil {
			return err
		}
	}
}

func readFramedMessage(r *bufio.Reader) ([]byte, error) {
	contentLength := 0
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(parts[0]), "Content-Length") {
			contentLength, err = strconv.Atoi(strings.TrimSpace(parts[1]))
			if err != nil {
				return nil, err
			}
		}
	}

	if contentLength <= 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}

	body := make([]byte, contentLength)
	_, err := io.ReadFull(r, body)
	return body, err
}

func writeFramedMessage(w io.Writer, payload []byte) error {
	if _, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n", len(payload)); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}
