package compress

import "testing"

var allCaps = Capabilities{
	VideoCodecs:     []string{"h264", "h265", "av1", "vp9"},
	VideoContainers: []string{"mp4", "webm", "mkv", "mov"},
	AudioFormats:    []string{"mp3", "aac", "opus", "ogg", "flac", "wav"},
	ImageFormats:    []string{"jpeg", "png", "webp", "avif"},
}

func TestParseOptions(t *testing.T) {
	cases := []struct {
		name    string
		typ     FileType
		raw     string
		caps    Capabilities
		wantErr bool
	}{
		{"video defaults", FileTypeVideo, "", allCaps, false},
		{"video vp9 in mp4", FileTypeVideo, `{"codec":"vp9","container":"mp4"}`, allCaps, true},
		{"video vp9 in webm", FileTypeVideo, `{"codec":"vp9","container":"webm"}`, allCaps, false},
		{"video codec unavailable", FileTypeVideo, `{"codec":"av1"}`, Capabilities{VideoCodecs: []string{"h264"}, VideoContainers: []string{"mp4"}}, true},
		{"video size mode needs target", FileTypeVideo, `{"mode":"size"}`, allCaps, true},
		{"video size mode", FileTypeVideo, `{"mode":"size","target_size_mb":8}`, allCaps, false},
		{"video odd resolution", FileTypeVideo, `{"resolution":123}`, allCaps, true},
		{"video bad trim", FileTypeVideo, `{"trim":{"start":5,"end":2}}`, allCaps, true},
		{"video bad json", FileTypeVideo, `{"quality":"high"}`, allCaps, true},
		{"audio defaults", FileTypeAudio, "", allCaps, false},
		{"audio unknown format", FileTypeAudio, `{"format":"wma"}`, allCaps, true},
		{"audio 3 channels", FileTypeAudio, `{"channels":3}`, allCaps, true},
		{"audio odd sample rate", FileTypeAudio, `{"sample_rate":12345}`, allCaps, true},
		{"image defaults", FileTypeImage, "", allCaps, false},
		{"image avif unavailable", FileTypeImage, `{"format":"avif"}`, Capabilities{ImageFormats: []string{"jpeg"}}, true},
		{"image huge resize", FileTypeImage, `{"max_width":99999}`, allCaps, true},
		{"unknown type", FileType("doc"), "", allCaps, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseOptions(tc.typ, []byte(tc.raw), tc.caps)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestParseOptionsClamps(t *testing.T) {
	o, err := ParseOptions(FileTypeVideo, []byte(`{"quality":500,"audio_bitrate":5}`), allCaps)
	if err != nil {
		t.Fatal(err)
	}
	if o.Video.Quality != 100 || o.Video.AudioBitrate != 32 {
		t.Fatalf("got quality %d bitrate %d", o.Video.Quality, o.Video.AudioBitrate)
	}
}

func TestTrimWindow(t *testing.T) {
	cases := []struct {
		trim               Trim
		duration           float64
		wantStart, wantLen float64
	}{
		{Trim{}, 10, 0, 10},
		{Trim{Start: 2}, 10, 2, 8},
		{Trim{Start: 2, End: 5}, 10, 2, 3},
		{Trim{End: 50}, 10, 0, 10},
		{Trim{Start: 1}, 0, 1, 0},
	}
	for _, tc := range cases {
		s, l := tc.trim.Window(tc.duration)
		if s != tc.wantStart || l != tc.wantLen {
			t.Errorf("%+v over %v: got (%v, %v), want (%v, %v)", tc.trim, tc.duration, s, l, tc.wantStart, tc.wantLen)
		}
	}
}
