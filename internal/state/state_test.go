package state

import (
	"testing"

	"github.com/mevdschee/logirgb/lighting"
)

func TestKeptProfileZone(t *testing.T) {
	all := lighting.Effect{Name: "static"}
	d := &Device{}
	d.SetZone(0, all)
	d.KeepProfileZone(2, 0)
	if _, ok := d.ProfileZone(2, 0); ok {
		t.Fatal("kept zone takes the effect for all profiles")
	}
	if _, ok := d.ProfileZone(1, 0); !ok {
		t.Fatal("other profile lost the effect for all profiles")
	}
	own := lighting.Effect{Name: "breathing"}
	d.SetProfileZone(2, 0, own)
	if e, ok := d.ProfileZone(2, 0); !ok || e.Name != own.Name {
		t.Fatalf("set zone = %v, %v; want %v", e, ok, own)
	}
	d.KeepProfileZone(3, 0)
	d.SetAllProfilesZone(0, all)
	if e, ok := d.ProfileZone(3, 0); !ok || e.Name != all.Name {
		t.Fatalf("zone after setting all profiles = %v, %v; want %v", e, ok, all)
	}
	if len(d.Kept) != 0 {
		t.Fatalf("kept zones left: %v", d.Kept)
	}
}

func TestProfilesDoNotShareValues(t *testing.T) {
	d := &Device{}
	d.SetZone(1, lighting.Effect{Name: "static", Values: map[lighting.Param]int{"color": 0xff4080}})
	// The TUI edits the effect it got for one profile in place.
	e, _ := d.ProfileZone(2, 1)
	e.Set("color", 0x00ff00)
	d.SetProfileZone(2, 1, e)
	e.Set("color", 0x0000ff)
	if got, _ := d.ProfileZone(1, 1); got.Values["color"] != 0xff4080 {
		t.Fatalf("profile 1 color = %06x, want ff4080", got.Values["color"])
	}
	if got, _ := d.ProfileZone(2, 1); got.Values["color"] != 0x00ff00 {
		t.Fatalf("profile 2 color = %06x, want 00ff00", got.Values["color"])
	}
}
