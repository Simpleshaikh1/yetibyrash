package notify

import "testing"

func TestFillBodyDropsEmptyLines(t *testing.T) {
	body := "Hi {first_name},\n\nWith {courier}.\nTracking number: {tracking_number} {tracking_link}\n{note}\nTrack: {track_url}"
	got := FillBody(body, map[string]string{
		"first_name": "Ada", "courier": "Own rider", "tracking_number": "", "tracking_link": "", "note": "", "track_url": "http://x/track",
	})
	want := "Hi Ada,\n\nWith Own rider.\nTrack: http://x/track\n"
	if got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
	got = FillBody("Tracking number: {tracking_number} {tracking_link}", map[string]string{"tracking_number": "AB12", "tracking_link": ""})
	if got != "Tracking number: AB12\n" {
		t.Fatalf("got %q", got)
	}
}
