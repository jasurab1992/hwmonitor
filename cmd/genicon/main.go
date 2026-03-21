//go:build ignore

package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
)

func main() {
	os.MkdirAll("assets", 0755)
	sizes := []int{16, 32, 48, 256}
	var pngs [][]byte
	for _, sz := range sizes {
		var buf bytes.Buffer
		_ = png.Encode(&buf, renderIcon(sz))
		pngs = append(pngs, buf.Bytes())
	}
	f, err := os.Create("assets/hwmonitor.ico")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	writeICO(f, sizes, pngs)
}

func writeICO(w io.Writer, sizes []int, pngs [][]byte) {
	n := len(sizes)
	w16 := func(v uint16) {
		b := [2]byte{}
		binary.LittleEndian.PutUint16(b[:], v)
		w.Write(b[:])
	}
	w32 := func(v uint32) {
		b := [4]byte{}
		binary.LittleEndian.PutUint32(b[:], v)
		w.Write(b[:])
	}
	w16(0)         // reserved
	w16(1)         // type: icon
	w16(uint16(n)) // count

	offset := uint32(6 + n*16)
	for i, sz := range sizes {
		wb := byte(sz)
		if sz == 256 {
			wb = 0
		}
		w.Write([]byte{wb, wb, 0, 0}) // w, h, colors, reserved
		w16(1)                        // planes
		w16(32)                       // bit depth
		w32(uint32(len(pngs[i])))
		w32(offset)
		offset += uint32(len(pngs[i]))
	}
	for _, p := range pngs {
		w.Write(p)
	}
}

var (
	cBg     = color.NRGBA{13, 17, 23, 255}   // #0D1117 deep dark
	cChip   = color.NRGBA{22, 33, 48, 255}   // #162130 chip body
	cBorder = color.NRGBA{0, 220, 180, 255}  // #00DCB4 teal
	cWave   = color.NRGBA{57, 255, 142, 255} // #39FF8E neon green
	cHot    = color.NRGBA{255, 100, 40, 255} // #FF6428 orange peak
	cAlpha  = color.NRGBA{0, 0, 0, 0}        // transparent
)

func renderIcon(sz int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, sz, sz))

	set := func(x, y int, c color.NRGBA) {
		if x >= 0 && x < sz && y >= 0 && y < sz {
			img.SetNRGBA(x, y, c)
		}
	}
	fill := func(x0, y0, x1, y1 int, c color.NRGBA) {
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				set(x, y, c)
			}
		}
	}

	// Background
	fill(0, 0, sz, sz, cBg)

	// Rounded corners (radius ~13% of size)
	r := int(float64(sz)*0.13 + 0.5)
	if r < 2 {
		r = 2
	}
	for y := 0; y < sz; y++ {
		for x := 0; x < sz; x++ {
			dx, dy := 0, 0
			if x < r {
				dx = r - x
			} else if x >= sz-r {
				dx = x - (sz - r - 1)
			}
			if y < r {
				dy = r - y
			} else if y >= sz-r {
				dy = y - (sz - r - 1)
			}
			if dx > 0 || dy > 0 {
				if float64(dx*dx+dy*dy) > float64(r*r) {
					img.SetNRGBA(x, y, cAlpha)
				}
			}
		}
	}

	// Chip body: 25%..75% of icon
	sc := func(v float64) int { return int(v*float64(sz) + 0.5) }
	cx0, cx1 := sc(0.25), sc(0.75)
	cy0, cy1 := sc(0.25), sc(0.75)

	// Pins: 3 per side, width ~6% of icon, length from edge to chip
	pw := sc(0.06)
	if pw < 1 {
		pw = 1
	}
	gap := sc(0.06) // gap between pin tip and icon edge

	for i := 0; i < 3; i++ {
		t := float64(i+1) / 4.0
		// left & right
		py := cy0 + int(float64(cy1-cy0)*t)
		for y := py - pw/2; y <= py+pw/2; y++ {
			for x := gap; x < cx0; x++ {
				set(x, y, cBorder)
			}
			for x := cx1; x < sz-gap; x++ {
				set(x, y, cBorder)
			}
		}
		// top & bottom
		px := cx0 + int(float64(cx1-cx0)*t)
		for x := px - pw/2; x <= px+pw/2; x++ {
			for y := gap; y < cy0; y++ {
				set(x, y, cBorder)
			}
			for y := cy1; y < sz-gap; y++ {
				set(x, y, cBorder)
			}
		}
	}

	// Chip body fill
	fill(cx0, cy0, cx1, cy1, cChip)

	// Chip border outline
	bw := 1
	if sz >= 128 {
		bw = 2
	}
	for b := 0; b < bw; b++ {
		for x := cx0 + b; x < cx1-b; x++ {
			set(x, cy0+b, cBorder)
			set(x, cy1-1-b, cBorder)
		}
		for y := cy0 + b; y < cy1-b; y++ {
			set(cx0+b, y, cBorder)
			set(cx1-1-b, y, cBorder)
		}
	}

	// ECG waveform inside chip
	my := (cy0 + cy1) / 2
	wm := sc(0.09)
	wx0 := cx0 + wm + bw
	wx1 := cx1 - wm - bw
	ww := wx1 - wx0
	peakH := (cy1 - cy0) * 32 / 100

	// Baseline
	for x := wx0; x < wx1; x++ {
		set(x, my, cWave)
	}

	// Spike occupies middle 40% of waveform width
	spikeStart := wx0 + ww*3/10
	spikeEnd := wx0 + ww*7/10
	sw := spikeEnd - spikeStart
	if sw < 1 {
		sw = 1
	}

	for xi := spikeStart; xi < spikeEnd; xi++ {
		t := float64(xi-spikeStart) / float64(sw)

		var y int
		var col color.NRGBA
		switch {
		case t < 0.20:
			y = my - int(float64(peakH)*t/0.20)
			col = cWave
		case t < 0.40:
			y = my - peakH
			col = cHot
		case t < 0.65:
			y = my - peakH + int(float64(peakH+peakH/2)*(t-0.40)/0.25)
			col = cHot
		case t < 0.85:
			y = my + peakH/2 - int(float64(peakH/2)*(t-0.65)/0.20)
			col = cWave
		default:
			y = my
			col = cWave
		}
		// clamp inside chip
		if y < cy0+bw+1 {
			y = cy0 + bw + 1
		}
		if y > cy1-bw-2 {
			y = cy1 - bw - 2
		}

		// At larger sizes draw a vertical line to fill the wave body
		if sz >= 32 {
			lo, hi := y, my
			if lo > hi {
				lo, hi = hi, lo
			}
			for yy := lo; yy <= hi; yy++ {
				set(xi, yy, col)
			}
		} else {
			set(xi, y, col)
		}
	}

	return img
}
