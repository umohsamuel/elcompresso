package image

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
)

// readJPEGOrientation returns the EXIF orientation (1..8) of a JPEG file, or 1
// if it is missing or unreadable.
func readJPEGOrientation(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 1
	}
	defer f.Close()

	head := make([]byte, 256*1024)
	n, _ := io.ReadFull(f, head)
	return parseJPEGOrientation(head[:n])
}

func parseJPEGOrientation(b []byte) int {
	if len(b) < 4 || b[0] != 0xFF || b[1] != 0xD8 {
		return 1
	}
	for i := 2; i+4 <= len(b); {
		if b[i] != 0xFF {
			return 1
		}
		marker := b[i+1]
		if marker == 0xD8 || (marker >= 0xD0 && marker <= 0xD7) || marker == 0x01 || marker == 0xFF {
			i += 2
			continue
		}
		if marker == 0xDA || marker == 0xD9 { // start of scan / end of image
			return 1
		}
		size := int(binary.BigEndian.Uint16(b[i+2:]))
		end := i + 2 + size
		if size < 2 || end > len(b) {
			return 1
		}
		if marker == 0xE1 && bytes.HasPrefix(b[i+4:end], []byte("Exif\x00\x00")) {
			if o := parseTIFFOrientation(b[i+10 : end]); o != 0 {
				return o
			}
		}
		i = end
	}
	return 1
}

func parseTIFFOrientation(t []byte) int {
	if len(t) < 8 {
		return 0
	}
	var order binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 0
	}
	ifd := int(order.Uint32(t[4:]))
	if ifd+2 > len(t) {
		return 0
	}
	count := int(order.Uint16(t[ifd:]))
	for e := 0; e < count; e++ {
		off := ifd + 2 + e*12
		if off+12 > len(t) {
			return 0
		}
		if order.Uint16(t[off:]) == 0x0112 { // Orientation, type SHORT
			if v := int(order.Uint16(t[off+8:])); v >= 1 && v <= 8 {
				return v
			}
			return 0
		}
	}
	return 0
}

// orientationFilters returns the ffmpeg filters that turn an image stored with
// the given EXIF orientation into its upright form.
func orientationFilters(o int) []string {
	switch o {
	case 2:
		return []string{"hflip"}
	case 3:
		return []string{"hflip", "vflip"}
	case 4:
		return []string{"vflip"}
	case 5:
		return []string{"transpose=cclock_flip"}
	case 6:
		return []string{"transpose=clock"}
	case 7:
		return []string{"transpose=clock_flip"}
	case 8:
		return []string{"transpose=cclock"}
	}
	return nil
}
