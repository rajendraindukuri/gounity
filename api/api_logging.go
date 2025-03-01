/*
Copyright (c) 2019 Dell EMC Corporation
All Rights Reserved
*/

package api

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"

	log "github.com/sirupsen/logrus"
)

func isBinOctetBody(h http.Header) bool {
	return h.Get(HeaderKeyContentType) == headerValContentTypeBinaryOctetStream
}

func logRequest(
	ctx context.Context,
	req *http.Request,
	lf func(func(args ...interface{}), string)) {

	w := &bytes.Buffer{}

	fmt.Fprintln(w)
	fmt.Fprint(w, "    -------------------------- ")
	fmt.Fprint(w, "GOUNITY HTTP REQUEST")
	fmt.Fprintln(w, " -------------------------")

	buf, err := httputil.DumpRequest(req, !isBinOctetBody(req.Header))
	if err != nil {
		return
	}

	err2 := WriteIndented(w, buf)
	if err2 != nil {
		message := fmt.Sprintf("Indentation failed with error: %v", err2)
		log.Info(message)
	}
	fmt.Fprintln(w)

	//Will not be logging request to avoid logging of headers as it is
	//lf(log.Debug, w.String())
}

func logResponse(
	ctx context.Context,
	res *http.Response,
	lf func(func(args ...interface{}), string)) {

	w := &bytes.Buffer{}

	fmt.Fprintln(w)
	fmt.Fprint(w, "    -------------------------- ")
	fmt.Fprint(w, "GOUNITY HTTP RESPONSE")
	fmt.Fprintln(w, " -------------------------")

	buf, err := httputil.DumpResponse(res, !isBinOctetBody(res.Header))
	if err != nil {
		return
	}

	bw := &bytes.Buffer{}
	err2 := WriteIndented(bw, buf)
	if err2 != nil {
		message := fmt.Sprintf("Indentation failed with error: %v", err2)
		log.Info(message)
	}

	scanner := bufio.NewScanner(bw)
	for {
		if !scanner.Scan() {
			break
		}
		fmt.Fprintln(w, scanner.Text())
	}

	log.Debug(w.String())
}

// WriteIndentedN indents all lines n spaces.
func WriteIndentedN(w io.Writer, b []byte, n int) error {
	s := bufio.NewScanner(bytes.NewReader(b))
	if !s.Scan() {
		return nil
	}
	l := s.Text()
	for {
		for x := 0; x < n; x++ {
			if _, err := fmt.Fprint(w, " "); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprint(w, l); err != nil {
			return err
		}
		if !s.Scan() {
			break
		}
		l = s.Text()
		if _, err := fmt.Fprint(w, "\n"); err != nil {
			return err
		}
	}
	return nil
}

// WriteIndented indents all lines four spaces.
func WriteIndented(w io.Writer, b []byte) error {
	return WriteIndentedN(w, b, 4)
}
