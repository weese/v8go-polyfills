/*
 * Copyright (c) 2021 Xingwang Liao
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 */

package internal

import (
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/andybalholm/brotli"
)

/*
Response keeps the *http.Response
*/
type Response struct {
	Header     http.Header
	Status     int32
	StatusText string
	OK         bool
	Redirected bool
	URL        string
	Body       string
}

/*
Handle the *http.Response, return *Response
*/
func HandleHttpResponse(res *http.Response, url string, redirected bool) (*Response, error) {
	defer res.Body.Close()
	var reader io.Reader = res.Body

	// Support gzip, br (brotli), and deflate encodings
	if encHeader := res.Header.Get("Content-Encoding"); encHeader != "" {
		// Multiple encodings are applied in the order listed; we must decode in reverse
		encodings := strings.Split(encHeader, ",")
		// Trim spaces
		for i := range encodings {
			encodings[i] = strings.TrimSpace(strings.ToLower(encodings[i]))
		}

		// Track closers for readers that require closing (e.g., gzip/zlib/flate)
		var closers []io.Closer
		// Decode in reverse order
		for i := len(encodings) - 1; i >= 0; i-- {
			switch enc := encodings[i]; enc {
			case "gzip":
				gr, err := gzip.NewReader(reader)
				if err != nil {
					// If we fail to create a gzip reader, stop and return the error
					return nil, err
				}
				reader = gr
				closers = append(closers, gr)
			case "br":
				// brotli reader does not implement io.Closer
				reader = brotli.NewReader(reader)
			case "deflate":
				// Try zlib-wrapped first (RFC1950), then raw deflate (RFC1951) as fallback
				zr, err := zlib.NewReader(reader)
				if err != nil {
					fr := flate.NewReader(reader)
					reader = fr
					closers = append(closers, fr)
				} else {
					reader = zr
					closers = append(closers, zr)
				}
			case "identity", "":
				// no-op
			default:
				// Unknown encoding; leave as-is
			}
		}
		// Ensure we close any layered readers after we are done reading
		if len(closers) > 0 {
			defer func() {
				for _, c := range closers {
					_ = c.Close()
				}
			}()
		}
	}

	resBody, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	return &Response{
		Header:     res.Header,
		Status:     int32(res.StatusCode), // int type is not support by v8go
		StatusText: res.Status,
		OK:         res.StatusCode >= 200 && res.StatusCode < 300,
		Redirected: redirected,
		URL:        url,
		Body:       string(resBody),
	}, nil
}
