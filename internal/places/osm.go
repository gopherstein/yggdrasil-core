package places

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Place is a place or address.
type Place struct {
	Name     string  `json:"name"`
	Category string  `json:"category,omitempty"`
	Address  string  `json:"address,omitempty"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	// ID is the OpenStreetMap object, such as node/3430732134, for
	// places.details.
	ID       string `json:"id,omitempty"`
	URL      string `json:"url,omitempty"`
	Distance *Dist  `json:"distance,omitempty"`
	Hours    string `json:"opening_hours,omitempty"`
	Phone    string `json:"phone,omitempty"`
	Website  string `json:"website,omitempty"`
	Cuisine  string `json:"cuisine,omitempty"`
}

// Dist is a distance in both units, so the answer can use the person's.
type Dist struct {
	Meters int     `json:"meters"`
	Km     float64 `json:"km"`
	Miles  float64 `json:"miles"`
}

func dist(m float64) *Dist {
	return &Dist{Meters: int(math.Round(m)), Km: math.Round(m/100) / 10, Miles: math.Round(m/160.9344) / 10}
}

// haversine is the straight-line distance in meters.
func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371000
	rad := math.Pi / 180
	dLat, dLon := (lat2-lat1)*rad, (lon2-lon1)*rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * r * math.Asin(math.Sqrt(a))
}

func osmURL(kind string, id int64) string {
	return "https://www.openstreetmap.org/" + kind + "/" + strconv.FormatInt(id, 10)
}

type nominatimPlace struct {
	OSMType     string            `json:"osm_type"`
	OSMID       int64             `json:"osm_id"`
	Lat         string            `json:"lat"`
	Lon         string            `json:"lon"`
	Category    string            `json:"category"`
	Type        string            `json:"type"`
	Name        string            `json:"name"`
	DisplayName string            `json:"display_name"`
	ExtraTags   map[string]string `json:"extratags"`
}

func (n nominatimPlace) place() Place {
	lat, _ := strconv.ParseFloat(n.Lat, 64)
	lon, _ := strconv.ParseFloat(n.Lon, 64)
	p := Place{Name: n.Name, Category: strings.TrimSpace(n.Category + " " + n.Type), Address: n.DisplayName, Lat: lat, Lon: lon,
		ID: n.OSMType + "/" + strconv.FormatInt(n.OSMID, 10), URL: osmURL(n.OSMType, n.OSMID)}
	if p.Name == "" {
		p.Name, _, _ = strings.Cut(n.DisplayName, ",")
	}
	if t := n.ExtraTags; t != nil {
		p.Hours, p.Phone, p.Website, p.Cuisine = t["opening_hours"], first(t["phone"], t["contact:phone"]), first(t["website"], t["contact:website"]), t["cuisine"]
	}
	return p
}

func first(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// Search finds places and addresses by name, such as "Coppa, Juneau".
func (c *Client) Search(ctx context.Context, query string, limit int) ([]Place, error) {
	q := url.Values{"q": {query}, "format": {"jsonv2"}, "limit": {strconv.Itoa(limit)}, "extratags": {"1"}}
	body, err := c.get(ctx, c.base(c.Geocoder, DefaultGeocoder)+"/search?"+q.Encode(), nil, query, true)
	if err != nil {
		return nil, err
	}
	var list []nominatimPlace
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("the map service returned something unexpected")
	}
	out := make([]Place, 0, len(list))
	for _, n := range list {
		out = append(out, n.place())
	}
	return out, nil
}

var latLonRe = regexp.MustCompile(`^\s*(-?\d{1,2}(?:\.\d+)?)\s*,\s*(-?\d{1,3}(?:\.\d+)?)\s*$`)

// Locate turns a place name or "lat,lon" into a point.
func (c *Client) Locate(ctx context.Context, where string) (Place, error) {
	where = strings.TrimSpace(where)
	if where == "" {
		return Place{}, fmt.Errorf("a place is required")
	}
	if m := latLonRe.FindStringSubmatch(where); m != nil {
		lat, _ := strconv.ParseFloat(m[1], 64)
		lon, _ := strconv.ParseFloat(m[2], 64)
		if lat >= -90 && lat <= 90 && lon >= -180 && lon <= 180 {
			return Place{Name: where, Lat: lat, Lon: lon}, nil
		}
	}
	list, err := c.Search(ctx, where, 1)
	if err != nil {
		return Place{}, err
	}
	if len(list) == 0 {
		return Place{}, fmt.Errorf("no place called %q was found", where)
	}
	return list[0], nil
}

var osmIDRe = regexp.MustCompile(`^(node|way|relation)/(\d+)$`)

// Details looks up one place by its OpenStreetMap id, such as node/123.
func (c *Client) Details(ctx context.Context, id string) (Place, error) {
	m := osmIDRe.FindStringSubmatch(strings.TrimSpace(id))
	if m == nil {
		return Place{}, fmt.Errorf("id must be an OpenStreetMap object such as node/3430732134, from places.search")
	}
	q := url.Values{"osm_ids": {strings.ToUpper(m[1][:1]) + m[2]}, "format": {"jsonv2"}, "extratags": {"1"}}
	body, err := c.get(ctx, c.base(c.Geocoder, DefaultGeocoder)+"/lookup?"+q.Encode(), nil, id, true)
	if err != nil {
		return Place{}, err
	}
	var list []nominatimPlace
	if err := json.Unmarshal(body, &list); err != nil || len(list) == 0 {
		return Place{}, fmt.Errorf("no place %s was found", id)
	}
	return list[0].place(), nil
}

// Nearby finds places of one kind around a point, nearest first.
func (c *Client) Nearby(ctx context.Context, tag Tag, lat, lon float64, radius, limit int) ([]Place, error) {
	query := fmt.Sprintf(`[out:json][timeout:20];nwr[%q=%q](around:%d,%.6f,%.6f);out center tags %d;`, tag.Key, tag.Value, radius, lat, lon, limit*3)
	body, err := c.get(ctx, c.base(c.Overpass, DefaultOverpass), url.Values{"data": {query}},
		fmt.Sprintf("%s within %d m of %.4f,%.4f", tag.Label, radius, lat, lon), false)
	if err != nil {
		return nil, err
	}
	var res struct {
		Elements []struct {
			Type   string                      `json:"type"`
			ID     int64                       `json:"id"`
			Lat    float64                     `json:"lat"`
			Lon    float64                     `json:"lon"`
			Center *struct{ Lat, Lon float64 } `json:"center"`
			Tags   map[string]string           `json:"tags"`
		} `json:"elements"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("the map service returned something unexpected")
	}
	var out []Place
	for _, e := range res.Elements {
		t := e.Tags
		if t["name"] == "" {
			continue
		}
		plat, plon := e.Lat, e.Lon
		if e.Center != nil {
			plat, plon = e.Center.Lat, e.Center.Lon
		}
		p := Place{Name: t["name"], Category: tag.Label, Lat: plat, Lon: plon, ID: e.Type + "/" + strconv.FormatInt(e.ID, 10), URL: osmURL(e.Type, e.ID),
			Address: address(t), Hours: t["opening_hours"], Phone: first(t["phone"], t["contact:phone"]),
			Website: first(t["website"], t["contact:website"]), Cuisine: t["cuisine"],
			Distance: dist(haversine(lat, lon, plat, plon))}
		out = append(out, p)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Distance.Meters < out[j].Distance.Meters })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// address is a place's street address from its tags.
