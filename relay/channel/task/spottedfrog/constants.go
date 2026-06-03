package spottedfrog

import (
	"strings"

	"github.com/QuantumNous/new-api/dto"
)

const ChannelName = "spottedfrog"

var ModelList = []string{
	"sora-2",
	"omni_flash",
	"grok-imagine-video",
	"veo",
}

const (
	defaultBaseURL = "https://api.hellobabygo.com"
	videosEndpoint = "/videos"
)

func defaultSpottedFrogModelMap() dto.SpottedFrogModelMap {
	return dto.SpottedFrogModelMap{
		Sora216x94s:        "sora-2-4s-16x9",
		Sora216x98s:        "sora-2-8s-16x9",
		Sora216x912s:       "sora-2-12s-16x9",
		Sora29x164s:        "sora-2-4s-9x16",
		Sora29x168s:        "sora-2-8s-9x16",
		Sora29x1612s:       "sora-2-12s-9x16",
		Sora2Pro16x912s:    "sora2-pro-12s-16x9",
		Sora2Pro9x1612s:    "sora2-pro-12s-9x16",
		OmniFlash:          "omni_flash",
		GrokImagineVideo:   "grok-imagine-video",
		VeoFast16x98s1080p: "firefly-veo31-fast-8s-16x9-1080p",
		VeoFast9x168s1080p: "firefly-veo31-fast-8s-9x16-1080p",
		VeoStd16x98s1080p:  "firefly-veo31-standard-8s-16x9-1080p",
		VeoStd9x168s1080p:  "firefly-veo31-standard-8s-9x16-1080p",
		VeoRef16x98s1080p:  "firefly-veo31-ref-8s-16x9-1080p",
		VeoRef9x168s1080p:  "firefly-veo31-ref-8s-9x16-1080p",
	}
}

func mergeSpottedFrogModelMap(overrides *dto.SpottedFrogModelMap) dto.SpottedFrogModelMap {
	effective := defaultSpottedFrogModelMap()
	if overrides == nil {
		return effective
	}
	applySpottedFrogOverride(&effective.Sora216x94s, overrides.Sora216x94s)
	applySpottedFrogOverride(&effective.Sora216x98s, overrides.Sora216x98s)
	applySpottedFrogOverride(&effective.Sora216x912s, overrides.Sora216x912s)
	applySpottedFrogOverride(&effective.Sora29x164s, overrides.Sora29x164s)
	applySpottedFrogOverride(&effective.Sora29x168s, overrides.Sora29x168s)
	applySpottedFrogOverride(&effective.Sora29x1612s, overrides.Sora29x1612s)
	applySpottedFrogOverride(&effective.Sora2Pro16x912s, overrides.Sora2Pro16x912s)
	applySpottedFrogOverride(&effective.Sora2Pro9x1612s, overrides.Sora2Pro9x1612s)
	applySpottedFrogOverride(&effective.OmniFlash, overrides.OmniFlash)
	applySpottedFrogOverride(&effective.GrokImagineVideo, overrides.GrokImagineVideo)
	applySpottedFrogOverride(&effective.VeoFast16x98s1080p, overrides.VeoFast16x98s1080p)
	applySpottedFrogOverride(&effective.VeoFast9x168s1080p, overrides.VeoFast9x168s1080p)
	applySpottedFrogOverride(&effective.VeoStd16x98s1080p, overrides.VeoStd16x98s1080p)
	applySpottedFrogOverride(&effective.VeoStd9x168s1080p, overrides.VeoStd9x168s1080p)
	applySpottedFrogOverride(&effective.VeoRef16x98s1080p, overrides.VeoRef16x98s1080p)
	applySpottedFrogOverride(&effective.VeoRef9x168s1080p, overrides.VeoRef9x168s1080p)
	return effective
}

func applySpottedFrogOverride(dst *string, src string) {
	if value := strings.TrimSpace(src); value != "" {
		*dst = value
	}
}
