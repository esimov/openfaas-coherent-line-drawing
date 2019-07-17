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
	"fmt"
	"image"
	"math"
	"os"
	"sync"

	"gocv.io/x/gocv"
)

// Cld is the main entry struct for the Coherent Line Drawing operations.
type Cld struct {
	image  gocv.Mat
	result gocv.Mat
	dog    gocv.Mat
	fDog   gocv.Mat
	etf    *Etf
	wg     sync.WaitGroup
	options
}

// Options struct contains all the options currently supported by Cld,
// exposed by the main CLI application.
type options struct {
	sigmaR        float64
	sigmaM        float64
	sigmaC        float64
	rho           float64
	tau           float32
	blurSize      int
	etfKernel     int
	etfIteration  int
	fDogIteration int
	antiAlias     bool
	visEtf        bool
	visResult     bool
}

// position is a basic struct for vector type operations
type position struct {
	x, y float64
}

// NewCLD is the constructor method which require the source image and the CLD options as parameters.
func NewCLD(imgFile string, cldOpts options) (*Cld, error) {
	f, err := os.Stat(imgFile)
	if os.IsNotExist(err) {
		return nil, err
	}
	if f.IsDir() {
		return nil, fmt.Errorf("missing file name")
	}

	srcImage := gocv.IMRead(imgFile, gocv.IMReadGrayScale)
	rows, cols := srcImage.Rows(), srcImage.Cols()

	result := gocv.NewMatWithSize(rows, cols, gocv.MatTypeCV8UC1)
	dog := gocv.NewMatWithSize(rows, cols, gocv.MatTypeCV32F)
	fDog := gocv.NewMatWithSize(rows, cols, gocv.MatTypeCV32F)

	var wg sync.WaitGroup

	etf := NewETF()
	etf.Init(cols, rows)

	err = etf.InitDefaultEtf(imgFile, image.Point{X: cols, Y: rows})
	if err != nil {
		return nil, fmt.Errorf("unable to initialize edge tangent flow: %s", err)
	}

	if cldOpts.etfIteration > 0 {
		for i := 0; i < cldOpts.etfIteration; i++ {
			etf.RefineEtf(cldOpts.etfKernel)
		}
	}

	return &Cld{
		srcImage, result, dog, fDog, etf, wg, cldOpts,
	}, nil
}

// GenerateCld is the entry method for generating the coherent line drawing output.
// It triggers the generate method in iterative manner and returns the resulting byte array.
func (c *Cld) GenerateCld() []byte {
	c.generate()

	if c.fDogIteration > 0 {
		for i := 0; i < c.fDogIteration; i++ {
			c.combineImage()
			c.generate()
		}
	}

	pp := NewPostProcessing(c.blurSize)
	if c.antiAlias {
		pp.AntiAlias(c.result, c.result)
	}

	return c.result.ToBytes()
}

// generate is a helper method which enclose all the requested operation for the CLD computation.
func (c *Cld) generate() {
	srcImg32FC1 := gocv.NewMatWithSize(c.image.Rows(), c.image.Cols(), gocv.MatTypeCV32F)
	c.image.ConvertTo(&srcImg32FC1, gocv.MatTypeCV32F, 1.0/255.0)

	c.gradientDoG(&srcImg32FC1, &c.dog, c.rho, c.sigmaC)
	c.flowDoG(&c.dog, &c.fDog, c.sigmaM)
	c.binaryThreshold(&c.fDog, &c.result, c.tau)
}

// gradientDoG computes the gradient difference-of-Gaussians (DoG)
func (c *Cld) gradientDoG(src, dst *gocv.Mat, rho, sigmaC float64) {
	var sigmaS = c.sigmaR * sigmaC
	gvc := makeGaussianVector(sigmaC)
	gvs := makeGaussianVector(sigmaS)
	kernel := len(gvs) - 1

	width, height := dst.Cols(), dst.Rows()
	c.wg.Add(width * height)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			go func(y, x int) {
				var (
					gauCAcc, gauSAcc             float64
					gauCWeightAcc, gauSWeightAcc float64
				)

				c.etf.mu.Lock()
				defer c.etf.mu.Unlock()

				tmp := c.etf.flowField.GetVecfAt(y, x)
				gradient := position{x: float64(-tmp[0]), y: float64(tmp[1])}

				for step := -kernel; step <= kernel; step++ {
					row := float64(y) + gradient.y*float64(step)
					col := float64(x) + gradient.x*float64(step)

					if row > float64(dst.Rows()-1) || row < 0.0 || col > float64(dst.Cols()-1) || col < 0.0 {
						continue
					}
					val := src.GetFloatAt(int(round(row)), int(round(col)))

					gauIdx := absInt(step)
					gauCWeight := func(gauIdx int) float64 {
						if gauIdx >= len(gvc) {
							return 0.0
						}
						return gvc[gauIdx]
					}(gauIdx)

					gauSWeight := gvs[gauIdx]
					gauCAcc += float64(val) * gauCWeight
					gauSAcc += float64(val) * gauSWeight
					gauCWeightAcc += gauCWeight
					gauSWeightAcc += gauSWeight
				}

				vc := gauCAcc / gauCWeightAcc
				vs := gauSAcc / gauSWeightAcc

				res := vc - rho*vs
				dst.SetFloatAt(y, x, float32(res))

				c.wg.Done()
			}(y, x)
		}
	}
	c.wg.Wait()
}

