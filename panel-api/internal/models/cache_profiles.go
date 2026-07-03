package models

// cache_profiles.go — GH #618. Named cache profiles per site type. A profile is
// a preset of extra page-cache bypass path prefixes applied ON TOP of the
// built-in wp-admin/login/cart/checkout/my-account/REST set (rendered by the
// nginx vhost template). The agent re-sanitizes every path before folding it
// into the bypass regex, so this is advisory convenience, not a trust boundary.

// CacheProfile is one named preset.
type CacheProfile struct {
	Key     string   `json:"key"`
	Label   string   `json:"label"`
	Paths   []string `json:"paths"`
	Warning string   `json:"warning"`
}

// CacheProfiles is the registry exposed to the UI (GET /applications/cache-profiles).
var CacheProfiles = []CacheProfile{
	{
		Key:     "",
		Label:   "Default (brochure)",
		Paths:   nil,
		Warning: "Standard caching for a content/brochure site. Anonymous pages are cached; wp-admin, login, cart, and checkout always bypass.",
	},
	{
		Key:     "woocommerce",
		Label:   "WooCommerce",
		Paths:   []string{"/cart", "/checkout", "/my-account", "/wc-api", "/store"},
		Warning: "Cart, checkout, and account pages always bypass. Never cache logged-in shopper sessions — the cookie gate handles that, but do not add store cookies to any allowlist.",
	},
	{
		Key:     "edd",
		Label:   "Easy Digital Downloads",
		Paths:   []string{"/checkout", "/purchase-confirmation", "/purchase-history", "/downloads"},
		Warning: "Checkout and purchase-history pages always bypass so download links + receipts stay per-user.",
	},
	{
		Key:     "membership_lms",
		Label:   "Membership / LMS",
		Paths:   []string{"/account", "/membership", "/members", "/courses", "/lessons", "/quizzes", "/dashboard"},
		Warning: "Member and course pages bypass the cache. Logged-in members are already bypassed by the session-cookie gate; verify your membership plugin sets a standard login cookie.",
	},
	{
		Key:     "custom",
		Label:   "Custom",
		Paths:   nil,
		Warning: "No preset paths — define your own bypass prefixes in URL exclusions below.",
	},
}

// CacheProfilePaths returns the preset bypass paths for a profile key.
// An unknown key (or "") yields nil (the built-in set still applies).
func CacheProfilePaths(key string) []string {
	for i := range CacheProfiles {
		if CacheProfiles[i].Key == key {
			return CacheProfiles[i].Paths
		}
	}
	return nil
}