func address(t map[string]string) string {
	street := strings.TrimSpace(t["addr:housenumber"] + " " + t["addr:street"])
	var parts []string
	for _, p := range []string{street, t["addr:city"], strings.TrimSpace(t["addr:state"] + " " + t["addr:postcode"])} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, ", ")
}

// Modes of travel, and the OSRM profile on routing.openstreetmap.de.
var modes = map[string][2]string{
	"driving": {"routed-car", "driving"},
	"walking": {"routed-foot", "foot"},
	"cycling": {"routed-bike", "bike"},
}

// Step is one instruction of a route.
type Step struct {
	Instruction string `json:"instruction"`
	Distance    *Dist  `json:"distance"`
}

// Route is a way from one place to another.
type Route struct {
	Distance *Dist   `json:"distance"`
	Minutes  float64 `json:"minutes"`
	Steps    []Step  `json:"steps,omitempty"`
}

// RouteBetween asks the router for a route; steps adds the instructions.
func (c *Client) RouteBetween(ctx context.Context, from, to Place, mode string, steps bool) (Route, error) {
	m, ok := modes[mode]
	if !ok {
		return Route{}, fmt.Errorf("mode must be driving, walking, or cycling")
	}
	router := c.base(c.Router, DefaultRouter)
	path := fmt.Sprintf("/route/v1/%s/%.6f,%.6f;%.6f,%.6f?overview=false&steps=%t", m[1], from.Lon, from.Lat, to.Lon, to.Lat, steps)
	if router == DefaultRouter {
		path = "/" + m[0] + path
	}
	body, err := c.get(ctx, router+path, nil, fmt.Sprintf("%s from %s to %s", mode, from.Name, to.Name), false)
	if err != nil {
		return Route{}, err
	}
	var res struct {
		Code   string `json:"code"`
		Routes []struct {
			Distance float64 `json:"distance"`
			Duration float64 `json:"duration"`
			Legs     []struct {
				Steps []osrmStep `json:"steps"`
			} `json:"legs"`
		} `json:"routes"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return Route{}, fmt.Errorf("the map service returned something unexpected")
	}
	if res.Code != "Ok" || len(res.Routes) == 0 {
		return Route{}, fmt.Errorf("no %s route was found between those places", mode)
	}
	r := res.Routes[0]
	out := Route{Distance: dist(r.Distance), Minutes: math.Round(r.Duration/6) / 10}
	for _, leg := range r.Legs {
		for _, s := range leg.Steps {
			out.Steps = append(out.Steps, Step{Instruction: s.instruction(), Distance: dist(s.Distance)})
		}
	}
	return out, nil
}

type osrmStep struct {
	Name     string  `json:"name"`
	Distance float64 `json:"distance"`
	Maneuver struct {
		Type         string `json:"type"`
		Modifier     string `json:"modifier"`
		BearingAfter int    `json:"bearing_after"`
		Exit         int    `json:"exit"`
	} `json:"maneuver"`
}

// instruction words an OSRM maneuver, such as "Turn left onto Glacier
// Avenue".
func (s osrmStep) instruction() string {
	m := s.Maneuver
	onto := ""
	if s.Name != "" {
		onto = " onto " + s.Name
	}
	switch m.Type {
	case "depart":
		dirs := []string{"north", "northeast", "east", "southeast", "south", "southwest", "west", "northwest"}
		head := "Head " + dirs[((m.BearingAfter+22)%360)/45]
		if s.Name != "" {
			head += " on " + s.Name
		}
		return head
	case "arrive":
		return "Arrive at the destination"
	case "roundabout", "rotary":
		if m.Exit > 0 {
			return fmt.Sprintf("At the roundabout, take exit %d%s", m.Exit, onto)
		}
		return "Go through the roundabout" + onto
	case "merge":
		return "Merge" + onto
	case "on ramp":
		return "Take the ramp" + onto
	case "off ramp":
		return "Take the exit" + onto
	case "fork":
		return "Keep " + m.Modifier + " at the fork" + onto
	}
	switch m.Modifier {
	case "straight":
		return "Continue straight" + onto
	case "uturn":
		return "Make a U-turn" + onto
	case "":
		return "Continue" + onto
	}
	if side, ok := strings.CutPrefix(m.Modifier, "slight "); ok {
		return "Bear " + side + onto
	}
	return "Turn " + m.Modifier + onto
}