// flowDoG computes the flow difference-of-Gaussians (DoG)
func (c *Cld) flowDoG(src, dst *gocv.Mat, sigmaM float64) {
	var (
		gauAcc       float64
		gauWeightAcc float64
	)

	gausVec := makeGaussianVector(sigmaM)
	width, height := src.Cols(), src.Rows()
	kernelHalf := len(gausVec) - 1

	c.wg.Add(width * height)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			go func(y, x int) {
				c.etf.mu.Lock()
				defer c.etf.mu.Unlock()

				gauAcc = -gausVec[0] * float64(src.GetFloatAt(y, x))
				gauWeightAcc = -gausVec[0]

				// Integral alone ETF
				pos := &position{x: float64(x), y: float64(y)}
				for step := 0; step < kernelHalf; step++ {
					tmp := c.etf.flowField.GetVecfAt(int(pos.y), int(pos.x))
					direction := &position{x: float64(tmp[1]), y: float64(tmp[0])}

					if direction.x == 0 && direction.y == 0 {
						break
					}

					if pos.x > float64(width-1) || pos.x < 0.0 ||
						pos.y > float64(height-1) || pos.y < 0.0 {
						break
					}

					value := src.GetFloatAt(int(pos.y), int(pos.x))
					weight := gausVec[step]

					gauAcc += float64(value) * weight
					gauWeightAcc += weight

					// move along ETF direction
					pos.x += direction.x
					pos.y += direction.y

					if int(round(pos.x)) < 0 || int(round(pos.x)) > width-1 ||
						int(round(pos.y)) < 0 || int(round(pos.y)) > height-1 {
						break
					}
				}

				// Integral alone inverse ETF
				pos = &position{x: float64(x), y: float64(y)}
				for step := 0; step < kernelHalf; step++ {
					tmp := c.etf.flowField.GetVecfAt(int(pos.y), int(pos.x))
					direction := &position{x: float64(-tmp[1]), y: float64(-tmp[0])}

					if direction.x == 0 && direction.y == 0 {
						break
					}

					if pos.x > float64(width-1) || pos.x < 0.0 ||
						pos.y > float64(height-1) || pos.y < 0.0 {
						break
					}

					value := src.GetFloatAt(int(pos.y), int(pos.x))
					weight := gausVec[step]

					gauAcc += float64(value) * weight
					gauWeightAcc += weight

					// move along ETF direction
					pos.x += direction.x
					pos.y += direction.y

					if int(round(pos.x)) < 0 || int(round(pos.x)) > width-1 ||
						int(round(pos.y)) < 0 || int(round(pos.y)) > height-1 {
						break
					}
				}

				newVal := func(gauAcc, gauWeightAcc float64) float64 {
					var res float64

					if gauAcc/gauWeightAcc > 0 {
						res = 1.0
					} else {
						res = 1.0 + math.Tanh(gauAcc/gauWeightAcc)
					}
					return res
				}

				// Update pixel value in the destination matrix.
				dst.SetFloatAt(y, x, float32(newVal(gauAcc, gauWeightAcc)))

				c.wg.Done()
			}(y, x)
		}
	}
	gocv.Normalize(*dst, dst, 0.0, 1.0, gocv.NormMinMax)

	c.wg.Wait()
}

// binaryThreshold threshold an image as black and white.
func (c *Cld) binaryThreshold(src, dst *gocv.Mat, tau float32) []byte {
	width, height := dst.Cols(), dst.Rows()
	c.wg.Add(width * height)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			go func(y, x int) {
				c.etf.mu.Lock()
				defer c.etf.mu.Unlock()

				h := src.GetFloatAt(y, x)
				v := func(h float32) uint8 {
					if h < tau {
						return 0
					}
					return 255
				}(h)
				dst.SetUCharAt(y, x, v)

				c.wg.Done()
			}(y, x)
		}
	}
	c.wg.Wait()

	return dst.ToBytes()
}

func (c *Cld) combineImage() {
	for y := 0; y < c.image.Rows(); y++ {
		for x := 0; x < c.image.Cols(); x++ {
			c.wg.Add(1)
			go func(y, x int) {
				c.etf.mu.Lock()
				defer c.etf.mu.Unlock()

				h := c.result.GetUCharAt(y, x)
				if h == 0 {
					c.image.SetUCharAt(y, x, 0)
				}
				c.wg.Done()
			}(y, x)
		}
	}

	// Apply a gaussian blur to let it more smooth
	gocv.GaussianBlur(c.image, &c.image, image.Point{c.blurSize, c.blurSize}, 0.0, 0.0, gocv.BorderConstant)
	c.wg.Wait()
}

// gauss computes gaussian function of variance
func gauss(x, mean, sigma float64) float64 {
	return math.Exp((-(x-mean)*(x-mean))/(2*sigma*sigma)) / math.Sqrt(math.Pi*2.0*sigma*sigma)
}

// makeGaussianVector constructs a gaussian vector field of floats
func makeGaussianVector(sigma float64) []float64 {
	var (
		gau       []float64
		threshold = 0.001
		i         int
	)

	for {
		i++
		if gauss(float64(i), 0.0, sigma) < threshold {
			break
		}
	}
	// clear slice
	gau = gau[:0]
	// extend slice
	gau = append(gau, make([]float64, i+1)...)
	gau[0] = gauss(0.0, 0.0, sigma)

	for j := 1; j < len(gau); j++ {
		gau[j] = gauss(float64(j), 0.0, sigma)
	}

	return gau
}

// absInt return the absolute value of x
func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// round returns the nearest integer, rounding ties away from zero.
func round(x float64) float64 {
	t := math.Trunc(x)
	if math.Abs(x-t) >= 0.5 {
		return t + math.Copysign(1, x)
	}
	return t
}
