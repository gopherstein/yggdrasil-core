package places

import (
	"regexp"
	"strings"
)

// Tag is a kind of place as OpenStreetMap tags it.
type Tag struct {
	Key, Value, Label string
}

// kinds maps the words people use to OpenStreetMap tags. The first word
// that matches decides.
var kinds = []struct {
	words *regexp.Regexp
	tag   Tag
}{
	{regexp.MustCompile(`\b(coffee|caf[eé]s?|espresso)\b`), Tag{"amenity", "cafe", "cafe"}},
	{regexp.MustCompile(`\b(restaurants?|places? to eat|dinner|lunch|food)\b`), Tag{"amenity", "restaurant", "restaurant"}},
	{regexp.MustCompile(`\b(fast food|burgers?)\b`), Tag{"amenity", "fast_food", "fast food"}},
	{regexp.MustCompile(`\b(bars?|pubs?)\b`), Tag{"amenity", "bar", "bar"}},
	{regexp.MustCompile(`\b(pharmac(y|ies)|drug ?stores?|chemists?)\b`), Tag{"amenity", "pharmacy", "pharmacy"}},
	{regexp.MustCompile(`\b(hospitals?|emergency rooms?)\b`), Tag{"amenity", "hospital", "hospital"}},
	{regexp.MustCompile(`\b(clinics?|doctors?)\b`), Tag{"amenity", "clinic", "clinic"}},
	{regexp.MustCompile(`\b(gas|petrol|fuel)( stations?)?\b`), Tag{"amenity", "fuel", "fuel station"}},
	{regexp.MustCompile(`\b(ev )?charging( stations?)?\b`), Tag{"amenity", "charging_station", "charging station"}},
	{regexp.MustCompile(`\bparking\b`), Tag{"amenity", "parking", "parking"}},
	{regexp.MustCompile(`\b(atms?|cash machines?)\b`), Tag{"amenity", "atm", "ATM"}},
	{regexp.MustCompile(`\bbanks?\b`), Tag{"amenity", "bank", "bank"}},
	{regexp.MustCompile(`\b(post offices?)\b`), Tag{"amenity", "post_office", "post office"}},
	{regexp.MustCompile(`\blibrar(y|ies)\b`), Tag{"amenity", "library", "library"}},
	{regexp.MustCompile(`\b(toilets?|restrooms?|bathrooms?)\b`), Tag{"amenity", "toilets", "toilets"}},
	{regexp.MustCompile(`\b(supermarkets?|grocer(y|ies)( stores?)?)\b`), Tag{"shop", "supermarket", "supermarket"}},
	{regexp.MustCompile(`\bbaker(y|ies)\b`), Tag{"shop", "bakery", "bakery"}},
	{regexp.MustCompile(`\b(hardware stores?)\b`), Tag{"shop", "hardware", "hardware store"}},
	{regexp.MustCompile(`\b(bookstores?|book shops?)\b`), Tag{"shop", "books", "bookshop"}},
	{regexp.MustCompile(`\b(hotels?|places? to stay)\b`), Tag{"tourism", "hotel", "hotel"}},
	{regexp.MustCompile(`\bmuseums?\b`), Tag{"tourism", "museum", "museum"}},
	{regexp.MustCompile(`\b(viewpoints?|lookouts?)\b`), Tag{"tourism", "viewpoint", "viewpoint"}},
	{regexp.MustCompile(`\b(campsites?|campgrounds?|camping)\b`), Tag{"tourism", "camp_site", "campsite"}},
	{regexp.MustCompile(`\bparks?\b`), Tag{"leisure", "park", "park"}},
	{regexp.MustCompile(`\bplaygrounds?\b`), Tag{"leisure", "playground", "playground"}},
	{regexp.MustCompile(`\b(gyms?|fitness)\b`), Tag{"leisure", "fitness_centre", "gym"}},
}

// kindOf returns the kind of place a query asks for, if it names one.
func kindOf(query string) (Tag, bool) {
	q := strings.ToLower(query)
	for _, k := range kinds {
		if k.words.MatchString(q) {
			return k.tag, true
		}
	}
	return Tag{}, false
}
