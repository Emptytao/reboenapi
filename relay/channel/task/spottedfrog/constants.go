package spottedfrog

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

type veoModelKey struct {
	Mode       string
	Duration   int
	Aspect     string
	Resolution string
}

var veoModelMap = map[veoModelKey]string{
	{Mode: "fast", Duration: 8, Aspect: "16x9", Resolution: "1080p"}:     "firefly-veo31-fast-8s-16x9-1080p",
	{Mode: "fast", Duration: 8, Aspect: "9x16", Resolution: "1080p"}:     "firefly-veo31-fast-8s-9x16-1080p",
	{Mode: "standard", Duration: 8, Aspect: "16x9", Resolution: "1080p"}: "firefly-veo31-standard-8s-16x9-1080p",
	{Mode: "standard", Duration: 8, Aspect: "9x16", Resolution: "1080p"}: "firefly-veo31-standard-8s-9x16-1080p",
	{Mode: "ref", Duration: 8, Aspect: "16x9", Resolution: "1080p"}:      "firefly-veo31-ref-8s-16x9-1080p",
	{Mode: "ref", Duration: 8, Aspect: "9x16", Resolution: "1080p"}:      "firefly-veo31-ref-8s-9x16-1080p",
}
