package common

import "testing"

// Cases mirror what a live deployment reports: CB-Spider declares
// ZoneBasedControl=true for every driver but KT, so the region's zone list is
// what actually decides most outcomes.
func TestDecideZoneCapability(t *testing.T) {
	tests := []struct {
		name        string
		zoneControl bool
		zones       []string
		region      string
		wantShift   bool
	}{
		{"aws us-west-2", true, []string{"us-west-2a", "us-west-2b", "us-west-2c", "us-west-2d"}, "us-west-2", true},
		{"gcp us-west3", true, []string{"us-west3-a", "us-west3-b", "us-west3-c"}, "us-west3", true},
		// Azure westus genuinely has no zones; 10 of its 48 regions are like this.
		{"azure westus (no zones)", true, nil, "westus", false},
		// KT and KT Classic are the only drivers declaring ZoneBasedControl=false.
		{"kt (no zone control)", false, []string{"z1", "z2"}, "kr1", false},
		{"single-zone region", true, []string{"only-1a"}, "kr1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			zc := decideZoneCapability(tt.zoneControl, tt.zones, tt.region)
			if zc.Shiftable != tt.wantShift {
				t.Errorf("shiftable = %v, want %v", zc.Shiftable, tt.wantShift)
			}
			if !zc.Shiftable && zc.Reason == "" {
				t.Error("a non-shiftable capability must explain why")
			}
			if zc.Shiftable && zc.Reason != "" {
				t.Errorf("shiftable capability should carry no reason, got %q", zc.Reason)
			}
		})
	}
}
