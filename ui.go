//go:build windows

package main

import (
	"bytes"
	_ "embed"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"unsafe"

	"github.com/gonutz/w32/v2"
	"github.com/gonutz/wui/v2"
)

//go:embed assets/logo.png
var logoBytes []byte

//go:embed assets/icon.ico
var iconBytes []byte

// Tibia-inspired dark color palette (COLORREF = 0x00BBGGRR).
var (
	colBackground = w32.COLORREF(0x00100A05) // near-black warm
	colText       = w32.COLORREF(0x00C8E0F0) // parchment cream
	colProgress   = w32.COLORREF(0x0050C8FF) // golden amber
	colShimmer    = w32.COLORREF(0x00D0F8FF) // light shimmer
	colTrack      = w32.COLORREF(0x00182030) // dark bar track
)

var (
	bgBrush    w32.HBRUSH
	logoBitmap w32.HBITMAP
	logoWidth  int
	logoHeight int

	progressHWND  w32.HWND
	progressValue = float64(-1) // -1 = hidden, 0..1 = visible
	animTick      int
)

// colorrefFromImage returns a COLORREF (0x00BBGGRR) for pixel (x, y) in img.
func colorrefFromImage(img image.Image, x, y int) w32.COLORREF {
	r, g, b, _ := img.At(x, y).RGBA()
	return w32.COLORREF(uint32(b>>8)<<16 | uint32(g>>8)<<8 | uint32(r>>8))
}

func initUI(hwnd w32.HWND) {
	progressHWND = hwnd

	img, err := png.Decode(bytes.NewReader(logoBytes))
	if err == nil {
		// Sample logo corner to match window background color exactly.
		colBackground = colorrefFromImage(img, 0, 0)
		logoBitmap, logoWidth, logoHeight = imageToHBITMAP(img)
	}
	bgBrush = w32.CreateSolidBrush(uint32(colBackground))
}

// LoadEmbeddedIcon writes the embedded icon to a temp file and returns a
// wui.Icon. The caller should not delete the file; it is cleaned up on exit.
func LoadEmbeddedIcon() *wui.Icon {
	tmp := filepath.Join(os.TempDir(), "rapaduraot-launcher.ico")
	if os.WriteFile(tmp, iconBytes, 0600) != nil {
		return nil
	}
	icon, err := wui.NewIconFromFile(tmp)
	if err != nil {
		return nil
	}
	return icon
}

func destroyUI() {
	if bgBrush != 0 {
		w32.DeleteObject(w32.HGDIOBJ(bgBrush))
	}
	if logoBitmap != 0 {
		w32.DeleteObject(w32.HGDIOBJ(logoBitmap))
	}
}

func setProgress(v float64) {
	progressValue = v
	if progressHWND != 0 {
		w32.InvalidateRect(progressHWND, nil, true)
	}
}

func tickAnimation() {
	animTick++
	if progressValue >= 0 && progressHWND != 0 {
		w32.InvalidateRect(progressHWND, nil, true)
	}
}

func drawBackground(hdc w32.HDC, winW, winH int) {
	bg := w32.RECT{Left: 0, Top: 0, Right: int32(winW), Bottom: int32(winH)}
	w32.FillRect(hdc, &bg, bgBrush)

	if logoBitmap != 0 {
		drawLogo(hdc, winW, winH)
	}

	if progressValue >= 0 {
		drawProgressBar(hdc, int32(winW))
	}
}

func drawLogo(hdc w32.HDC, winW, winH int) {
	maxW := winW - 40
	maxH := winH / 3

	dstW := maxW
	dstH := logoHeight * dstW / logoWidth
	if dstH > maxH {
		dstH = maxH
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

func drawProgressBar(hdc w32.HDC, totalW int32) {
	const (
		padX = int32(30)
		barY = int32(158)
		barH = int32(10)
	)
	barW := totalW - padX*2

	// Track
	track := w32.RECT{Left: padX, Top: barY, Right: padX + barW, Bottom: barY + barH}
	trackBrush := w32.CreateSolidBrush(uint32(colTrack))
	w32.FillRect(hdc, &track, trackBrush)
	w32.DeleteObject(w32.HGDIOBJ(trackBrush))

	fillW := int32(float64(barW) * progressValue)
	if fillW <= 0 {
		return
	}

	// Golden fill
	fill := w32.RECT{Left: padX, Top: barY, Right: padX + fillW, Bottom: barY + barH}
	fillBrush := w32.CreateSolidBrush(uint32(colProgress))
	w32.FillRect(hdc, &fill, fillBrush)
	w32.DeleteObject(w32.HGDIOBJ(fillBrush))

	// Animated shimmer stripe
	const shimSize = int32(60)
	offset := int32(animTick*5) % (barW + shimSize)
	shimLeft := padX + offset - shimSize
	shimRight := shimLeft + shimSize
	if shimLeft < padX {
		shimLeft = padX
	}
	if shimRight > padX+fillW {
		shimRight = padX + fillW
	}
	if shimLeft < shimRight {
		shim := w32.RECT{Left: shimLeft, Top: barY, Right: shimRight, Bottom: barY + barH}
		shimBrush := w32.CreateSolidBrush(uint32(colShimmer))
		w32.FillRect(hdc, &shim, shimBrush)
		w32.DeleteObject(w32.HGDIOBJ(shimBrush))
	}
}

func handleCtlColorStatic(wParam uintptr) (bool, uintptr) {
	hdc := w32.HDC(wParam)
	w32.SetTextColor(hdc, colText)
	w32.SetBkMode(hdc, w32.TRANSPARENT)
	if bgBrush != 0 {
		return true, uintptr(bgBrush)
	}
	return false, 0
}

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
			BiHeight:      -int32(h),
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
			slice[off+0] = byte(b >> 8)
			slice[off+1] = byte(g >> 8)
			slice[off+2] = byte(r >> 8)
			slice[off+3] = 0
		}
	}

	return hbmp, w, h
}
