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
	"image"
	"math"

	"gocv.io/x/gocv"
)

// PostProcessing is a basic struct used for post processing operations
type PostProcessing struct {
	Etf
	blurSize int
}

// NewPostProcessing is a constructor method which initialize a PostProcessing struct.
func NewPostProcessing(blurSize int) *PostProcessing {
	return &PostProcessing{
		blurSize: blurSize,
	}
}

// VizEtf visualize the edge tangent flow flowfield.
func (pp *PostProcessing) VizEtf(flowField, dst *gocv.Mat) {
	var (
		it    = 10.0
		sigma = 2.0 * it * it
	)

	noise := gocv.NewMatWithSize(flowField.Rows()/2, flowField.Cols()/2, gocv.MatTypeCV32F+gocv.MatChannels3)
	gocv.Randu(&noise, 0.0, 1.0)
	gocv.Resize(noise, &noise, image.Point{flowField.Cols(), flowField.Rows()}, 0, 0, gocv.InterpolationNearestNeighbor)

	rows := noise.Rows()
	cols := noise.Cols()

	pp.wg.Add(rows * cols)

	for i := 0; i < rows; i++ {
		for j := 0; j < cols; j++ {
			go func(i, j int) {
				defer pp.wg.Done()

				wSum := 0.0
				x := float32(i)
				y := float32(j)

				for k := 0; k < int(it); k++ {
					v := flowField.GetVecfAt((int(x)+rows)%rows, (int(y)+cols)%cols)
					if v[0] != 0 {
						x = x + (abs(v[0])/float32(abs(v[0])+abs(v[1])))*(abs(v[0])/v[0])
					}
					if v[1] != 0 {
						y = y + (abs(v[1])/float32(abs(v[0])+abs(v[1])))*(abs(v[1])/v[1])
					}
					r2 := float32(k * k)
					w := (1.0 / (math.Pi * sigma)) * math.Exp(-(float64(r2))/sigma)

					xx := (int(x) + rows) % rows
					yy := (int(y) + cols) % cols

					dstAt := dst.GetFloatAt(i, j)
					noiseAt := noise.GetFloatAt(xx, yy)
					newVal := dstAt + (float32(w) * noiseAt)
					wSum += w

					dst.SetFloatAt(i, j, float32(newVal))
				}

				x = float32(i)
				y = float32(j)
				for k := 0; k < int(it); k++ {
					v := flowField.GetVecfAt((int(x)+rows)%rows, (int(y)+cols)%cols)
					if -v[0] != 0 {
						x = x + (abs(-v[0])/float32(abs(-v[0])+abs(-v[1])))*(abs(-v[0])/-v[0])
					}
					if -v[1] != 0 {
						y = y + (abs(-v[1])/float32(abs(-v[0])+abs(-v[1])))*(abs(-v[1])/-v[1])
					}
					r2 := float32(k * k)
					w := (1.0 / (math.Pi * sigma)) * math.Exp(-(float64(r2))/sigma)

					xx := (int(x) + rows) % rows
					yy := (int(y) + cols) % cols

					dstAt := dst.GetFloatAt(i, j)
					noiseAt := noise.GetFloatAt(xx, yy)
					newVal := dstAt + (float32(w) * noiseAt)
					wSum += w

					dst.SetFloatAt(i, j, float32(newVal))
				}

				dstAt := dst.GetFloatAt(i, j)
				dstAt /= float32(wSum)

				dst.SetFloatAt(i, j, dstAt)
			}(i, j)
		}
	}

	pp.wg.Wait()
}

// AntiAlias smooths out the destination matrix.
func (pp *PostProcessing) AntiAlias(src, dst gocv.Mat) {
	gocv.Normalize(src, &dst, 0.0, 255.0, gocv.NormMinMax)
	gocv.GaussianBlur(dst, &dst, image.Point{pp.blurSize, pp.blurSize}, 0.0, 0.0, gocv.BorderConstant)
}

func abs(val float32) float32 {
	if val < 0.0 {
		return -val
	}
	return val
}
