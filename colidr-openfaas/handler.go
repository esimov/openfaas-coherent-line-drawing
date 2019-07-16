// MIT License
//
// Copyright (c) 2019 Endre Simo
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package function

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/jpeg"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"gocv.io/x/gocv"
)

// Handle a serverless request
func Handle(req []byte) string {
	var (
		data   []byte
		image  []byte
		params url.Values
	)

	if val, exists := os.LookupEnv("input_mode"); exists && val == "url" {
		inputURL := strings.TrimSpace(string(req))
		u, err := url.Parse(inputURL)
		if err != nil {
			return fmt.Sprintf("Unable to parse url: %s", err)
		}
		link := strings.Split(inputURL, "?")[0]
		params = u.Query()

		resp, err := http.Get(link)
		if err != nil {
			return fmt.Sprintf("Unable to download image file from URI: %s, status %v", inputURL, resp.Status)
		}
		defer resp.Body.Close()

		data, err = ioutil.ReadAll(resp.Body)

		if err != nil {
			return fmt.Sprintf("Unable to read response body: %s", err)
		}
	} else {
		var decodeError error
		data, decodeError = base64.StdEncoding.DecodeString(string(req))
		if decodeError != nil {
			data = req
		}

		contentType := http.DetectContentType(req)
		if contentType != "image/jpeg" && contentType != "image/png" {
			return fmt.Sprintf("Only jpeg or png images, either raw uncompressed bytes or base64 encoded are acceptable inputs, you uploaded: %s", contentType)
		}
	}
	var (
		sr, sm, sc, rho, tau float64
		k, ei, di, bl        int64
		ai                   bool
	)
	if params.Get("sr") != "" {
		sr, _ = strconv.ParseFloat(params.Get("sr"), 64)
	} else {
		sr = 2.6
	}
	if params.Get("sm") != "" {
		sm, _ = strconv.ParseFloat(params.Get("sm"), 64)
	} else {
		sm = 3.0
	}
	if params.Get("sc") != "" {
		sc, _ = strconv.ParseFloat(params.Get("sc"), 64)
	} else {
		sc = 1.0
	}
	if params.Get("rho") != "" {
		rho, _ = strconv.ParseFloat(params.Get("rho"), 64)
	} else {
		rho = 0.98
	}
	if params.Get("tau") != "" {
		tau, _ = strconv.ParseFloat(params.Get("tau"), 32)
	} else {
		tau = 0.98
	}
	if params.Get("k") != "" {
		k, _ = strconv.ParseInt(params.Get("k"), 10, 32)
	} else {
		k = 1
	}
	if params.Get("ei") != "" {
		ei, _ = strconv.ParseInt(params.Get("ei"), 10, 32)
	} else {
		ei = 1
	}
	if params.Get("di") != "" {
		di, _ = strconv.ParseInt(params.Get("di"), 10, 32)
	} else {
		di = 1
	}
	if params.Get("bl") != "" {
		bl, _ = strconv.ParseInt(params.Get("bl"), 10, 32)
	} else {
		bl = 3
	}
	if params.Get("ai") != "" {
		ai, _ = strconv.ParseBool(params.Get("ai"))
	} else {
		ai = false
	}

	opts := options{
		sigmaR:        sr,
		sigmaM:        sm,
		sigmaC:        sc,
		rho:           rho,
		tau:           float32(tau),
		etfKernel:     int(k),
		etfIteration:  int(ei),
		fDogIteration: int(di),
		blurSize:      int(bl),
		antiAlias:     ai,
	}

	tmpfile, err := ioutil.TempFile("/tmp", "image")
	if err != nil {
		log.Fatalf("Unable to create temporary file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	_, err = io.Copy(tmpfile, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Sprintf("Unable to copy the source URI to the destionation file")
	}

	var output string
	query, err := url.ParseQuery(os.Getenv("Http_Query"))
	if err == nil {
		output = query.Get("output")
	}

	if val, exists := os.LookupEnv("output_mode"); exists {
		output = val
	}

	if output == "image" || output == "json_image" {
		cld, err := NewCLD(tmpfile.Name(), opts)
		if err != nil {
			log.Fatalf("cannot initialize CLD: %v", err)
		}

		cldData := cld.GenerateCld()

		rows, cols := cld.image.Rows(), cld.image.Cols()
		mat, err := gocv.NewMatFromBytes(rows, cols, gocv.MatTypeCV8UC1, cldData)
		if err != nil {
			log.Fatalf("error retrieving the byte array: %v", err)
		}

		filename := fmt.Sprintf("/tmp/%d.jpg", time.Now().UnixNano())
		dst, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0755)
		if err != nil {
			return fmt.Sprintf("unable to open the destination file: %v", err)
		}
		defer os.Remove(filename)

		img, err := mat.ToImage()
		if err != nil {
			return fmt.Sprintf("error converting matrix to image: %v", err)
		}

		err = jpeg.Encode(dst, img, &jpeg.Options{Quality: 100})
		if err != nil {
			return fmt.Sprintf("cannot encode the jpeg image: %v", err)
		}

		// Retrieve the resized image.
		image, err = ioutil.ReadFile(filename)
		if err != nil {
			return fmt.Sprintf("unable to read the generated image: %v", err)
		}
	}

	return string(image)
}
