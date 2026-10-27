package app

import "testing"

func TestImageResolutionFromSizeSupportsSeedream15k(t *testing.T) {
	cases := map[string]string{
		"1024x1024": "1k",
		"1424x800":  "1k",
		"1536x1536": "1.5k",
		"2048x1152": "1.5k",
		"2352x1008": "1.5k",
		"2048x2048": "2k",
		"2816x1584": "2k",
		"3840x2160": "4k",
	}
	for size, want := range cases {
		if got := imageResolutionFromSize(size); got != want {
			t.Fatalf("imageResolutionFromSize(%q) = %q, want %q", size, got, want)
		}
	}
}

func TestNormalizeImagePriceQualityKeepsSeedream15k(t *testing.T) {
	if got := normalizeImagePriceQuality("", "1536x1536"); got != "1.5k" {
		t.Fatalf("normalizeImagePriceQuality(1536x1536) = %q, want 1.5k", got)
	}
	if got := normalizeImagePriceQuality("1.5k", ""); got != "1.5k" {
		t.Fatalf("normalizeImagePriceQuality(1.5k) = %q, want 1.5k", got)
	}
}

func TestIsImageResolutionTierAccepts15k(t *testing.T) {
	if !isImageResolutionTier("1.5K") {
		t.Fatal("isImageResolutionTier(1.5K) = false, want true")
	}
}

func TestMatchSKUSelectorRequiresConfiguredSeedream15kPrice(t *testing.T) {
	// 不借用相邻档价格；1.5K 必须由已配置的条件精确匹配。
	for _, expected := range []string{"1k", "2k"} {
		tier := map[string]string{"imageResolution": expected}
		requested := map[string]string{"imageResolution": "1.5k"}
		matched, _ := matchSKUSelector(tier, requested)
		if matched {
			t.Fatalf("unconfigured 1.5k borrowed price from %v", tier)
		}
	}
	// 精确命中 1.5K 档，且 4K 档不参与回退。
	if matched, _ := matchSKUSelector(map[string]string{"imageResolution": "1.5k"}, map[string]string{"imageResolution": "1.5k"}); !matched {
		t.Fatal("exact 1.5k selector should match")
	}
	if matched, _ := matchSKUSelector(map[string]string{"imageResolution": "4k"}, map[string]string{"imageResolution": "1.5k"}); matched {
		t.Fatal("4k selector must not fall back to 1.5k")
	}
}
