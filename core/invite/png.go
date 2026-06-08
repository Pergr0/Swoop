package invite

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strings"

	"github.com/skip2/go-qrcode"
	qrcoderead "github.com/tuotoo/qrcode"
)

// RenderPNG builds a branded invite card: QR code + short verification code.
// The same blob is embedded in a PNG tEXt chunk (SwoopInvite) for fallback import.
func RenderPNG(bundle Bundle) ([]byte, error) {
	qr, err := qrcode.New(bundle.Blob, qrcode.High)
	if err != nil {
		return nil, err
	}
	qr.DisableBorder = false
	qrImg := qr.Image(280)

	const w, h = 360, 440
	canvas := image.NewRGBA(image.Rect(0, 0, w, h))
	bg := color.RGBA{R: 0x17, G: 0x1d, B: 0x27, A: 0xff}
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: bg}, image.Point{}, draw.Src)

	for y := 0; y < 48; y++ {
		for x := 0; x < w; x++ {
			canvas.Set(x, y, color.RGBA{R: 0x19, G: 0x21, B: 0x2c, A: 0xff})
		}
	}

	qrX := (w - qrImg.Bounds().Dx()) / 2
	draw.Draw(canvas, image.Rect(qrX, 72, qrX+qrImg.Bounds().Dx(), 72+qrImg.Bounds().Dy()), qrImg, image.Point{}, draw.Over)

	drawLabel(canvas, 24, 24, "Swoop")
	drawLabel(canvas, 24, 368, "CODE "+bundle.ShortCode)
	drawLabel(canvas, 24, 396, "INTERNET INVITE")

	var buf bytes.Buffer
	if err := png.Encode(&buf, canvas); err != nil {
		return nil, err
	}
	return appendPNGTextChunk(buf.Bytes(), "SwoopInvite", bundle.Blob)
}

// DecodeFromPNG reads an invite blob from a PNG (QR first, then tEXt fallback).
func DecodeFromPNG(data []byte) (string, error) {
	if blob, err := decodeQRFromPNG(data); err == nil && blob != "" {
		return blob, nil
	}
	if blob, err := pngTextChunk(data, "SwoopInvite"); err == nil && blob != "" {
		return blob, nil
	}
	return "", ErrInvalid
}

func decodeQRFromPNG(data []byte) (string, error) {
	m, err := qrcoderead.Decode(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	return m.Content, nil
}

func drawLabel(dst *image.RGBA, x, y int, text string) {
	c := color.RGBA{R: 0x9f, G: 0xd0, B: 0xff, A: 0xff}
	col := 0
	row := 0
	for _, ch := range strings.ToUpper(text) {
		drawChar(dst, x+col*12, y+row*14, byte(ch), c)
		col++
		if col >= 22 {
			col = 0
			row++
		}
	}
}

func drawChar(dst *image.RGBA, x, y int, ch byte, c color.RGBA) {
	for dy := 0; dy < 7; dy++ {
		for dx := 0; dx < 5; dx++ {
			if simpleGlyph(ch, dx, dy) {
				dst.Set(x+dx, y+dy, c)
			}
		}
	}
}

func simpleGlyph(ch byte, dx, dy int) bool {
	if ch == ' ' {
		return false
	}
	if dy == 0 || dy == 6 {
		return dx > 0 && dx < 4
	}
	if dx == 0 || dx == 4 {
		return dy > 0 && dy < 6
	}
	return (dx+dy)%2 == 0
}

func appendPNGTextChunk(pngData []byte, key, value string) ([]byte, error) {
	if len(pngData) < 12 {
		return nil, ErrInvalid
	}
	iend := bytes.Index(pngData, []byte("IEND"))
	if iend < 4 {
		return nil, ErrInvalid
	}
	iend -= 4

	textData := append([]byte(key), 0)
	textData = append(textData, []byte(value)...)
	chunk := make([]byte, 8+len(textData)+4)
	binary.BigEndian.PutUint32(chunk[0:4], uint32(len(textData)))
	copy(chunk[4:8], "tEXt")
	copy(chunk[8:], textData)
	crc := crc32.ChecksumIEEE(chunk[4 : 8+len(textData)])
	binary.BigEndian.PutUint32(chunk[8+len(textData):], crc)

	out := make([]byte, 0, len(pngData)+len(chunk))
	out = append(out, pngData[:iend]...)
	out = append(out, chunk...)
	out = append(out, pngData[iend:]...)
	return out, nil
}

func pngTextChunk(pngData []byte, key string) (string, error) {
	if len(pngData) < 8 || string(pngData[:8]) != "\x89PNG\r\n\x1a\n" {
		return "", ErrInvalid
	}
	off := 8
	for off+8 <= len(pngData) {
		length := int(binary.BigEndian.Uint32(pngData[off:]))
		ctype := string(pngData[off+4 : off+8])
		dataEnd := off + 8 + length
		if dataEnd+4 > len(pngData) {
			break
		}
		data := pngData[off+8 : dataEnd]
		if ctype == "tEXt" {
			idx := bytes.IndexByte(data, 0)
			if idx < 0 {
				break
			}
			k := string(data[:idx])
			v := string(data[idx+1:])
			if k == key {
				return v, nil
			}
		}
		if ctype == "IEND" {
			break
		}
		off = dataEnd + 4
	}
	return "", ErrInvalid
}
