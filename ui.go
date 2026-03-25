//go:build windows

package main

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"unsafe"

	"github.com/gonutz/w32/v2"
)

// Tibia-inspired dark color palette.
// COLORREF format: 0x00BBGGRR
var (
	colBackground = w32.COLORREF(0x00050A10) // near-black, dark blue-brown
	colTitle      = w32.COLORREF(0x0050C8FF) // warm amber/gold
	colText       = w32.COLORREF(0x00C8E0F0) // parchment cream
	colProgress   = w32.COLORREF(0x0050C8FF) // same as title
)

var (
	bgBrush   w32.HBRUSH // dark background brush, created once
	logoBitmap w32.HBITMAP
	logoWidth  int
	logoHeight int
)

// initUI must be called once after the window exists (from WM_CREATE).
// It creates brushes and loads the logo bitmap from the install directory.
func initUI(installDir string) {
	bgBrush = w32.CreateSolidBrush(uint32(colBackground))
	loadLogo(installDir)
}

func destroyUI() {
	if bgBrush != 0 {
		w32.DeleteObject(w32.HGDIOBJ(bgBrush))
	}
	if logoBitmap != 0 {
		w32.DeleteObject(w32.HGDIOBJ(logoBitmap))
	}
}

func loadLogo(installDir string) {
	logoPath := filepath.Join(installDir, "data", "images", "logo_home.png")
	f, err := os.Open(logoPath)
	if err != nil {
		return
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		return
	}

	logoBitmap, logoWidth, logoHeight = imageToHBITMAP(img)
}

// imageToHBITMAP converts a Go image to a Windows HBITMAP (32-bit BGRA, top-down DIB).
func imageToHBITMAP(img image.Image) (w32.HBITMAP, int, int) {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w == 0 || h == 0 {
		return 0, 0, 0
	}

	bi := w32.BITMAPINFO{
		BmiHeader: w32.BITMAPINFOHEADER{
			BiSize:        uint32(unsafe.Sizeof(w32.BITMAPINFOHEADER{})),
			BiWidth:       int32(w),
			BiHeight:      -int32(h), // negative = top-down
			BiPlanes:      1,
			BiBitCount:    32,
			BiCompression: w32.BI_RGB,
		},
	}

	var pixels unsafe.Pointer
	hbmp := w32.CreateDIBSection(0, &bi, w32.DIB_RGB_COLORS, &pixels, 0, 0)
	if hbmp == 0 || pixels == nil {
		return 0, 0, 0
	}

	stride := w * 4
	slice := unsafe.Slice((*byte)(pixels), stride*h)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, _ := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			off := (y*w + x) * 4
			slice[off+0] = byte(b >> 8) // B
			slice[off+1] = byte(g >> 8) // G
			slice[off+2] = byte(r >> 8) // R
			slice[off+3] = 0
		}
	}

	return hbmp, w, h
}

// drawBackground paints the dark background and the logo onto the given DC,
// sized to fill (winW x winH).
func drawBackground(hdc w32.HDC, winW, winH int) {
	// Fill background with dark color.
	rect := w32.RECT{Left: 0, Top: 0, Right: int32(winW), Bottom: int32(winH)}
	w32.FillRect(hdc, &rect, bgBrush)

	if logoBitmap == 0 {
		return
	}

	// Draw logo centred horizontally near the top.
	maxLogoW := winW - 40
	maxLogoH := winH / 3

	dstW := maxLogoW
	dstH := logoHeight * dstW / logoWidth
	if dstH > maxLogoH {
		dstH = maxLogoH
		dstW = logoWidth * dstH / logoHeight
	}
	dstX := (winW - dstW) / 2
	dstY := 16

	memDC := w32.CreateCompatibleDC(hdc)
	defer w32.DeleteDC(memDC)

	old := w32.SelectObject(memDC, w32.HGDIOBJ(logoBitmap))
	defer w32.SelectObject(memDC, old)

	w32.SetStretchBltMode(hdc, w32.STRETCH_HALFTONE)
	w32.StretchBlt(hdc, dstX, dstY, dstW, dstH,
		memDC, 0, 0, logoWidth, logoHeight, w32.SRCCOPY)
}

// handleCtlColorStatic processes WM_CTLCOLORSTATIC so labels use our palette.
func handleCtlColorStatic(wParam uintptr) (bool, uintptr) {
	hdc := w32.HDC(wParam)
	w32.SetTextColor(hdc, colText)
	w32.SetBkMode(hdc, w32.TRANSPARENT)
	if bgBrush != 0 {
		return true, uintptr(bgBrush)
	}
	return false, 0
}
